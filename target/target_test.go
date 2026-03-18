package target_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/rdark/cdagee/target"
)

func TestParseConfigEmpty(t *testing.T) {
	cfg, err := target.ParseConfig(strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.DependsOn) != 0 {
		t.Errorf("expected empty depends_on, got %v", cfg.DependsOn)
	}
	if len(cfg.Tags) != 0 {
		t.Errorf("expected empty tags, got %v", cfg.Tags)
	}
}

func TestParseConfigFull(t *testing.T) {
	input := `{"depends_on": ["a", "b"], "tags": ["infra", "aws"]}`
	cfg, err := target.ParseConfig(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.DependsOn) != 2 || cfg.DependsOn[0] != "a" || cfg.DependsOn[1] != "b" {
		t.Errorf("unexpected depends_on: %v", cfg.DependsOn)
	}
	if len(cfg.Tags) != 2 || cfg.Tags[0] != "infra" || cfg.Tags[1] != "aws" {
		t.Errorf("unexpected tags: %v", cfg.Tags)
	}
}

func TestParseConfigUnknownField(t *testing.T) {
	input := `{"depends_on": [], "depnds_on": []}`
	_, err := target.ParseConfig(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for unknown field, got nil")
	}
}

func TestParseConfigTrailingContent(t *testing.T) {
	_, err := target.ParseConfig(strings.NewReader(`{}GARBAGE`))
	if err == nil {
		t.Fatal("expected error for trailing content, got nil")
	}
}

func TestParseConfigInvalidJSON(t *testing.T) {
	_, err := target.ParseConfig(strings.NewReader("{invalid"))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestParseConfigFileNotFound(t *testing.T) {
	_, err := target.ParseConfigFile("/nonexistent/cdagee.json")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	var pe *target.ParseError
	if !errors.As(err, &pe) {
		t.Errorf("expected *ParseError, got %T", err)
	}
}

func TestParseConfigFileBadJSON(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/cdagee.json"
	if err := os.WriteFile(path, []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := target.ParseConfigFile(path)
	if err == nil {
		t.Fatal("expected error for bad JSON file, got nil")
	}
	var pe *target.ParseError
	if !errors.As(err, &pe) {
		t.Errorf("expected *ParseError, got %T", err)
	}
}

func TestParseConfigWithTargets(t *testing.T) {
	input := `{"depends_on":["infra"],"tags":["deploy"],"serial":true,"targets":{"staging":{"tags":["staging"]},"prod":{"depends_on":[":staging"],"tags":["prod"]}}}`
	cfg, err := target.ParseConfig(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.DependsOn) != 1 || cfg.DependsOn[0] != "infra" {
		t.Errorf("unexpected depends_on: %v", cfg.DependsOn)
	}
	if len(cfg.Tags) != 1 || cfg.Tags[0] != "deploy" {
		t.Errorf("unexpected tags: %v", cfg.Tags)
	}
	if cfg.Serial == nil || !*cfg.Serial {
		t.Errorf("expected serial=true, got %v", cfg.Serial)
	}
	if len(cfg.Targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(cfg.Targets))
	}
	staging := cfg.Targets["staging"]
	if len(staging.Tags) != 1 || staging.Tags[0] != "staging" {
		t.Errorf("staging tags: %v", staging.Tags)
	}
	prod := cfg.Targets["prod"]
	if len(prod.DependsOn) != 1 || prod.DependsOn[0] != ":staging" {
		t.Errorf("prod depends_on: %v", prod.DependsOn)
	}
	if len(prod.Tags) != 1 || prod.Tags[0] != "prod" {
		t.Errorf("prod tags: %v", prod.Tags)
	}
}

func TestParseConfigSerialFalse(t *testing.T) {
	input := `{"serial":false,"targets":{"one":{},"two":{}}}`
	cfg, err := target.ParseConfig(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Serial == nil || *cfg.Serial {
		t.Errorf("expected serial=false, got %v", cfg.Serial)
	}
}

func TestParseConfigTargetWithUnknownField(t *testing.T) {
	// "serial" inside a target block should be rejected by DisallowUnknownFields
	input := `{"targets":{"alpha":{"serial":true}}}`
	_, err := target.ParseConfig(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for unknown field inside target, got nil")
	}
}

// --- Settings parsing ---

func TestParseSettingsEmpty(t *testing.T) {
	s, err := target.ParseSettings(strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	if s.Direnv != nil {
		t.Errorf("expected nil Direnv, got %v", *s.Direnv)
	}
}

func TestParseSettingsDirenvTrue(t *testing.T) {
	s, err := target.ParseSettings(strings.NewReader(`{"direnv": true}`))
	if err != nil {
		t.Fatal(err)
	}
	if s.Direnv == nil || !*s.Direnv {
		t.Errorf("expected direnv=true, got %v", s.Direnv)
	}
}

func TestParseSettingsDirenvFalse(t *testing.T) {
	s, err := target.ParseSettings(strings.NewReader(`{"direnv": false}`))
	if err != nil {
		t.Fatal(err)
	}
	if s.Direnv == nil || *s.Direnv {
		t.Errorf("expected direnv=false, got %v", s.Direnv)
	}
}

func TestParseSettingsUnknownField(t *testing.T) {
	_, err := target.ParseSettings(strings.NewReader(`{"direnv": true, "bogus": 42}`))
	if err == nil {
		t.Fatal("expected error for unknown field, got nil")
	}
}

func TestParseSettingsTrailingContent(t *testing.T) {
	_, err := target.ParseSettings(strings.NewReader(`{}GARBAGE`))
	if err == nil {
		t.Fatal("expected error for trailing content, got nil")
	}
}

func TestParseSettingsFileNotFound(t *testing.T) {
	_, err := target.ParseSettingsFile("/nonexistent/cdagee.json")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	var pe *target.ParseError
	if !errors.As(err, &pe) {
		t.Errorf("expected *ParseError, got %T", err)
	}
}

func TestParseConfigDirenvField(t *testing.T) {
	input := `{"direnv": true, "tags": ["aws"]}`
	cfg, err := target.ParseConfig(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Direnv == nil || !*cfg.Direnv {
		t.Errorf("expected direnv=true, got %v", cfg.Direnv)
	}
	if len(cfg.Tags) != 1 || cfg.Tags[0] != "aws" {
		t.Errorf("unexpected tags: %v", cfg.Tags)
	}
}

func TestParseConfigInvalidTargetName(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"colon", `{"targets":{"a:b":{}}}`},
		{"slash", `{"targets":{"a/b":{}}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := target.ParseConfig(strings.NewReader(tt.input))
			if err == nil {
				t.Fatal("expected error for invalid target name, got nil")
			}
		})
	}
}
