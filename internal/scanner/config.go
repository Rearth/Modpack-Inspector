package scanner

import (
	"os"
	"path/filepath"
	"strings"

	"modpacktool/internal/db"
)

var configExtensions = map[string]bool{
	".toml": true, ".json": true, ".cfg": true, ".yml": true,
	".yaml": true, ".properties": true, ".conf": true, ".json5": true,
}

// ConfigFile represents a discovered config file.
type ConfigFile struct {
	Path         string `json:"path"`         // relative to config dir
	AbsolutePath string `json:"absolutePath"` // full path on disk
	FileName     string `json:"fileName"`
}

// ScanConfigFolder recursively finds all config files.
func ScanConfigFolder(configDir string) ([]ConfigFile, error) {
	var configs []ConfigFile

	err := filepath.WalkDir(configDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if !configExtensions[ext] {
			return nil
		}
		rel, _ := filepath.Rel(configDir, path)
		configs = append(configs, ConfigFile{
			Path:         filepath.ToSlash(rel),
			AbsolutePath: path,
			FileName:     d.Name(),
		})
		return nil
	})

	return configs, err
}

// MatchConfigsToMods scores each config file against each mod.
// Manual overrides from the DB take absolute priority.
func MatchConfigsToMods(configs []ConfigFile, mods []db.Mod, manualOverrides []db.ConfigMapping, onProgress func(current, total int)) []db.ConfigMapping {
	// Build a lookup of manual overrides
	manualSet := make(map[string]bool)
	var results []db.ConfigMapping
	total := len(configs)

	for _, o := range manualOverrides {
		manualSet[o.ConfigPath+"|"+o.ModID] = true
		results = append(results, o)
	}

	for i, cfg := range configs {
		for _, mod := range mods {
			key := cfg.Path + "|" + mod.ID
			if manualSet[key] {
				continue // already have a manual mapping
			}

			score := scoreMatch(cfg, mod)
			if score <= 0 {
				continue
			}

			results = append(results, db.ConfigMapping{
				ConfigPath: cfg.Path,
				ModID:      mod.ID,
				Confidence: score,
				IsManual:   false,
			})
		}
		if onProgress != nil {
			onProgress(i+1, total)
		}
	}

	return results
}

// scoreMatch calculates a confidence score (0-100) for how likely a config belongs to a mod.
func scoreMatch(cfg ConfigFile, mod db.Mod) float64 {
	modID := strings.ToLower(mod.ID)
	modName := strings.ToLower(mod.Name)
	cfgName := strings.ToLower(strings.TrimSuffix(cfg.FileName, filepath.Ext(cfg.FileName)))
	cfgPath := strings.ToLower(cfg.Path)

	// Exact mod ID match in filename
	if cfgName == modID || cfgName == modID+"-common" || cfgName == modID+"-client" || cfgName == modID+"-server" {
		return 95
	}

	// Config is in a directory named after the mod ID
	dir := strings.ToLower(filepath.Dir(cfg.Path))
	if dir == modID || strings.HasPrefix(dir, modID+"/") {
		return 90
	}

	// Filename starts with mod ID
	if strings.HasPrefix(cfgName, modID) {
		return 85
	}

	// Exact mod name match (not ID)
	if modName != "" && cfgName == strings.ToLower(strings.ReplaceAll(modName, " ", "")) {
		return 80
	}

	// Config path contains mod ID
	if strings.Contains(cfgPath, modID) {
		return 70
	}

	// Filename contains mod ID
	if strings.Contains(cfgName, modID) {
		return 60
	}

	// Token-based partial matching
	modTokens := tokenize(modID)
	cfgTokens := tokenize(cfgName)
	if len(modTokens) > 0 && len(cfgTokens) > 0 {
		matched := 0
		for _, mt := range modTokens {
			for _, ct := range cfgTokens {
				if mt == ct && len(mt) > 2 {
					matched++
				}
			}
		}
		if matched > 0 {
			ratio := float64(matched) / float64(len(modTokens))
			if ratio >= 0.5 {
				return 30 + ratio*20
			}
		}
	}

	return 0
}

// tokenize splits a string into lowercase tokens on common delimiters.
func tokenize(s string) []string {
	s = strings.ToLower(s)
	f := func(c rune) bool {
		return c == '-' || c == '_' || c == '.' || c == ' '
	}
	tokens := strings.FieldsFunc(s, f)
	return tokens
}
