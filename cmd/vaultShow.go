package cmd

import (
	"github.com/bizshuk/gosdk/config/vault"
	"github.com/spf13/cobra"
)

// VaultShowCmd prints a decrypted vault without writing plaintext to disk.
var VaultShowCmd = &cobra.Command{
	Use:          "show [vault-file]",
	Short:        "Print the decrypted vault in .env format, without writing a file",
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         RunVaultShow,
}

// RunVaultShow executes VaultShowCmd.
func RunVaultShow(c *cobra.Command, args []string) error {
	entries, err := decryptAll(vaultArg(args, 0, VAULT_FILE))
	if err != nil {
		return err
	}
	_, err = c.OutOrStdout().Write(vault.MarshalEnv(entries))
	return err
}
