package cmd

import (
	"fmt"

	"github.com/bizshuk/gosdk/config/vault"
	"github.com/spf13/cobra"
)

// VaultRevokeCmd invalidates every token issued on this machine.
var VaultRevokeCmd = &cobra.Command{
	Use:   "revoke",
	Short: "Invalidate every token issued on this machine",
	Long: `Invalidate every token issued on this machine.

Tokens are not registered anywhere, so they cannot be revoked one by one. What
can be replaced is the device key every token was wrapped with — rotating it
makes all of them undecryptable at once.

The vault itself is untouched: the master password still opens it, and no
secret needs re-encrypting.`,
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE:         RunVaultRevoke,
}

// RunVaultRevoke executes VaultRevokeCmd.
func RunVaultRevoke(c *cobra.Command, args []string) error {
	path := vault.DefaultDeviceKeyPath()
	key, err := vault.RotateDeviceKey(path)
	if err != nil {
		return err
	}
	vault.Wipe(key)

	_, err = fmt.Fprintf(c.OutOrStdout(),
		"rotated the device key at %s; every previously issued token is now invalid\n", path)
	return err
}
