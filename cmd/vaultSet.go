package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// VaultSetCmd adds or replaces one variable in an existing vault.
var VaultSetCmd = &cobra.Command{
	Use:          "set KEY VALUE [vault-file]",
	Short:        "Add or replace a single variable in the vault",
	Args:         cobra.RangeArgs(2, 3),
	SilenceUsage: true,
	RunE:         RunVaultSet,
}

// RunVaultSet executes VaultSetCmd.
func RunVaultSet(c *cobra.Command, args []string) error {
	path := vaultArg(args, 2, VAULT_FILE)

	v, err := openVault(path)
	if err != nil {
		return err
	}
	defer v.Close()

	if err := v.Set(args[0], args[1]); err != nil {
		return err
	}
	if err := v.SaveFile(path); err != nil {
		return err
	}
	_, err = fmt.Fprintf(c.OutOrStdout(), "updated %s in %s\n", args[0], path)
	return err
}
