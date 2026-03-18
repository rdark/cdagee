// Package target provides discovery, parsing, and dependency graph
// construction for targets identified by cdagee.json marker files.
package target

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// ConfigFile is the marker file name that identifies a target directory.
const ConfigFile = "cdagee.json"

// Settings holds root-level configuration from the cdagee.json in the root directory.
// This is separate from target Config — the root cdagee.json configures discovery-wide
// behaviour rather than defining a target.
type Settings struct {
	Direnv *bool `json:"direnv,omitempty"`
}

// Config is the file-level contents of cdagee.json.
type Config struct {
	DependsOn []string               `json:"depends_on,omitempty"`
	Tags      []string               `json:"tags,omitempty"`
	Serial    *bool                  `json:"serial,omitempty"`
	Direnv    *bool                  `json:"direnv,omitempty"`
	Targets   map[string]TargetConfig `json:"targets,omitempty"`
}

// TargetConfig is the resolved per-target configuration.
type TargetConfig struct {
	DependsOn []string `json:"depends_on,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}

// Target represents a discovered target directory.
type Target struct {
	// ID is the target identifier: the relative path from root using forward slashes.
	// For multi-target directories, the format is "dir:name".
	ID string
	// Dir is the absolute filesystem path to the target directory.
	Dir string
	// Serial is true when targets sharing Dir must run one at a time.
	Serial bool
	// Direnv indicates whether commands should be wrapped with "direnv exec".
	// Resolved from root Settings (default) and per-directory Config override.
	Direnv bool
	// Config is the resolved per-target configuration.
	Config TargetConfig
}

// ParseConfig decodes a Config from r.
// Unknown JSON fields and trailing content after the JSON object are rejected.
func ParseConfig(r io.Reader) (Config, error) {
	var cfg Config
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	if dec.More() {
		return Config{}, fmt.Errorf("decode config: unexpected trailing content")
	}
	if err := validateConfig(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// ParseConfigFile reads and parses an cdagee.json file at the given path.
// Errors are wrapped as *ParseError.
func ParseConfigFile(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, &ParseError{Path: path, Err: err}
	}
	defer f.Close() //nolint:errcheck // read-only file
	cfg, err := ParseConfig(f)
	if err != nil {
		return Config{}, &ParseError{Path: path, Err: err}
	}
	return cfg, nil
}

// ParseSettings decodes a Settings from r.
// Unknown JSON fields and trailing content after the JSON object are rejected.
func ParseSettings(r io.Reader) (Settings, error) {
	var s Settings
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return Settings{}, fmt.Errorf("decode settings: %w", err)
	}
	if dec.More() {
		return Settings{}, fmt.Errorf("decode settings: unexpected trailing content")
	}
	return s, nil
}

// ParseSettingsFile reads and parses a root cdagee.json file as Settings.
// Errors are wrapped as *ParseError.
func ParseSettingsFile(path string) (Settings, error) {
	f, err := os.Open(path)
	if err != nil {
		return Settings{}, &ParseError{Path: path, Err: err}
	}
	defer f.Close() //nolint:errcheck // read-only file
	s, err := ParseSettings(f)
	if err != nil {
		return Settings{}, &ParseError{Path: path, Err: err}
	}
	return s, nil
}

// validateConfig checks that target names in cfg.Targets are valid.
func validateConfig(cfg Config) error {
	for name := range cfg.Targets {
		if name == "" {
			return fmt.Errorf("empty target name")
		}
		if strings.ContainsAny(name, ":/") {
			return fmt.Errorf("invalid target name %q: must not contain ':' or '/'", name)
		}
	}
	return nil
}
