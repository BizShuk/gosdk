package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var patchCmd = &cobra.Command{
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

func init() {
	rootCmd.AddCommand(patchCmd)
}