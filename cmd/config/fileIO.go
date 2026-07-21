package config

import (
	"fmt"
	"os"
	"path/filepath"

	sdkconfig "github.com/bizshuk/gosdk/config"
)

// configData is the in-memory representation of a config file. The same type
// is used for every supported format — json and yaml nest via map[string]any
// values, while env stays one level deep with the env var name as the key.
// Keeping a single type lets the mutation helpers in key.go work on a document
// without knowing which format it came from.
type configData = map[string]any

// This file is the format dispatcher, and nothing more. How a given format
// decodes and encodes belongs to the SDK's config package, next to the loader
// for that same format — sdkconfig.ParseJSON lives beside JsonConfig in
// config/json.go, ParseYAML beside YamlConfig, ParseEnv beside EnvConfig. What
// belongs here is the file-level policy those functions deliberately do not
// have: which format a file name implies, that a missing file reads as an
// empty document, and that a parent directory is created on write.

// readConfigFile loads a config file from disk. A missing file is not an
// error: the caller treats it as an empty document and the next
// writeConfigFile call creates the file.
func readConfigFile(path, fileName string) (configData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return configData{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	m, err := parseConfigBytes(data, fileName)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return m, nil
}

// codec is the pair of functions the SDK exposes for one config format.
// Pairing them in a table rather than repeating a switch per direction means a
// new format is one entry, and a format can never be readable but not
// writable.
type codec struct {
	parse func([]byte) (configData, error)
	write func(string, configData) error
}

var codecs = map[string]codec{
	"json": {sdkconfig.ParseJSON, sdkconfig.WriteJSON},
	"yaml": {sdkconfig.ParseYAML, sdkconfig.WriteYAML},
	"env":  {sdkconfig.ParseEnv, sdkconfig.WriteEnv},
}

// codecFor resolves the codec a file name implies. Only a name that passed
// normalizeFile reaches here, so an unknown format is a bug in this package
// rather than bad user input — but it is still reported instead of panicking.
func codecFor(fileName string) (codec, error) {
	c, ok := codecs[fileFormat(fileName)]
	if !ok {
		return codec{}, fmt.Errorf("unsupported config file %q", fileName)
	}
	return c, nil
}

// parseConfigBytes decodes in-memory content in the format implied by
// fileName. It is readConfigFile's counterpart for content that never reached
// the disk — an embedded seed default, most notably — so both paths agree on
// how a given format decodes.
func parseConfigBytes(data []byte, fileName string) (configData, error) {
	c, err := codecFor(fileName)
	if err != nil {
		return nil, err
	}
	return c.parse(data)
}

// writeConfigFile serialises data to path in the format implied by fileName,
// creating the parent directory when it does not exist.
func writeConfigFile(path, fileName string, data configData) error {
	c, err := codecFor(fileName)
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	return c.write(path, data)
}
