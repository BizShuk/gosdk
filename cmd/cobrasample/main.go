// Sample CLI showing how to instrument a cobra-based tool with the
// gosdk/metric cobra hook.
//
// Every execution emits a command_line_trigger{cmd=..., flag=...}
// metric via Prometheus remote-write to the backend configured by
// METRIC_URL (default: VictoriaMetrics at localhost:8428/api/v1/write).
//
// Try:
//
//	go run ./cmd/cobrasample deploy local --env=prod
//	go run ./cmd/cobrasample deploy cloud --region=eu --verbose
//	METRIC_URL=http://victoria:8428/api/v1/write go run ./cmd/cobrasample deploy local
package main

import (
	"fmt"
	"os"

	"github.com/bizshuk/gosdk/metric"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func main() {
	// Let METRIC_URL from the environment override the viper default,
	// since this sample does not load any config files.
	viper.AutomaticEnv()

	root := newRootCmd()
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "myapp",
		Short: "Sample CLI instrumented with the gosdk/metric cobra hook",
		Long: `myapp is a tiny demo. Run any subcommand and the gosdk/metric
cobra hook emits a single command_line_trigger{cmd=..., flag=...}
metric describing the invocation.`,
	}

	// Persistent flags live on the root and are inherited by every
	// subcommand. The hook will collect them as long as the user
	// actually sets them.
	root.PersistentFlags().String("env", "dev", "environment (dev/staging/prod)")
	root.PersistentFlags().Bool("verbose", false, "verbose output")

	root.AddCommand(newDeployCmd())

	// === The whole point of this sample: one line wires the hook in. ===
	metric.CobraCMDHook(root)

	return root
}

func newDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy commands",
	}
	// Persistent flag — inherited by both `local` and `cloud`. Cobra
	// only forwards flags it knows about, so making this persistent
	// is what lets `--region=eu` work no matter which leaf you pick.
	cmd.PersistentFlags().String("region", "us-west", "deployment region")
	cmd.AddCommand(newDeployLocalCmd(), newDeployCloudCmd())
	return cmd
}

func newDeployLocalCmd() *cobra.Command {
	return &cobra.Command{
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
}

func newDeployCloudCmd() *cobra.Command {
	return &cobra.Command{
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
}
