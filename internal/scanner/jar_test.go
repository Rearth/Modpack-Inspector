package scanner

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
)

// createTestJar creates a .jar (zip) file in dir with the given files.
func createTestJar(t *testing.T, dir, jarName string, files map[string][]byte) string {
	t.Helper()
	path := filepath.Join(dir, jarName)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create jar: %v", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	for name, content := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		fw.Write(content)
	}
	w.Close()
	return path
}

func TestScanJarFabric(t *testing.T) {
	dir := t.TempDir()

	innerFabricMod := map[string]interface{}{
		"id":      "embedded-lib",
		"version": "1.0.0",
		"name":    "Embedded Lib",
	}
	innerData, _ := json.Marshal(innerFabricMod)
	var innerBuf bytes.Buffer
	innerZip := zip.NewWriter(&innerBuf)
	innerFile, _ := innerZip.Create("fabric.mod.json")
	innerFile.Write(innerData)
	innerZip.Close()

	fabricMod := map[string]interface{}{
		"id":          "test-fabric-mod",
		"version":     "1.2.3",
		"name":        "Test Fabric Mod",
		"description": "A test fabric mod",
		"authors":     []string{"Alice", "Bob"},
		"provides":    []string{"fabric-api-base", "fabric-resource-loader-v0"},
		"depends": map[string]string{
			"fabricloader": ">=0.14.0",
			"minecraft":    "~1.20",
			"some-lib":     "*",
		},
		"suggests": map[string]string{
			"optional-mod": "*",
		},
	}
	data, _ := json.Marshal(fabricMod)

	jarPath := createTestJar(t, dir, "test-fabric-mod-1.2.3.jar", map[string][]byte{
		"fabric.mod.json":                data,
		"META-INF/jars/embedded-lib.jar": innerBuf.Bytes(),
	})

	result, err := ScanJar(jarPath)
	if err != nil {
		t.Fatalf("ScanJar() error: %v", err)
	}

	if result.Mod.ID != "test-fabric-mod" {
		t.Errorf("mod ID: got %q, want test-fabric-mod", result.Mod.ID)
	}
	if result.Mod.Name != "Test Fabric Mod" {
		t.Errorf("mod name: got %q, want Test Fabric Mod", result.Mod.Name)
	}
	if result.Mod.Version != "1.2.3" {
		t.Errorf("version: got %q, want 1.2.3", result.Mod.Version)
	}
	if result.Mod.ModLoader != "fabric" {
		t.Errorf("loader: got %q, want fabric", result.Mod.ModLoader)
	}
	if result.Mod.ProvidedIDs != "fabric-api-base,fabric-resource-loader-v0,embedded-lib" {
		t.Errorf("provided IDs: got %q", result.Mod.ProvidedIDs)
	}
	if result.Mod.JarSHA1 == "" || result.Mod.JarSHA512 == "" {
		t.Error("expected SHA hashes to be computed")
	}
	if result.Mod.Fingerprint == 0 {
		t.Error("expected fingerprint to be non-zero")
	}

	// Dependencies: should have some-lib (required) and optional-mod (optional), but NOT fabricloader/minecraft
	requiredDeps := 0
	optionalDeps := 0
	for _, dep := range result.Deps {
		if dep.DepModID == "fabricloader" || dep.DepModID == "minecraft" {
			t.Errorf("should not include loader/minecraft dep: %s", dep.DepModID)
		}
		switch dep.Type {
		case "required":
			requiredDeps++
		case "optional":
			optionalDeps++
		}
	}
	if requiredDeps != 1 {
		t.Errorf("expected 1 required dep (some-lib), got %d", requiredDeps)
	}
	if optionalDeps != 1 {
		t.Errorf("expected 1 optional dep (optional-mod), got %d", optionalDeps)
	}
}

func TestScanJarForge(t *testing.T) {
	dir := t.TempDir()

	mt := modsToml{
		ModLoader: "javafml",
		Mods: []modsTomlEntry{
			{
				ModId:       "test-forge-mod",
				Version:     "3.0.0",
				DisplayName: "Test Forge Mod",
				Description: "A forge mod for testing",
				Authors:     "Charlie",
			},
		},
		Dependencies: map[string][]modsTomlDep{
			"test-forge-mod": {
				{ModId: "forge", Mandatory: true},
				{ModId: "minecraft", Mandatory: true},
				{ModId: "jei", Mandatory: false},
			},
		},
	}
	var buf bytes.Buffer
	toml.NewEncoder(&buf).Encode(mt)

	jarPath := createTestJar(t, dir, "test-forge-mod-3.0.0.jar", map[string][]byte{
		"META-INF/mods.toml": buf.Bytes(),
	})

	result, err := ScanJar(jarPath)
	if err != nil {
		t.Fatalf("ScanJar() error: %v", err)
	}

	if result.Mod.ID != "test-forge-mod" {
		t.Errorf("mod ID: got %q", result.Mod.ID)
	}
	if result.Mod.ModLoader != "forge" {
		t.Errorf("loader: got %q, want forge", result.Mod.ModLoader)
	}
	if result.Mod.Authors != "Charlie" {
		t.Errorf("authors: got %q, want Charlie", result.Mod.Authors)
	}

	// Only jei should be in deps (forge and minecraft are filtered)
	if len(result.Deps) != 1 {
		t.Fatalf("expected 1 dep, got %d: %+v", len(result.Deps), result.Deps)
	}
	if result.Deps[0].DepModID != "jei" || result.Deps[0].Type != "optional" {
		t.Errorf("unexpected dep: %+v", result.Deps[0])
	}
}

