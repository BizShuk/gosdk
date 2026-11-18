package cmd

import (
	"fmt"
	"strings"

	"github.com/bizshuk/gosdk/cmd/config"
	sdkconfig "github.com/bizshuk/gosdk/config"
	"github.com/spf13/cobra"
)

// ConfigDefaultCmd is the "default" subcommand, attached to ConfigCmd in
// init(). Registering ConfigCmd on a root command brings it along:
//
//	root.AddCommand(cmd.ConfigCmd)   // enables "app config default"
//
// The content it writes comes from the hosting application, handed in through
// config.RegisterDefault; this file only turns flags into a call and a report
// into text.
var ConfigDefaultCmd = &cobra.Command{
	Use:   "default",
	Short: "Write the application's default configuration file",
	Long: `Write the application's default configuration file.

The content is supplied by the hosting application, normally the files in its
config/example/ directory embedded with go:embed:

    //go:embed config/example/settings.example.json
    var defaultSettings []byte

    //go:embed config/example/.env.example
    var defaultEnv []byte

    func init() {
        config.MustRegisterDefault("settings.json", defaultSettings)
        config.MustRegisterDefault(".env", defaultEnv)
    }

Without --file every registered file is written; --file picks one. .env files
are created with mode 0600 because they hold credentials.

The file is written into the application config directory reported by
config.GetAppConfigDir() — ~/.config/<appName> — so the application must have
been configured with config.Default(config.WithAppName("myapp")) first. Without
an app name the command refuses rather than dropping a config file into the
working directory by accident; pass --local when that is what you want.

A missing file is copied from the default byte-for-byte. An existing file is
merged: the fields the default has and the file lacks are added, every value
already in the file is kept, so running this after every upgrade picks up new
settings safely. --force replaces the file with the default instead,
discarding local edits.

Before a merge or --force rewrites a file, the old one is copied to
<file>.bak (older backups become <file>.bak.<date>, newest 5 kept). A merge
goes through the config parser, so comments and key order in a .env or
config.yaml are not preserved; --dry-run shows which keys would be added.

An application that ships no default for the requested file is not an error:
the command says so and exits successfully, so an installer can call it
without knowing whether this build carries a default.`,
	Example: `  # create missing files, add newly introduced settings to existing ones
  app config default

  # see which files would be created and which keys added, touching nothing
  app config default --dry-run

  # seed only one file
  app config default --file config.yaml

  # replace an existing file
  app config default --force

  # write to the working directory instead of ~/.config/<appName>/
  app config default --local`,
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE:         RunConfigDefault,
}

// Flag values bound in init(); package-level for the same reason ConfigCmd's
// are — a process that runs this command twice must clear them in between.
var (
	configDefaultFile   string
	configDefaultForce  bool
	configDefaultDryRun bool
	configDefaultLocal  bool
)

func init() {
	f := ConfigDefaultCmd.Flags()
	f.StringVar(&configDefaultFile, "file", "",
		"write only this file (default: every registered one): "+strings.Join(config.SupportedFiles(), ", "))
	f.BoolVar(&configDefaultForce, "force", false,
		"replace an existing file instead of merging the missing fields into it")
	f.BoolVar(&configDefaultDryRun, "dry-run", false, "report what would happen without writing anything")
	f.BoolVar(&configDefaultLocal, "local", false,
		"write to the working directory instead of the app config dir (GetAppConfigDir)")

	ConfigCmd.AddCommand(ConfigDefaultCmd)
}

// RunConfigDefault executes ConfigDefaultCmd using its bound flags.
func RunConfigDefault(c *cobra.Command, args []string) error {
	// Seeding a default is a first-run action, and the file it produces is
	// meant to live with the application rather than wherever the operator
	// happened to be standing. Refuse here rather than letting FilePath fall
	// back to the working directory: a settings.json quietly dropped into a
	// shell's cwd looks like success and is never read again.
	//
	// The CLI is the right place for the refusal — it is a precondition on
	// this command, not on config.Default, which a library caller may
	// legitimately point anywhere.
	if !configDefaultLocal && sdkconfig.GetAppConfigDir() == "" {
		return fmt.Errorf("no application config directory: call " +
			`config.Default(config.WithAppName("myapp")) at startup, ` +
			"or pass --local to write to the working directory")
	}

	mode := config.DefaultModeMerge
	if configDefaultForce {
		mode = config.DefaultModeForce
	}

	if configDefaultFile != "" {
		report, err := config.Default(configDefaultFile, mode, configDefaultDryRun, configDefaultLocal)
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(c.OutOrStdout(), config.RenderDefaultReport(report))
		return err
	}

	reports, err := config.DefaultAll(mode, configDefaultDryRun, configDefaultLocal)
	for _, report := range reports {
		if _, werr := fmt.Fprint(c.OutOrStdout(), config.RenderDefaultReport(report)); werr != nil {
			return werr
		}
	}
	if err != nil {
		return err
	}
	if len(reports) == 0 {
		_, err = fmt.Fprintln(c.OutOrStdout(), "no default configuration registered; nothing to do")
	}
	return err
}
