package instance

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Instance represents a detected Minecraft instance.
type Instance struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Launcher string `json:"launcher"`
	HasMods  bool   `json:"hasMods"`
}

type DetectOptions struct {
	ModrinthRoots   []string
	CurseForgeRoots []string
	FTBRoots        []string
	OtherRoots      []string
}

// DetectInstances scans common launcher paths for Minecraft instances.
func DetectInstances(opts *DetectOptions) []Instance {
	var instances []Instance

	if runtime.GOOS != "windows" {
		return instances
	}

	appData := os.Getenv("APPDATA")
	userProfile := os.Getenv("USERPROFILE")
	localAppData := os.Getenv("LOCALAPPDATA")

	// Vanilla .minecraft
	if p := filepath.Join(appData, ".minecraft"); dirExists(p) {
		instances = append(instances, Instance{
			Name:     ".minecraft (Vanilla)",
			Path:     p,
			Launcher: "vanilla",
			HasMods:  dirExists(filepath.Join(p, "mods")),
		})
	}

	// Prism Launcher
	prismDirs := []string{
		filepath.Join(appData, "PrismLauncher", "instances"),
		filepath.Join(appData, "PolyMC", "instances"),
	}
	for _, dir := range prismDirs {
		instances = append(instances, scanInstancesDir(dir, "prism")...)
	}

	// MultiMC
	multiMCDir := filepath.Join(appData, "MultiMC", "instances")
	instances = append(instances, scanInstancesDir(multiMCDir, "multimc")...)

	// CurseForge
	cfDirs := append([]string{
		filepath.Join(userProfile, "curseforge", "minecraft", "Instances"),
	}, optionRoots(opts, "curseforge")...)
	for _, dir := range cfDirs {
		instances = append(instances, scanInstancesDir(dir, "curseforge")...)
	}

	// FTB App
	ftbDirs := append([]string{
		filepath.Join(appData, "FTBA", "instances"),
		filepath.Join(localAppData, "FTBApp", "instances"),
	}, optionRoots(opts, "ftb")...)
	for _, dir := range ftbDirs {
		instances = append(instances, scanInstancesDir(dir, "ftb")...)
	}

	// ATLauncher
	atlDir := filepath.Join(appData, "ATLauncher", "instances")
	instances = append(instances, scanInstancesDir(atlDir, "atlauncher")...)

	// Modrinth App (both old theseus and new ModrinthApp paths)
	mrDirs := append([]string{
		filepath.Join(appData, "com.modrinth.theseus", "profiles"),
		filepath.Join(appData, "ModrinthApp", "profiles"),
	}, optionRoots(opts, "modrinth")...)
	for _, dir := range mrDirs {
		instances = append(instances, scanInstancesDir(dir, "modrinth")...)
	}

	// GDLauncher
	gdDir := filepath.Join(appData, "gdlauncher_next", "instances")
	instances = append(instances, scanInstancesDir(gdDir, "gdlauncher")...)

	for _, dir := range optionRoots(opts, "other") {
		instances = append(instances, scanInstancesDir(dir, "custom")...)
	}

	return dedupeInstances(instances)
}

func optionRoots(opts *DetectOptions, launcher string) []string {
	if opts == nil {
		return nil
	}
	switch launcher {
	case "modrinth":
		return opts.ModrinthRoots
	case "curseforge":
		return opts.CurseForgeRoots
	case "ftb":
		return opts.FTBRoots
	case "other":
		return opts.OtherRoots
	default:
		return nil
	}
}

func ParseRoots(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == ';'
	})
	roots := make([]string, 0, len(fields))
	seen := make(map[string]bool)
	for _, field := range fields {
		root := strings.TrimSpace(field)
		if root == "" {
			continue
		}
		normalized := strings.ToLower(filepath.Clean(root))
		if seen[normalized] {
			continue
		}
		seen[normalized] = true
		roots = append(roots, root)
	}
	return roots
}

func dedupeInstances(instances []Instance) []Instance {
	seen := make(map[string]bool)
	result := make([]Instance, 0, len(instances))
	for _, inst := range instances {
		key := strings.ToLower(filepath.Clean(inst.Path))
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, inst)
	}
	return result
}

func scanInstancesDir(dir, launcher string) []Instance {
	if !dirExists(dir) {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var instances []Instance
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Skip hidden/system directories
		if entry.Name()[0] == '.' || entry.Name()[0] == '_' {
			continue
		}

		instPath := filepath.Join(dir, entry.Name())

		// For Prism/MultiMC, the actual minecraft dir is inside .minecraft/
		mcDir := instPath
		if launcher == "prism" || launcher == "multimc" {
			subDir := filepath.Join(instPath, ".minecraft")
			if dirExists(subDir) {
				mcDir = subDir
			} else {
				subDir = filepath.Join(instPath, "minecraft")
				if dirExists(subDir) {
					mcDir = subDir
				}
			}
		}

		instances = append(instances, Instance{
			Name:     entry.Name(),
			Path:     mcDir,
			Launcher: launcher,
			HasMods:  dirExists(filepath.Join(mcDir, "mods")),
		})
	}
	return instances
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
