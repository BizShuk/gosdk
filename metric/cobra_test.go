package metric

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// captureServer records every POST body so tests can decode and assert
// on the metrics that were emitted.
type captureServer struct {
	mu     sync.Mutex
	bodies [][]byte
}

func (c *captureServer) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		c.mu.Lock()
		c.bodies = append(c.bodies, body)
		c.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
}

func (c *captureServer) lastBody() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.bodies) == 0 {
		return nil
	}
	return c.bodies[len(c.bodies)-1]
}

// setupSink starts a capturing remote-write server and points METRIC_URL
// at it, so the hook's NewMetricService("") emits there.
func setupSink(t *testing.T) *captureServer {
	t.Helper()
	cap := &captureServer{}
	ts := httptest.NewServer(cap.handler())
	t.Cleanup(ts.Close)

	prev := viper.GetString("METRIC_URL")
	viper.Set("METRIC_URL", ts.URL)

	// Send() lazy-inits a package-level MetricService singleton on first
	// call using the current METRIC_URL. If a previous test already
	// triggered that lazy init, the singleton still points at that
	// test's (now-closed) httptest server, so subsequent tests would
	// silently fail to send. Reset it so each test's Send() rebinds to
	// this test's server.
	prevSvc := globalMetricService
	globalMetricService = nil
	t.Cleanup(func() {
		viper.Set("METRIC_URL", prev)
		globalMetricService = prevSvc
	})
	return cap
}

// newTestRootCmd builds:
//
//	myapp         --app (persistent string, inherited by all children)
//	  └─ sub        --verbose (persistent bool, inherited by action)
//	       └─ action --name (local string)
//
// --app and --verbose are persistent so cobra's flag forwarder (in
// Find/stripFlags) propagates them to the leaf when typed at any level.
func newTestRootCmd() *cobra.Command {
	root := &cobra.Command{Use: "myapp"}
	root.PersistentFlags().String("app", "", "app flag (inherited)")
	sub := &cobra.Command{
		Use:  "sub",
		RunE: func(cmd *cobra.Command, args []string) error { return nil },
	}
	sub.PersistentFlags().Bool("verbose", false, "verbose flag (inherited)")
	action := &cobra.Command{
		Use:  "action",
		RunE: func(cmd *cobra.Command, args []string) error { return nil },
	}
	action.Flags().String("name", "", "name flag")
	sub.AddCommand(action)
	root.AddCommand(sub)
	return root
}

func decodeLastMetric(t *testing.T, cap *captureServer) *prompb.TimeSeries {
	t.Helper()
	body := cap.lastBody()
	if body == nil {
		t.Fatal("no metric emitted")
	}
	decompressed, err := snappy.Decode(nil, body)
	if err != nil {
		t.Fatalf("snappy decode: %v\nbody=%q", err, body)
	}
	req := &prompb.WriteRequest{}
	if err := req.Unmarshal(decompressed); err != nil {
		t.Fatalf("unmarshal: %v\ndecompressed=%q", err, decompressed)
	}
	if len(req.Timeseries) != 1 {
		t.Fatalf("expected 1 time series, got %d", len(req.Timeseries))
	}
	return &req.Timeseries[0]
}

func labelsToMap(ts *prompb.TimeSeries) map[string]string {
	m := make(map[string]string, len(ts.Labels))
	for _, l := range ts.Labels {
		m[l.Name] = l.Value
	}
	return m
}

func TestCobraHook_EmitsCommandChainAndFlag(t *testing.T) {
	cap := setupSink(t)
	root := newTestRootCmd()
	CobraCMDHook(root)

	root.SetArgs([]string{"sub", "action", "--name=foo"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	ts := decodeLastMetric(t, cap)
	labels := labelsToMap(ts)
	if got, want := labels["__name__"], COBRA_HOOK_METRIC_NAME; got != want {
		t.Errorf("metric name: got %q, want %q", got, want)
	}
	if got, want := labels["cmd"], "myapp sub action"; got != want {
		t.Errorf("cmd: got %q, want %q", got, want)
	}
	if got, want := labels["flag"], "name"; got != want {
		t.Errorf("flag: got %q, want %q", got, want)
	}
}

func TestCobraHook_FlagsSortedAcrossParentChain(t *testing.T) {
	cap := setupSink(t)
	root := newTestRootCmd()
	CobraCMDHook(root)

	// --app is a persistent flag on root, --verbose is local to sub,
	// --name is local to action. The hook must collect all three in
	// alphabetical order joined with "-".
	root.SetArgs([]string{"--app=cli", "sub", "--verbose", "action", "--name=foo"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	ts := decodeLastMetric(t, cap)
	labels := labelsToMap(ts)
	if got, want := labels["flag"], "app-name-verbose"; got != want {
		t.Errorf("flag: got %q, want %q", got, want)
	}
}

func TestCobraHook_EmptyFlag(t *testing.T) {
	cap := setupSink(t)
	root := newTestRootCmd()
	CobraCMDHook(root)

	root.SetArgs([]string{"sub"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	ts := decodeLastMetric(t, cap)
	labels := labelsToMap(ts)
	if got, want := labels["cmd"], "myapp sub"; got != want {
		t.Errorf("cmd: got %q, want %q", got, want)
	}
	if got, want := labels["flag"], ""; got != want {
		t.Errorf("flag: got %q, want %q", got, want)
	}
}

func TestCobraHook_PreservesExistingPreRun(t *testing.T) {
	setupSink(t)
	root := newTestRootCmd()

	preRan := false
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		preRan = true
		return nil
	}
	CobraCMDHook(root)

	root.SetArgs([]string{"sub"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if !preRan {
		t.Error("existing PersistentPreRunE was not called")
	}
}

func TestCobraHook_PreservesExistingPreRunError(t *testing.T) {
	setupSink(t)
	root := newTestRootCmd()

	sentinel := errors.New("boom")
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		return sentinel
	}
	CobraCMDHook(root)

	root.SetArgs([]string{"sub"})
	if err := root.Execute(); !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
}
