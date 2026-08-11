// Package cmd provides ready-made cobra commands for applications built on the
// SDK. Nothing here is executable on its own; the hosting application registers
// what it needs on its own root command.
package cmd

import (
	"fmt"
	"strings"

	"github.com/bizshuk/gosdk/cmd/config"
	sdkconfig "github.com/bizshuk/gosdk/config"
	"github.com/spf13/cobra"
)

// ConfigCmd is the "config" command for the hosting application to register:
//
//	root.AddCommand(cmd.ConfigCmd)
//
// This file is the CLI shell and nothing more: it owns the flags, the help
// text and the dispatch, then hands off to cmd/config for the work and back to
// cmd/config for the rendering. Everything a second front end would want — the
// merged view, the mutations, the reports — is exported from there.
var ConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Show and modify application configuration",
	Long: `Show and modify application configuration.

With no flags, prints every configuration key merged across the files the
config package loads, with OS environment variables applied on top. Every row
carries the path of the file the winning value was actually read from; --source
adds the values it overrode. Use --files to list the search itself.

Each file is looked for in the working directory, then ./conf, then the app
config directory (~/.config/<appName>/, when the host application registered a
name via config.WithAppName or forced a directory with config.WithConfigDir).
The first directory holding a given file wins outright — directories are a
fallback chain, not a merge, so a config.yaml in the working directory hides
the one in ~/.config/<appName>/ entirely.

Keys use viper's dotted path syntax; "." is the nesting delimiter:

    a.b.c   ->  {"a": {"b": {"c": "xyz"}}}

"_" is an ordinary character, so a_b_c is a separate flat key from a.b.c. Both
flat and nested keys can be overridden from the environment: the config package
binds every key to UPPER(key) with "." replaced by "_", unprefixed, so a.b.c
reads A_B_C and log_level reads LOG_LEVEL.

--update, --add and --delete write to one of the six writable config files.
By default the target is ` + config.LOCAL_SETTINGS_FILE + ` — the last file the
JSON loader merges, so a value written there overrides every JSON and YAML file
but neither .env nor an environment variable. Pass --file to choose a different
target from the list the config package actually loads.

Precedence, lowest to highest:

    config.yaml < config.local.yaml < settings.json < settings.local.json
              < .env.vault < .env.local.vault < .env < .env.local

A write to a lower-precedence file is shadowed by every file above it, and the
report will warn when that happens at runtime.

The vault files are shown and shadow other layers, but cannot be written here:
changing an encrypted value needs the master password. Use the vault command
(cmd.VaultCmd) for that.

The target file lives in the app config directory (~/.config/<appName>/) when
the host application set an app name via config.WithAppName. Pass --local to
opt out and write to the working-directory location instead.

For .env targets nested keys are flattened: a.b.c becomes A_B_C. .env has no
nesting, so the only way to express a multi-segment key is the upper-snake
form. A per-key note is added to the report whenever a flattening happens.

--append and --remove-from work at the element level: --append a.b.c=value
adds value (as a string) to the array at a.b.c, creating []any when the path
is missing; --remove-from a.b.c=value removes the first matching element.
Both target a single element and are rejected for .env files because env
files are flat. For typed-array writes (numbers, booleans, nested objects)
use --update with a JSON literal instead.`,
	Example: `  # show the merged configuration, each value with the file it came from
  app config

  # add the values each key overrode
  app config --source

  # list the config files that were searched and where each one resolved to
  app config --files

  # set a nested field (creates intermediate levels)
  app config --update server.host=0.0.0.0

  # values that parse as JSON keep their type; quote to force a string
  app config --update server.port=8080
  app config --update build.number='"1234"'

  # JSON literals preserve their type: arrays become []any in the file
  app config --update 'tags=["a","b","c"]'
  app config --update 'ports=[80,443]'

  # append or remove a single string element from an array field
  app config --append tags=d
  app config --remove-from tags=a

  # remove a field
  app config --delete server.host

  # write into a different config file
  app config --file config.yaml --update server.port=8080
  app config --file .env --update log_level=info

  # write to the working directory instead of ~/.config/<appName>/
  app config --local --update a.b=xyz`,
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE:         RunConfig,
}

// Flag values bound in init(). They are package-level because ConfigCmd is a
// single shared command rather than a per-call instance; a process that runs
// ConfigCmd more than once must clear them in between.
var (
	configSource  bool
	configFiles   bool
	configUpdates []string
	configAdds    []string
	configDeletes []string
	configAppends []string
	configRemoves []string
	configFile    string
	configLocal   bool
)

func init() {
	f := ConfigCmd.Flags()
	f.BoolVar(&configSource, "source", false,
		"list the values each key overrode (the winning source is always shown)")
	f.BoolVar(&configFiles, "files", false,
		"list the config files searched and the path each one resolved to")
	f.StringArrayVar(&configUpdates, "update", nil, "set a field, as a.b.c=value (repeatable)")
	f.StringArrayVar(&configAdds, "add", nil, "alias for --update (repeatable)")
	f.StringArrayVar(&configDeletes, "delete", nil, "remove a field, as a.b.c (repeatable)")
	f.StringArrayVar(&configAppends, "append", nil,
		"append a string element to an array field, as a.b.c=value (repeatable; not supported for .env files)")
	f.StringArrayVar(&configRemoves, "remove-from", nil,
		"remove the first matching element from an array field, as a.b.c=value (repeatable; not supported for .env files)")
	f.StringVar(&configFile, "file", config.LOCAL_SETTINGS_FILE,
		"config file to write: "+strings.Join(config.SupportedFiles(), ", "))
	f.BoolVar(&configLocal, "local", false,
		"write to the working directory instead of the app config dir (GetAppConfigDir)")
}

// RunConfig executes ConfigCmd using the values bound to its flags.
func RunConfig(c *cobra.Command, args []string) error {
	// --add is an alias for --update; concatenating keeps the two flags
	// independent instead of aliasing one slice variable.
	updates := append(append([]string{}, configUpdates...), configAdds...)

	// No mutation flag at all means "show". Anything else is a write, and
	// every combination goes through config.Apply so the target file is
	// rewritten exactly once — that single-write property is what makes
	// --append immediately followed by --update on the same key predictable.
	if len(updates)+len(configDeletes)+len(configAppends)+len(configRemoves) == 0 {
		if configFiles {
			_, err := fmt.Fprint(c.OutOrStdout(), config.RenderSourceTable(sdkconfig.Sources()))
			return err
		}
		_, err := fmt.Fprint(c.OutOrStdout(), config.RenderShowTable(config.Show(), configSource))
		return err
	}

	report, err := config.Apply(updates, configDeletes, configAppends, configRemoves,
		config.WriteOptions{File: configFile, Local: configLocal})
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(c.OutOrStdout(), config.RenderChangeReport(report))
	return err
}
