// Package configdefault shows how a hosting application seeds its own default
// configuration file through the SDK's "config default" subcommand.
//
// The application owns the content — a settings.example.json sitting in its
// repository root, compiled in with go:embed — and the SDK owns the plumbing:
// where the file goes, whether it may overwrite an existing one, and how the
// result is reported.
//
//	seedapp config default              # write ~/.config/seedapp/settings.json
//	seedapp config default --dry-run    # show the target path, touch nothing
//	seedapp config default --merge      # after an upgrade: add newly shipped fields
//	seedapp config default --force      # replace an existing file
//	seedapp config default --file config.yaml
//
// --merge is the upgrade path. A release that adds a field to
// settings.example.json reaches existing installs through it: the new keys are
// grafted into the operator's file and every value already there is left
// alone. Pair it with --dry-run to see the additions before committing.
//
// Two SDK mechanisms seed a config file; they are not interchangeable:
//
//   - config.WithDefaultValue writes settings.json during config.Default(),
//     implicitly, before the user has run anything. Good for "the app must not
//     start without a config".
//   - "config default" writes only when the user asks, reports what it did,
//     and refuses to clobber an edited file without --force. Good for
//     "give me a config I can edit".
//
// Run it via the test in this directory: go test ./cmd/sample/configdefault
package configdefault

import (
	_ "embed"
	"io"

	"github.com/bizshuk/gosdk/cmd"
	cfgcmd "github.com/bizshuk/gosdk/cmd/config"
	"github.com/bizshuk/gosdk/config"
	"github.com/spf13/cobra"
)

// APP_NAME decides the config directory: ~/.config/seedapp.
const APP_NAME = "seedapp"

// defaultSettings is the file the application ships as its documented default.
// Keeping it a real .json file (rather than a Go string literal) means editors,
// linters and diffs treat it as configuration.
//
// The embed directive is resolved by the compiler: the bytes are linked into
// the binary, so the shipped executable needs neither settings.example.json nor
// its own source tree at runtime. Two constraints follow from that:
//
//   - the file must exist at *build* time, and must live in this package's own
//     directory or below it — "//go:embed ../settings.example.json" does not
//     compile. To embed a file from the repository root, put the directive in
//     the root package (usually main) and pass the bytes down.
//   - changing the file requires a rebuild; it is release content, not
//     configuration the operator can edit in place.
//
// RegisterDefaultConfig takes plain []byte, so embedding is a convention rather
// than a requirement. Any source works — an inline literal when the default is
// two lines and a separate file would be noise:
//
//	cmd.MustRegisterDefaultConfig("settings.json", []byte(`{"log_level":"info"}`))
//
// as does generated code, or content assembled at startup.
//
//go:embed settings.example.json
var defaultSettings []byte

var RootCmd = &cobra.Command{
	Use:   APP_NAME,
	Short: "Sample CLI that seeds its own default configuration",
}

func init() {
	// Hand the embedded content to the SDK once, at startup. Register (rather
	// than Set) so that if a dependency ever seeds settings.json too, the
	// conflict surfaces immediately instead of being decided by init() order.
	//
	// Two config packages are in play here and the aliases keep them apart:
	// cfgcmd (gosdk/cmd/config) is the command's logic, config (gosdk/config)
	// is the loader that decides where ~/.config/seedapp lives.
	cfgcmd.MustRegisterDefault("settings.json", defaultSettings)

	// Registering ConfigCmd brings its "default" subcommand along.
	RootCmd.AddCommand(cmd.ConfigCmd)
}

// run is the application entry point: load the config so GetAppConfigDir()
// resolves, then let cobra dispatch.
//
// Taking args instead of reading os.Args is what lets an upstream application —
// or a test — drive the command programmatically:
//
//	run([]string{"config", "default"}, os.Stdout)
func run(args []string, out io.Writer) error {
	// WithAppName is what makes ~/.config/seedapp the config directory; without
	// it "config default" has nowhere to write and says so.
	config.Default(config.WithAppName(APP_NAME))

	RootCmd.SetArgs(args)
	RootCmd.SetOut(out)
	RootCmd.SetErr(out)
	return RootCmd.Execute()
}
