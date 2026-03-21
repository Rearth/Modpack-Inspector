package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"modpacktool/internal/db"
)

func TestScanConfigFolder(t *testing.T) {
	dir := t.TempDir()

	// Create some config files
	os.WriteFile(filepath.Join(dir, "mymod.toml"), []byte("key=1"), 0644)
	os.WriteFile(filepath.Join(dir, "other.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not a config"), 0644) // .txt not in extensions

	sub := filepath.Join(dir, "subdir")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(sub, "nested.yml"), []byte("a: b"), 0644)

	configs, err := ScanConfigFolder(dir)
	if err != nil {
		t.Fatalf("ScanConfigFolder() error: %v", err)
	}

	// Should find .toml, .json, .yml but NOT .txt
	if len(configs) != 3 {
		t.Errorf("expected 3 configs, got %d", len(configs))
		for _, c := range configs {
			t.Logf("  %s", c.Path)
		}
	}

	// Check relative paths use forward slashes
	for _, c := range configs {
		if filepath.IsAbs(c.Path) {
			t.Errorf("config path should be relative: %s", c.Path)
		}
	}
}

func TestScoreMatch(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ConfigFile
		modID   string
		modName string
		minConf float64
		maxConf float64
	}{
		{
			name:    "exact ID match",
			cfg:     ConfigFile{Path: "mymod.toml", FileName: "mymod.toml"},
			modID:   "mymod",
			minConf: 90,
			maxConf: 100,
		},
		{
			name:    "common suffix",
			cfg:     ConfigFile{Path: "mymod-common.toml", FileName: "mymod-common.toml"},
			modID:   "mymod",
			minConf: 90,
			maxConf: 100,
		},
		{
			name:    "directory match",
			cfg:     ConfigFile{Path: "mymod/settings.toml", FileName: "settings.toml"},
			modID:   "mymod",
			minConf: 85,
			maxConf: 95,
		},
		{
			name:    "prefix match",
			cfg:     ConfigFile{Path: "mymod_extra.toml", FileName: "mymod_extra.toml"},
			modID:   "mymod",
			minConf: 80,
			maxConf: 90,
		},
		{
			name:    "path contains mod ID",
			cfg:     ConfigFile{Path: "config/mymod/stuff.toml", FileName: "stuff.toml"},
			modID:   "mymod",
			minConf: 60,
			maxConf: 80,
		},
		{
			name:    "no match",
			cfg:     ConfigFile{Path: "totally-different.toml", FileName: "totally-different.toml"},
			modID:   "mymod",
			minConf: 0,
			maxConf: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mod := db.Mod{ID: tt.modID, Name: tt.modName}
			score := scoreMatch(tt.cfg, mod)
			if score < tt.minConf || score > tt.maxConf {
				t.Errorf("score %f not in expected range [%f, %f]", score, tt.minConf, tt.maxConf)
			}
		})
	}
}

func TestMatchConfigsToMods(t *testing.T) {
	configs := []ConfigFile{
		{Path: "jei-common.toml", FileName: "jei-common.toml"},
		{Path: "waystones.toml", FileName: "waystones.toml"},
		{Path: "unrelated.toml", FileName: "unrelated.toml"},
	}
	mods := []db.Mod{
		{ID: "jei", Name: "Just Enough Items"},
		{ID: "waystones", Name: "Waystones"},
	}

	results := MatchConfigsToMods(configs, mods, nil, nil)

	// Should have matches for jei-common -> jei and waystones -> waystones
	jeiMatch := false
	waystonesMatch := false
	for _, r := range results {
		if r.ModID == "jei" && r.ConfigPath == "jei-common.toml" {
			jeiMatch = true
			if r.Confidence < 80 {
				t.Errorf("expected high confidence for jei, got %f", r.Confidence)
			}
		}
		if r.ModID == "waystones" && r.ConfigPath == "waystones.toml" {
			waystonesMatch = true
			if r.Confidence < 90 {
				t.Errorf("expected high confidence for waystones, got %f", r.Confidence)
			}
		}
	}
	if !jeiMatch {
		t.Error("expected jei-common.toml to match jei")
	}
	if !waystonesMatch {
		t.Error("expected waystones.toml to match waystones")
	}
}

func TestMatchConfigsToModsManualOverride(t *testing.T) {
	configs := []ConfigFile{
		{Path: "custom.toml", FileName: "custom.toml"},
	}
	mods := []db.Mod{
		{ID: "mymod", Name: "My Mod"},
	}
	overrides := []db.ConfigMapping{
		{ConfigPath: "custom.toml", ModID: "mymod", Confidence: 100, IsManual: true},
	}

	results := MatchConfigsToMods(configs, mods, overrides, nil)

	manualFound := false
	for _, r := range results {
		if r.ConfigPath == "custom.toml" && r.ModID == "mymod" {
			if !r.IsManual {
				t.Error("expected manual override to be preserved")
			}
			manualFound = true
		}
	}
	if !manualFound {
		t.Error("manual override not found in results")
	}
}

func TestScanConfigFolderReal(t *testing.T) {
	configDir := `C:\Users\Darkp\AppData\Roaming\ModrinthApp\profiles\pack testing\config`
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		t.Skip("test config folder not available")
	}

	configs, err := ScanConfigFolder(configDir)
	if err != nil {
		t.Fatalf("ScanConfigFolder() error: %v", err)
	}

	t.Logf("Found %d config files in real modpack", len(configs))
	if len(configs) == 0 {
		t.Error("expected at least some config files")
	}
}
