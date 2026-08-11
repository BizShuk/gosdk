package cmd

import (
	"fmt"
	"os"

	"github.com/bizshuk/gosdk/config/vault"
	"github.com/spf13/cobra"
)

// VaultDecryptCmd restores a vault back to a plaintext .env file.
var VaultDecryptCmd = &cobra.Command{
	Use:   "decrypt [vault-file]",
	Short: "Decrypt a vault back into a .env file (mode 0600)",
	Long: `Decrypt a vault back into a .env file (mode 0600).

The output path is the vault path with ".vault" removed. A vault not named
that way is written to <vault-file>.decrypted rather than over itself.

Use "show" to read the contents without putting plaintext on disk.`,
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         RunVaultDecrypt,
}

// RunVaultDecrypt executes VaultDecryptCmd.
func RunVaultDecrypt(c *cobra.Command, args []string) error {
	vaultPath := vaultArg(args, 0, VAULT_FILE)

	entries, err := decryptAll(vaultPath)
	if err != nil {
		return err
	}
	outPath := plaintextPathFor(vaultPath)
	if err := os.WriteFile(outPath, vault.MarshalEnv(entries), VAULT_FILE_PERM); err != nil {
		return err
	}
	fmt.Fprintf(c.OutOrStdout(), "decrypted %d variables -> %s (mode 0600)\n", len(entries), outPath)
	return nil
}

// decryptAll unlocks the vault at path and returns every variable in plaintext.
// Shared with VaultShowCmd, which differs only in where the result goes.
func decryptAll(vaultPath string) (map[string]string, error) {
	v, err := openVault(vaultPath)
	if err != nil {
		return nil, err
	}
	defer v.Close()
	return v.DecryptAll()
}
