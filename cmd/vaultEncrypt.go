package cmd

import (
	"fmt"
	"os"

	"github.com/bizshuk/gosdk/config/vault"
	"github.com/spf13/cobra"
)

// VaultEncryptCmd turns a plaintext .env into an encrypted .env.vault.
var VaultEncryptCmd = &cobra.Command{
	Use:   "encrypt [env-file]",
	Short: "Encrypt a .env file into <env-file>.vault",
	Long: `Encrypt a .env file into <env-file>.vault.

The plaintext file is left untouched — deleting it (and adding it to
.gitignore) is a decision for the caller, not a side effect of encrypting.`,
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         RunVaultEncrypt,
}

// RunVaultEncrypt executes VaultEncryptCmd.
func RunVaultEncrypt(c *cobra.Command, args []string) error {
	envPath := vaultArg(args, 0, VAULT_ENV_FILE)

	data, err := os.ReadFile(envPath)
	if err != nil {
		return err
	}
	entries := vault.ParseEnv(data)
	if len(entries) == 0 {
		return fmt.Errorf("%s holds no variables", envPath)
	}

	// Confirmed twice: a typo here is unrecoverable, because the password is
	// never stored anywhere to check it against later.
	pw, err := vaultPassword(true)
	if err != nil {
		return err
	}
	defer vault.Wipe(pw)

	v, err := vault.New(pw)
	if err != nil {
		return err
	}
	defer v.Close()

	if err := v.SetAll(entries); err != nil {
		return err
	}

	outPath := envPath + VAULT_SUFFIX
	if err := v.SaveFile(outPath); err != nil {
		return err
	}
	fmt.Fprintf(c.OutOrStdout(), "encrypted %d variables -> %s\n", len(entries), outPath)
	fmt.Fprintf(c.OutOrStdout(), "add %s to .gitignore; %s is the committable file\n", envPath, outPath)
	return nil
}
