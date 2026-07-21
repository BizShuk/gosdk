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

The content is supplied by the hosting application, normally from a
settings.example.json or config.example.yaml embedded with go:embed:

    //go:embed settings.example.json
    var defaultSettings []byte

    func init() { config.MustRegisterDefault("settings.json", defaultSettings) }

The file is written into the application config directory reported by
config.GetAppConfigDir() — ~/.config/<appName> — so the application must have
been configured with config.Default(config.WithAppName("myapp")) first. Without
an app name the command refuses rather than dropping a config file into the
working directory by accident; pass --local when that is what you want.

An existing file is left alone. Two flags change that:

    --merge   add the fields the default has and the file lacks, keeping
              every value already in the file. Use this after an upgrade to
              pick up settings a new release introduced.
    --force   replace the file with the default, discarding local edits.

--merge rewrites the file through the config parser, so comments and key
order in a .env or config.yaml are not preserved. Run --merge --dry-run
first to see exactly which keys would be added.

An application that ships no default for the requested file is not an error:
the command says so and exits successfully, so an installer can call it
without knowing whether this build carries a default.`,
	Example: `  # write ~/.config/myapp/settings.json
  app config default

  # after upgrading: add newly introduced settings, keep local edits
  app config default --merge

  # see which keys an upgrade would add, touching nothing
  app config default --merge --dry-run

  # seed a YAML config instead
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
	configDefaultMerge  bool
	configDefaultDryRun bool
	configDefaultLocal  bool
)

func init() {
	f := ConfigDefaultCmd.Flags()
	f.StringVar(&configDefaultFile, "file", config.DEFAULT_CONFIG_FILE,
		"config file to write: "+strings.Join(config.SupportedFiles(), ", "))
	f.BoolVar(&configDefaultMerge, "merge", false,
		"add fields the default has and the existing file lacks, keeping existing values")
	f.BoolVar(&configDefaultForce, "force", false, "overwrite the file if it already exists")
	f.BoolVar(&configDefaultDryRun, "dry-run", false, "report what would happen without writing anything")
	f.BoolVar(&configDefaultLocal, "local", false,
		"write to the working directory instead of the app config dir (GetAppConfigDir)")

	// Merging and replacing are opposite answers to the same question, so
	// let cobra reject the combination instead of silently picking one.
	ConfigDefaultCmd.MarkFlagsMutuallyExclusive("merge", "force")

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

	mode := config.DefaultModeSkip
	switch {
	case configDefaultMerge:
		mode = config.DefaultModeMerge
	case configDefaultForce:
		mode = config.DefaultModeForce
	}

	report, err := config.Default(configDefaultFile, mode, configDefaultDryRun, configDefaultLocal)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(c.OutOrStdout(), config.RenderDefaultReport(report))
	return err
}
