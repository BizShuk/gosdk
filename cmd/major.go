package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// MajorCmd increments the major version in VERSION_FILE and resets minor and
// patch to 0.
var MajorCmd = &cobra.Command{
	Use:   "major",
	Short: "Increment major version",
	Long:  `Increments the major version and resets minor and patch to 0.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		v, err := ReadVersion()
		if err != nil {
			return err
		}
		v.Major++
		v.Minor = 0
		v.Patch = 0
		if err := WriteVersion(v); err != nil {
			return err
		}
		fmt.Println(v)
		return nil
	},
}
