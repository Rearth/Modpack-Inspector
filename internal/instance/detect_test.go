package instance

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectInstances(t *testing.T) {
	instances := DetectInstances(nil)

	t.Logf("Detected %d instances:", len(instances))
	for _, inst := range instances {
		t.Logf("  %s (%s) at %s [hasMods=%v]", inst.Name, inst.Launcher, inst.Path, inst.HasMods)
	}

	// On the dev machine, we know ModrinthApp profiles exist
	modrinthDir := filepath.Join(os.Getenv("APPDATA"), "ModrinthApp", "profiles")
	if _, err := os.Stat(modrinthDir); err == nil {
		found := false
		for _, inst := range instances {
			if inst.Launcher == "modrinth" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to detect modrinth instances but found none")
		}
	}
}

func TestDetectInstancesIncludesPackTesting(t *testing.T) {
	// The user has a modpack at ModrinthApp\profiles\pack testing
	testPath := filepath.Join(os.Getenv("APPDATA"), "ModrinthApp", "profiles", "pack testing")
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		t.Skip("test modpack not available")
	}

	instances := DetectInstances(nil)

	found := false
	for _, inst := range instances {
		if inst.Name == "pack testing" && inst.Launcher == "modrinth" {
			found = true
			if !inst.HasMods {
				t.Error("expected pack testing to have mods")
			}
			break
		}
	}
	if !found {
		t.Error("expected to find 'pack testing' instance from ModrinthApp")
	}
}

func TestParseRoots(t *testing.T) {
	roots := ParseRoots("C:/A\nC:/B; C:/A ")
	if len(roots) != 2 {
		t.Fatalf("expected 2 unique roots, got %d: %v", len(roots), roots)
	}
}

func TestScanInstancesDir(t *testing.T) {
	dir := t.TempDir()

	// Create fake instances
	os.MkdirAll(filepath.Join(dir, "instance-a", "mods"), 0755)
	os.MkdirAll(filepath.Join(dir, "instance-b"), 0755) // no mods folder
	os.MkdirAll(filepath.Join(dir, ".hidden"), 0755)    // hidden, should be skipped

	instances := scanInstancesDir(dir, "test")

	if len(instances) != 2 {
		t.Errorf("expected 2 instances (not hidden), got %d", len(instances))
		for _, i := range instances {
			t.Logf("  %s", i.Name)
		}
	}

	for _, inst := range instances {
		if inst.Name == "instance-a" && !inst.HasMods {
			t.Error("instance-a should have hasMods=true")
		}
		if inst.Name == "instance-b" && inst.HasMods {
			t.Error("instance-b should have hasMods=false")
		}
		if inst.Name == ".hidden" {
			t.Error("hidden directory should be skipped")
		}
	}
}

func TestDirExists(t *testing.T) {
	dir := t.TempDir()

	if !dirExists(dir) {
		t.Error("expected existing dir to return true")
	}
	if dirExists(filepath.Join(dir, "nonexistent")) {
		t.Error("expected nonexistent dir to return false")
	}

	// File, not directory
	f := filepath.Join(dir, "file.txt")
	os.WriteFile(f, []byte("hi"), 0644)
	if dirExists(f) {
		t.Error("expected file to return false from dirExists")
	}
}
