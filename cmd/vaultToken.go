package cmd

import (
	"fmt"
	"time"

	sdkconfig "github.com/bizshuk/gosdk/config"
	"github.com/bizshuk/gosdk/config/vault"
	"github.com/spf13/cobra"
)

// DEFAULT_TOKEN_TTL is one working day: long enough that a session is not
// interrupted, short enough that a leaked token is not a standing key.
const DEFAULT_TOKEN_TTL = 8 * time.Hour

// VaultTokenCmd issues a time-limited credential for the vault.
var VaultTokenCmd = &cobra.Command{
	Use:   "token [vault-file]",
	Short: "Issue a time-limited token that unlocks the vault without the password",
	Long: `Issue a time-limited token that unlocks the vault without the password.

The token is printed to stdout and the expiry to stderr, so it can be captured
directly:

    export ` + sdkconfig.VAULT_TOKEN_ENV + `=$(app vault token --ttl 8h)

Issuing one requires the master password. After that, every vault command and
config.Default() accept the token until it expires.

How it works: the token carries the vault's data key, wrapped with a key
derived from this machine's device key and the expiry timestamp. The timestamp
is authenticated, so extending it by editing the token makes it undecryptable.

Treat a token as a password with a deadline. Anyone holding both the token and
this machine's device key can decrypt the vault, expiry included — the check is
performed by this program, not by the cryptography. Run "vault revoke" to
invalidate every token at once.`,
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         RunVaultToken,
}

var vaultTokenTTL time.Duration

func init() {
	VaultTokenCmd.Flags().DurationVar(&vaultTokenTTL, "ttl", DEFAULT_TOKEN_TTL,
		"how long the token stays valid (e.g. 30m, 8h)")
}

// RunVaultToken executes VaultTokenCmd.
func RunVaultToken(c *cobra.Command, args []string) error {
	path := vaultArg(args, 0, VAULT_FILE)

	// Deliberately not openVault: a token must never be able to mint another
	// token, or the deadline could be extended indefinitely without the
	// password ever being entered again.
	pw, err := vaultPassword(false)
	if err != nil {
		return err
	}
	defer vault.Wipe(pw)

	v, err := vault.OpenFile(path, pw)
	if err != nil {
		return err
	}
	defer v.Close()

	deviceKey, err := vault.EnsureDeviceKey(vault.DefaultDeviceKeyPath())
	if err != nil {
		return err
	}
	defer vault.Wipe(deviceKey)

	token, err := v.IssueToken(deviceKey, vaultTokenTTL)
	if err != nil {
		return err
	}
	exp, err := vault.TokenExpiry(token)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.ErrOrStderr(), "valid until %s (%s)\n",
		exp.Format(time.RFC3339), vaultTokenTTL)
	_, err = fmt.Fprintln(c.OutOrStdout(), token)
	return err
}
