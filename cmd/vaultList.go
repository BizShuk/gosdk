package cmd

import (
	"fmt"

	"github.com/bizshuk/gosdk/config/vault"
	"github.com/spf13/cobra"
)

// VaultListCmd lists variable names without decrypting anything.
var VaultListCmd = &cobra.Command{
	Use:   "list [vault-file]",
	Short: "List variable names in the vault (no password required)",
	Long: `List variable names in the vault (no password required).

Only values are encrypted. Names are stored in the clear, which is what makes
"which secrets does this application expect" answerable on a machine that
cannot decrypt them.`,
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         RunVaultList,
}

// RunVaultList executes VaultListCmd.
func RunVaultList(c *cobra.Command, args []string) error {
	keys, err := vault.KeysOfFile(vaultArg(args, 0, VAULT_FILE))
	if err != nil {
		return err
	}
	for _, k := range keys {
		if _, err := fmt.Fprintln(c.OutOrStdout(), k); err != nil {
			return err
		}
	}
	return nil
}
