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