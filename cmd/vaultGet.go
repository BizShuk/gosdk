package cmd

import (
	"fmt"

	"github.com/bizshuk/gosdk/config/vault"
	"github.com/spf13/cobra"
)

// VaultGetCmd decrypts a single variable.
var VaultGetCmd = &cobra.Command{
	Use:   "get KEY [vault-file]",
	Short: "Decrypt and print a single variable",
	Long: `Decrypt and print a single variable.

Only the value is written to stdout — the prompt goes to stderr — so the
result can be captured directly:

    export API_KEY=$(app vault get API_KEY)`,
	Args:         cobra.RangeArgs(1, 2),
	SilenceUsage: true,
	RunE:         RunVaultGet,
}

// RunVaultGet executes VaultGetCmd.
func RunVaultGet(c *cobra.Command, args []string) error {
	v, err := openVault(vaultArg(args, 1, VAULT_FILE))
	if err != nil {
		return err
	}
	defer v.Close()

	// GetBytes rather than Get: a single value is exactly the case where the
	// plaintext can be wiped once it has been written out.
	value, err := v.GetBytes(args[0])
	if err != nil {
		return err
	}
	defer vault.Wipe(value)

	_, err = fmt.Fprintf(c.OutOrStdout(), "%s\n", value)
	return err
}
