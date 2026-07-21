package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// PatchCmd increments the patch version in VERSION_FILE.
var PatchCmd = &cobra.Command{
	Use:   "patch",
	Short: "Increment patch version",
	Long:  `Increments the patch version.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		v, err := ReadVersion()
		if err != nil {
			return err
		}
		v.Patch++
		if err := WriteVersion(v); err != nil {
			return err
		}
		fmt.Println(v)
		return nil
	},
}
