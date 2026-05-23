# Versioning CLI Tool Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create `versioning` CLI tool with `major`, `minor`, `patch` subcommands to manage semver version in a `version` file.

**Architecture:** A Cobra-based CLI with `cmd/versioning/` containing `main.go` entry point and `cmd/` subpackage with root and subcommands. Version file is plain text in current working directory.

**Tech Stack:** Go 1.24+, spf13/cobra (already in go.mod), standard library only.

---

## File Structure

```
cmd/versioning/
├── main.go          # Entry point, calls cmd.Execute()
└── cmd/
    ├── root.go      # Root command + version file read/write logic
    ├── major.go     # major subcommand
    ├── minor.go     # minor subcommand
    └── patch.go     # patch subcommand
```

---

## Task 1: Create cmd/versioning/main.go

**Files:**
- Create: `cmd/versioning/main.go`

- [ ] **Step 1: Write the file**

```go
package main

import "github.com/bizshuk/gosdk/cmd/versioning/cmd"

func main() {
	cmd.Execute()
}
```

- [ ] **Step 2: Commit**

```bash
git add cmd/versioning/main.go
git commit -m "feat(versioning): add main.go entry point"
```

---

## Task 2: Create cmd/versioning/cmd/root.go

**Files:**
- Create: `cmd/versioning/cmd/root.go`

- [ ] **Step 1: Write the file**

```go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const versionFile = "version"

var rootCmd = &cobra.Command{
	Use:   "versioning",
	Short: "Version management CLI tool",
	Long:  `A CLI tool to manage semver version in a version file.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

type Version struct {
	Major int
	Minor int
	Patch int
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func ParseVersion(s string) (Version, error) {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("invalid version format: %s (expected major.minor.patch)", s)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return Version{}, fmt.Errorf("invalid major version: %s", parts[0])
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return Version{}, fmt.Errorf("invalid minor version: %s", parts[1])
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return Version{}, fmt.Errorf("invalid patch version: %s", parts[2])
	}
	return Version{Major: major, Minor: minor, Patch: patch}, nil
}

func ReadVersion() (Version, error) {
	data, err := os.ReadFile(versionFile)
	if err != nil {
		if os.IsNotExist(err) {
			return Version{}, nil
		}
		return Version{}, fmt.Errorf("failed to read version file: %w", err)
	}
	version, err := ParseVersion(strings.TrimSpace(string(data)))
	if err != nil {
		return Version{}, err
	}
	return version, nil
}

func WriteVersion(v Version) error {
	dir := filepath.Dir(versionFile)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}
	if err := os.WriteFile(versionFile, []byte(v.String()), 0644); err != nil {
		return fmt.Errorf("failed to write version file: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Commit**

```bash
git add cmd/versioning/cmd/root.go
git commit -m "feat(versioning): add root command with version read/write logic"
```

---

## Task 3: Create cmd/versioning/cmd/major.go

**Files:**
- Create: `cmd/versioning/cmd/major.go`

- [ ] **Step 1: Write the file**

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var majorCmd = &cobra.Command{
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

func init() {
	rootCmd.AddCommand(majorCmd)
}
```

- [ ] **Step 2: Commit**

```bash
git add cmd/versioning/cmd/major.go
git commit -m "feat(versioning): add major subcommand"
```

---

## Task 4: Create cmd/versioning/cmd/minor.go

**Files:**
- Create: `cmd/versioning/cmd/minor.go`

- [ ] **Step 1: Write the file**

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var minorCmd = &cobra.Command{
	Use:   "minor",
	Short: "Increment minor version",
	Long:  `Increments the minor version and resets patch to 0.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		v, err := ReadVersion()
		if err != nil {
			return err
		}
		v.Minor++
		v.Patch = 0
		if err := WriteVersion(v); err != nil {
			return err
		}
		fmt.Println(v)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(minorCmd)
}
```

- [ ] **Step 2: Commit**

```bash
git add cmd/versioning/cmd/minor.go
git commit -m "feat(versioning): add minor subcommand"
```

---

## Task 5: Create cmd/versioning/cmd/patch.go

**Files:**
- Create: `cmd/versioning/cmd/patch.go`

- [ ] **Step 1: Write the file**

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var patchCmd = &cobra.Command{
	Use:   "patch",
	Short: "Increment patch version",
	Long:  `Increments the patch version.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		v, err := ReadVersion()
		if err != nil {
			return err
		}
		v.Patch++
		if err := WriteVersion(v); err != nil {
			return err
		}
		fmt.Println(v)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(patchCmd)
}
```

- [ ] **Step 2: Commit**

```bash
git add cmd/versioning/cmd/patch.go
git commit -m "feat(versioning): add patch subcommand"
```

---

## Task 6: Verify build and test

**Files:**
- Test: `cmd/versioning/cmd/`

- [ ] **Step 1: Build the tool**

Run: `go build -o bin/versioning ./cmd/versioning`
Expected: Build succeeds, `bin/versioning` binary created

- [ ] **Step 2: Test major subcommand**

```bash
echo "1.2.3" > version
./bin/versioning major
cat version
```
Expected: version file contains `2.0.0`

- [ ] **Step 3: Test minor subcommand**

```bash
echo "1.2.3" > version
./bin/versioning minor
cat version
```
Expected: version file contains `1.3.0`

- [ ] **Step 4: Test patch subcommand**

```bash
echo "1.2.3" > version
./bin/versioning patch
cat version
```
Expected: version file contains `1.2.4`

- [ ] **Step 5: Test default version (file not exists)**

```bash
rm -f version
./bin/versioning patch
cat version
```
Expected: version file contains `0.0.1`

- [ ] **Step 6: Clean up test artifact**

```bash
rm -f version bin/versioning
```

- [ ] **Step 7: Commit final verification**

```bash
git add -A
git commit -m "chore(versioning): verify build and functionality"
```

---

## Self-Review Checklist

1. **Spec coverage:**
   - [x] `major` subcommand → Task 3
   - [x] `minor` subcommand → Task 4
   - [x] `patch` subcommand → Task 5
   - [x] Version file read/write → Task 2
   - [x] Semver format → Task 2 (`ParseVersion`, `String`)
   - [x] Error handling for invalid format → Task 2
   - [x] Default to `0.0.0` if file doesn't exist → Task 2 (`ReadVersion`)
   - [x] Major resets minor/patch → Task 3
   - [x] Minor resets patch → Task 4

2. **Placeholder scan:** No TBD, TODO, or placeholder patterns found.

3. **Type consistency:** `Version` struct, `ParseVersion()`, `ReadVersion()`, `WriteVersion()` all consistently defined in Task 2 and used in Tasks 3-5.

---

**Plan complete.**