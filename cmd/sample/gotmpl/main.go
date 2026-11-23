/*
Copyright © 2023 Shuk
*/
package gotmpl

import "github.com/bizshuk/gosdk/v2/cmd/sample/gotmpl/cmd"

func run(args []string) error {
	cmd.RootCmd.SetArgs(args)
	return cmd.RootCmd.Execute()
}
