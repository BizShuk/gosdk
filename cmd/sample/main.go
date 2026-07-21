// Sample CLI showing how to instrument a cobra-based tool with the
// gosdk/metric cobra hook.
//
// Every execution emits a command_line_trigger{cmd=..., flag=...}
// metric via Prometheus remote-write to the backend configured by
// METRIC_URL (default: VictoriaMetrics at localhost:8428/api/v1/write).
//
// Try:
//
//	go run ./cmd/sample deploy local --env=prod
//	go run ./cmd/sample deploy cloud --region=eu --verbose
//	METRIC_URL=http://victoria:8428/api/v1/write go run ./cmd/sample deploy local
//
// Commands are declared as package-level values and wired up in init().
// Package-level variables are fully initialised before any init() runs, so an
// init() may reference any command in the file regardless of ordering.
package main

import (
	"fmt"
	"os"

	"github.com/bizshuk/gosdk/metric"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var RootCmd = &cobra.Command{
	Use:   "myapp",
	Short: "Sample CLI instrumented with the gosdk/metric cobra hook",
	Long: `myapp is a tiny demo. Run any subcommand and the gosdk/metric
cobra hook emits a single command_line_trigger{cmd=..., flag=...}
metric describing the invocation.`,
}

func init() {
	// Persistent flags live on the root and are inherited by every
	// subcommand. The hook will collect them as long as the user
	// actually sets them.
	RootCmd.PersistentFlags().String("env", "dev", "environment (dev/staging/prod)")
	RootCmd.PersistentFlags().Bool("verbose", false, "verbose output")

	RootCmd.AddCommand(DeployCmd)

	// === The whole point of this sample: one line wires the hook in. ===
	metric.CobraCMDHook(RootCmd)
}

var DeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy commands",
}

func init() {
	// Persistent flag — inherited by both `local` and `cloud`. Cobra
	// only forwards flags it knows about, so making this persistent
	// is what lets `--region=eu` work no matter which leaf you pick.
	DeployCmd.PersistentFlags().String("region", "us-west", "deployment region")

	DeployCmd.AddCommand(DeployLocalCmd, DeployCloudCmd)
}

var DeployLocalCmd = &cobra.Command{
	Use:   "local",
	Short: "Deploy locally",
	RunE: func(cmd *cobra.Command, args []string) error {
		env, _ := cmd.Flags().GetString("env")
		region, _ := cmd.Flags().GetString("region")
		verbose, _ := cmd.Flags().GetBool("verbose")
		fmt.Printf("→ deploying locally: env=%s region=%s verbose=%v\n",
			env, region, verbose)
		return nil
	},
}

var DeployCloudCmd = &cobra.Command{
	Use:   "cloud",
	Short: "Deploy to cloud",
	RunE: func(cmd *cobra.Command, args []string) error {
		env, _ := cmd.Flags().GetString("env")
		region, _ := cmd.Flags().GetString("region")
		verbose, _ := cmd.Flags().GetBool("verbose")
		fmt.Printf("→ deploying to cloud: env=%s region=%s verbose=%v\n",
			env, region, verbose)
		return nil
	},
}

func main() {
	// Let METRIC_URL from the environment override the viper default,
	// since this sample does not load any config files.
	viper.AutomaticEnv()

	if err := RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
