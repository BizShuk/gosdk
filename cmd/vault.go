package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	sdkconfig "github.com/bizshuk/gosdk/config"
	"github.com/bizshuk/gosdk/config/vault"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// Default file names the vault subcommands operate on when no path is given.
const (
	VAULT_ENV_FILE   = ".env"
	VAULT_FILE       = ".env.vault"
	VAULT_FILE_PERM  = 0o600
	VAULT_SUFFIX     = ".vault"
	VAULT_DECRYPT_TO = ".decrypted"
)

// VaultCmd is the "vault" command for the hosting application to register:
//
//	root.AddCommand(cmd.VaultCmd)
//
// This file is the CLI shell for the whole vault family: it owns the command
// tree and the two things every subcommand needs — the master password and the
// unlocked vault — and hands the actual crypto to config/vault. Each leaf
// command lives in its own vault*.go file.
var VaultCmd = &cobra.Command{
	Use:   "vault",
	Short: "Encrypt and read a .env secret vault",
	Long: `Encrypt and read a .env secret vault.

A vault is a .env file encrypted with a master password (scrypt + AES-256-GCM).
Unlike .env, ` + VAULT_FILE + ` is safe to commit: the values are ciphertext.
Variable names are not encrypted, so "vault list" needs no password.

Credentials are resolved in one order everywhere — ` + sdkconfig.VAULT_TOKEN_ENV + `,
then ` + sdkconfig.VAULT_PASSWORD_ENV + `, then a terminal prompt. Those are the
same variables config.Default() uses to decrypt the vault at startup, so a shell
that can run the application can run these commands without retyping anything.

"vault token" issues a credential that stops working after a deadline, so a
build agent or a long-lived shell never has to hold the master password.
"vault revoke" invalidates every token ever issued on this machine.

Precedence when the application loads its configuration, lowest to highest:

    config.yaml < config.local.yaml < settings.json < settings.local.json
              < ` + VAULT_FILE + ` < .env.local.vault < .env < .env.local

The vault sits below .env on purpose: it holds the values the whole team
shares, and a committed file must not override a developer's own machine.`,
	Example: `  # encrypt .env into .env.vault, then delete the plaintext
  app vault encrypt

  # print the decrypted contents without writing a file
  app vault show

  # take a single secret into the environment
  export API_KEY=$(app vault get API_KEY)

  # list variable names (no password needed)
  app vault list

  # add or replace one variable
  app vault set API_KEY sk-123

  # unlock once, then work for 8 hours without the master password
  export VAULT_TOKEN=$(app vault token --ttl 8h)

  # invalidate every token issued on this machine
  app vault revoke`,
	SilenceUsage: true,
}

func init() {
	VaultCmd.AddCommand(
		VaultEncryptCmd,
		VaultDecryptCmd,
		VaultShowCmd,
		VaultGetCmd,
		VaultListCmd,
		VaultSetCmd,
		VaultTokenCmd,
		VaultRevokeCmd,
	)
}

// vaultArg returns positional argument i, or def when it was not supplied.
func vaultArg(args []string, i int, def string) string {
	if len(args) > i {
		return args[i]
	}
	return def
}

// openVault unlocks the vault at path with whatever credential is available:
// VAULT_TOKEN first, then VAULT_PASSWORD, then a prompt.
//
// The caller owns the returned vault and must Close it — that is what wipes
// the data key out of memory.
func openVault(path string) (*vault.Vault, error) {
	if token := os.Getenv(sdkconfig.VAULT_TOKEN_ENV); token != "" {
		key, err := vault.LoadDeviceKey(vault.DefaultDeviceKeyPath())
		if err != nil {
			return nil, err
		}
		defer vault.Wipe(key)
		return vault.OpenFileWithToken(path, key, token)
	}

	pw, err := vaultPassword(false)
	if err != nil {
		return nil, err
	}
	defer vault.Wipe(pw)
	return vault.OpenFile(path, pw)
}

// vaultPassword returns the master password from VAULT_PASSWORD, or prompts
// for it when the variable is unset. The caller should wipe the result.
//
// The environment always wins over the prompt: a script that exported the
// password to run the application should not be interrupted by a terminal
// read it cannot answer. confirm asks twice, and only ever at the prompt —
// a value already in the environment has nothing to be confirmed against.
//
// A password taken from the environment cannot really be wiped: it is a Go
// string owned by the process environment. Wiping the copy is still worth
// doing — the prompt path, which is the one an operator uses interactively,
// never turns the password into a string at all.
func vaultPassword(confirm bool) ([]byte, error) {
	if pw := os.Getenv(sdkconfig.VAULT_PASSWORD_ENV); pw != "" {
		return []byte(pw), nil
	}
	return promptPassword(confirm)
}

// promptPassword reads the password from the terminal without echoing it,
// falling back to stdin when there is no terminal so the commands stay usable
// from a pipeline.
//
// The password stays []byte the whole way through — term.ReadPassword already
// returns bytes, and never converting to string is what makes it wipeable.
func promptPassword(confirm bool) ([]byte, error) {
	read := func(prompt string) ([]byte, error) {
		fmt.Fprint(os.Stderr, prompt)
		if term.IsTerminal(int(syscall.Stdin)) {
			b, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Fprintln(os.Stderr)
			return b, err
		}
		var s string
		_, err := fmt.Fscanln(os.Stdin, &s)
		return []byte(s), err
	}

	pw, err := read("Master password: ")
	if err != nil {
		return nil, err
	}
	if len(pw) == 0 {
		return nil, errors.New("password must not be empty")
	}
	if confirm {
		again, err := read("Confirm password: ")
		if err != nil {
			return nil, err
		}
		defer vault.Wipe(again)
		if !bytes.Equal(pw, again) {
			vault.Wipe(pw)
			return nil, errors.New("the two passwords do not match")
		}
	}
	return pw, nil
}

// plaintextPathFor maps a vault path back to the .env path decrypt writes to,
// never returning the input itself: overwriting the vault with its own
// plaintext would destroy the only encrypted copy.
func plaintextPathFor(vaultPath string) string {
	out := strings.TrimSuffix(vaultPath, VAULT_SUFFIX)
	if out == vaultPath {
		return vaultPath + VAULT_DECRYPT_TO
	}
	return out
}
