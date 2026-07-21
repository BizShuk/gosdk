package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// MinorCmd increments the minor version in VERSION_FILE and resets patch to 0.
var MinorCmd = &cobra.Command{
	Use:   "minor",
	Short: "Increment minor version",
	Long:  `Increments the minor version and resets patch to 0.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		v, err := ReadVersion()
		if err != nil {
			return err
		}
		v.Minor++
		v.Patch = 0
		if err := WriteVersion(v); err != nil {
			return err
		}
		fmt.Println(v)
		return nil
	},
}
