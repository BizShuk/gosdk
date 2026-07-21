package gotmpl

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bizshuk/gosdk/cmd/sample/gotmpl/cmd"
)

func TestRun(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	configPath := filepath.Join(filepath.Dir(sourceFile), "config.yaml")
	cmdArgs := []string{"--config", configPath}
	defer cmd.RootCmd.SetArgs(nil)

	output := captureStdout(t, func() {
		if err := run(cmdArgs); err != nil {
			t.Fatalf("run failed: %v", err)
		}
	})

	for _, want := range []string{
		"SampleEvent",
		"Customized:true",
		"ConfigA:true",
		"ConfigB:true",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("generated output missing %q:\n%s", want, output)
		}
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}

	original := os.Stdout
	os.Stdout = writer
	fn()
	os.Stdout = original

	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout pipe: %v", err)
	}
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read captured stdout: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close stdout pipe: %v", err)
	}
	return string(output)
}