func TestScanJarNeoForge(t *testing.T) {
	dir := t.TempDir()

	mt := modsToml{
		ModLoader: "javafml",
		Mods: []modsTomlEntry{
			{ModId: "neo-mod", Version: "1.0.0", DisplayName: "Neo Mod"},
		},
	}
	var buf bytes.Buffer
	toml.NewEncoder(&buf).Encode(mt)

	jarPath := createTestJar(t, dir, "neo-mod.jar", map[string][]byte{
		"META-INF/neoforge.mods.toml": buf.Bytes(),
	})

	result, err := ScanJar(jarPath)
	if err != nil {
		t.Fatalf("ScanJar() error: %v", err)
	}
	if result.Mod.ModLoader != "neoforge" {
		t.Errorf("loader: got %q, want neoforge", result.Mod.ModLoader)
	}
}

func TestScanJarNoManifest(t *testing.T) {
	dir := t.TempDir()

	jarPath := createTestJar(t, dir, "unknown-mod-1.0.jar", map[string][]byte{
		"com/example/Main.class": []byte("fake class"),
	})

	result, err := ScanJar(jarPath)
	if err != nil {
		t.Fatalf("ScanJar() error: %v", err)
	}
	// Should fall back to filename
	if result.Mod.ID != "unknown-mod-1.0" {
		t.Errorf("expected fallback ID from filename, got %q", result.Mod.ID)
	}
}

func TestScanModsFolder(t *testing.T) {
	dir := t.TempDir()

	// Create 2 test jars
	fabricMod := map[string]interface{}{
		"id": "mod-a", "version": "1.0", "name": "Mod A",
	}
	data, _ := json.Marshal(fabricMod)
	createTestJar(t, dir, "mod-a.jar", map[string][]byte{
		"fabric.mod.json": data,
	})

	fabricMod2 := map[string]interface{}{
		"id": "mod-b", "version": "2.0", "name": "Mod B",
	}
	data2, _ := json.Marshal(fabricMod2)
	createTestJar(t, dir, "mod-b.jar", map[string][]byte{
		"fabric.mod.json": data2,
	})

	// Also create a non-jar file (should be ignored)
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hi"), 0644)

	results, err := ScanModsFolder(dir, nil)
	if err != nil {
		t.Fatalf("ScanModsFolder() error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestComputeFingerprint(t *testing.T) {
	// MurmurHash2 with whitespace stripping
	data := []byte("hello world") // space should be stripped
	fp := ComputeFingerprint(data)
	if fp == 0 {
		t.Error("fingerprint should not be zero")
	}

	// Same content with extra whitespace should produce same fingerprint
	data2 := []byte("hello \t\r\n world")
	fp2 := ComputeFingerprint(data2)
	if fp != fp2 {
		t.Errorf("fingerprints should match after whitespace stripping: %d vs %d", fp, fp2)
	}

	// Different content should produce different fingerprint
	data3 := []byte("different content")
	fp3 := ComputeFingerprint(data3)
	if fp == fp3 {
		t.Error("different data should produce different fingerprints")
	}
}

func TestScanJarWithRealModpack(t *testing.T) {
	modsDir := `C:\Users\Darkp\AppData\Roaming\ModrinthApp\profiles\pack testing\mods`
	if _, err := os.Stat(modsDir); os.IsNotExist(err) {
		t.Skip("test modpack not available at", modsDir)
	}

	results, err := ScanModsFolder(modsDir, nil)
	if err != nil {
		t.Fatalf("ScanModsFolder() error: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least some mods from the real modpack")
	}

	t.Logf("Scanned %d mods from real modpack", len(results))

	// Spot-check: every result should have an ID and filename
	for _, r := range results {
		if r.Mod.ID == "" {
			t.Errorf("mod has empty ID: file=%s", r.Mod.JarFileName)
		}
		if r.Mod.JarFileName == "" {
			t.Errorf("mod has empty jar filename: id=%s", r.Mod.ID)
		}
		if r.Mod.JarSHA1 == "" {
			t.Errorf("mod %s has empty SHA1", r.Mod.ID)
		}
	}

	// Log some details for debugging
	loaders := make(map[string]int)
	withDeps := 0
	for _, r := range results {
		loaders[r.Mod.ModLoader]++
		if len(r.Deps) > 0 {
			withDeps++
		}
	}
	t.Logf("Loader distribution: %v", loaders)
	t.Logf("Mods with dependencies: %d", withDeps)
}
