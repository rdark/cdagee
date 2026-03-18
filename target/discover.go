package target

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// DiscoverResult holds the outcome of a Discover call.
type DiscoverResult struct {
	Targets  []Target
	Settings Settings
}

// Discover walks rootDir and returns all targets found by locating
// cdagee.json marker files. Hidden directories (starting with '.') are skipped.
// Symlinks are not followed. An cdagee.json in rootDir itself is parsed as
// root-level Settings (not a target). A malformed cdagee.json halts discovery.
// Results are returned in lexicographic walk order.
func Discover(rootDir string) (DiscoverResult, error) {
	rootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return DiscoverResult{}, err
	}

	// Parse root-level settings if present.
	var settings Settings
	rootConfig := filepath.Join(rootDir, ConfigFile)
	if _, statErr := os.Stat(rootConfig); statErr == nil {
		settings, err = ParseSettingsFile(rootConfig)
		if err != nil {
			return DiscoverResult{}, err
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return DiscoverResult{}, statErr
	}

	var targets []Target
	err = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") && path != rootDir {
				return filepath.SkipDir
			}
			return nil
		}

		if d.Name() != ConfigFile {
			return nil
		}

		dir := filepath.Dir(path)
		if dir == rootDir {
			return nil // root cdagee.json is settings, not a target
		}

		cfg, err := ParseConfigFile(path)
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(rootDir, dir)
		if err != nil {
			return err
		}
		// Use forward slashes for the target ID, even on Windows.
		id := filepath.ToSlash(rel)

		// Resolve direnv: per-directory override takes precedence over root settings.
		direnv := resolveDirenv(settings, cfg)

		if len(cfg.Targets) > 0 {
			serial := cfg.Serial != nil && *cfg.Serial // default false
			names := sortedKeys(cfg.Targets)
			for _, name := range names {
				tc := cfg.Targets[name]
				resolved := resolveTargetConfig(id, cfg, tc)
				targets = append(targets, Target{
					ID:     id + ":" + name,
					Dir:    dir,
					Serial: serial,
					Direnv: direnv,
					Config: resolved,
				})
			}
		} else {
			targets = append(targets, Target{
				ID:     id,
				Dir:    dir,
				Direnv: direnv,
				Config: TargetConfig{
					DependsOn: cfg.DependsOn,
					Tags:      cfg.Tags,
				},
			})
		}
		return nil
	})
	if err != nil {
		return DiscoverResult{}, err
	}
	return DiscoverResult{Targets: targets, Settings: settings}, nil
}

// resolveDirenv returns the effective direnv setting for a target directory.
// Per-directory Config.Direnv overrides root Settings.Direnv.
func resolveDirenv(s Settings, cfg Config) bool {
	if cfg.Direnv != nil {
		return *cfg.Direnv
	}
	if s.Direnv != nil {
		return *s.Direnv
	}
	return false
}

// resolveTargetConfig merges directory-level config with per-target config.
// Colon-prefixed deps (e.g. ":staging") are expanded to "dirID:staging".
// Duplicate dependencies are removed (first occurrence wins).
func resolveTargetConfig(dirID string, dirCfg Config, tc TargetConfig) TargetConfig {
	seen := make(map[string]struct{}, len(dirCfg.DependsOn)+len(tc.DependsOn))
	var deps []string
	for _, dep := range dirCfg.DependsOn {
		if _, ok := seen[dep]; !ok {
			seen[dep] = struct{}{}
			deps = append(deps, dep)
		}
	}
	for _, dep := range tc.DependsOn {
		if strings.HasPrefix(dep, ":") {
			dep = dirID + dep
		}
		if _, ok := seen[dep]; !ok {
			seen[dep] = struct{}{}
			deps = append(deps, dep)
		}
	}

	var tags []string
	tags = append(tags, dirCfg.Tags...)
	tags = append(tags, tc.Tags...)

	return TargetConfig{
		DependsOn: deps,
		Tags:      tags,
	}
}

func sortedKeys(m map[string]TargetConfig) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
