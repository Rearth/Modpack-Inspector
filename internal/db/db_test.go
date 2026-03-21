package db

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) *Database {
	t.Helper()
	dir := t.TempDir()
	db, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestNewDatabase(t *testing.T) {
	dir := t.TempDir()
	db, err := New(dir)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	defer db.Close()

	// Verify the database file exists
	dbPath := filepath.Join(dir, "modpacktool.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}

func TestModCRUD(t *testing.T) {
	db := setupTestDB(t)

	// Insert
	mod := &Mod{
		ID:          "test-mod",
		Name:        "Test Mod",
		Version:     "1.0.0",
		Description: "A test mod",
		Authors:     "tester",
		ModLoader:   "fabric",
		JarFileName: "test-mod-1.0.0.jar",
		JarSHA1:     "abc123",
		IsLibrary:   false,
		LastScanned: time.Now(),
	}
	if err := db.UpsertMod(mod); err != nil {
		t.Fatalf("UpsertMod() error: %v", err)
	}

	// Read all
	mods, err := db.GetAllMods()
	if err != nil {
		t.Fatalf("GetAllMods() error: %v", err)
	}
	if len(mods) != 1 {
		t.Fatalf("expected 1 mod, got %d", len(mods))
	}
	if mods[0].ID != "test-mod" || mods[0].Name != "Test Mod" {
		t.Errorf("unexpected mod data: %+v", mods[0])
	}

	// Read by ID
	got, err := db.GetModByID("test-mod")
	if err != nil {
		t.Fatalf("GetModByID() error: %v", err)
	}
	if got == nil || got.Version != "1.0.0" {
		t.Errorf("GetModByID returned wrong data: %+v", got)
	}

	// Get non-existent
	got, err = db.GetModByID("nonexistent")
	if err != nil {
		t.Fatalf("GetModByID(nonexistent) error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for non-existent mod")
	}

	// Update via upsert
	mod.Version = "2.0.0"
	if err := db.UpsertMod(mod); err != nil {
		t.Fatalf("UpsertMod(update) error: %v", err)
	}
	got, _ = db.GetModByID("test-mod")
	if got.Version != "2.0.0" {
		t.Errorf("expected version 2.0.0, got %s", got.Version)
	}

	// Still only 1 mod
	mods, _ = db.GetAllMods()
	if len(mods) != 1 {
		t.Errorf("expected 1 mod after upsert, got %d", len(mods))
	}

	// Delete
	if err := db.DeleteMod("test-mod"); err != nil {
		t.Fatalf("DeleteMod() error: %v", err)
	}
	mods, _ = db.GetAllMods()
	if len(mods) != 0 {
		t.Errorf("expected 0 mods after delete, got %d", len(mods))
	}
}

func TestModDeleteByFileName(t *testing.T) {
	db := setupTestDB(t)

	db.UpsertMod(&Mod{ID: "m1", Name: "M1", JarFileName: "m1.jar"})
	db.UpsertMod(&Mod{ID: "m2", Name: "M2", JarFileName: "m2.jar"})

	if err := db.DeleteModByFileName("m1.jar"); err != nil {
		t.Fatalf("DeleteModByFileName() error: %v", err)
	}
	mods, _ := db.GetAllMods()
	if len(mods) != 1 || mods[0].ID != "m2" {
		t.Errorf("wrong mods after delete: %+v", mods)
	}
}

func TestDependencyCRUD(t *testing.T) {
	db := setupTestDB(t)

	dep := &Dependency{
		ModID:    "parent",
		DepModID: "child",
		DepName:  "child-mod",
		Type:     "required",
		Source:   "manifest",
	}
	if err := db.UpsertDependency(dep); err != nil {
		t.Fatalf("UpsertDependency() error: %v", err)
	}

	deps, err := db.GetDependenciesByModID("parent")
	if err != nil {
		t.Fatalf("GetDependenciesByModID() error: %v", err)
	}
	if len(deps) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(deps))
	}
	if deps[0].DepModID != "child" || deps[0].Type != "required" {
		t.Errorf("unexpected dep: %+v", deps[0])
	}

	// Get all
	allDeps, _ := db.GetAllDependencies()
	if len(allDeps) != 1 {
		t.Errorf("expected 1 total dep, got %d", len(allDeps))
	}

	// Delete by mod ID
	db.DeleteDependenciesByModID("parent")
	deps, _ = db.GetDependenciesByModID("parent")
	if len(deps) != 0 {
		t.Errorf("expected 0 deps after delete, got %d", len(deps))
	}
}

func TestConfigMappingCRUD(t *testing.T) {
	db := setupTestDB(t)

	cm := &ConfigMapping{
		ConfigPath: "mymod.toml",
		ModID:      "mymod",
		Confidence: 95,
		IsManual:   false,
	}
	if err := db.UpsertConfigMapping(cm); err != nil {
		t.Fatalf("UpsertConfigMapping() error: %v", err)
	}

	mappings, err := db.GetConfigMappingsByModID("mymod")
	if err != nil {
		t.Fatalf("GetConfigMappingsByModID() error: %v", err)
	}
	if len(mappings) != 1 || mappings[0].Confidence != 95 {
		t.Errorf("unexpected mapping: %+v", mappings)
	}

	// Manual override
	manual := &ConfigMapping{
		ConfigPath: "other.toml",
		ModID:      "mymod",
		Confidence: 100,
		IsManual:   true,
	}
	db.UpsertConfigMapping(manual)

	all, _ := db.GetAllConfigMappings()
	if len(all) != 2 {
		t.Errorf("expected 2 mappings, got %d", len(all))
	}

	// Delete non-manual
	db.DeleteNonManualMappings()
	all, _ = db.GetAllConfigMappings()
	if len(all) != 1 || !all[0].IsManual {
		t.Errorf("expected only manual mapping remaining, got: %+v", all)
	}

	// Delete specific
	db.DeleteConfigMapping("other.toml", "mymod")
	all, _ = db.GetAllConfigMappings()
	if len(all) != 0 {
		t.Errorf("expected 0 mappings after delete, got %d", len(all))
	}
}

func TestAPICache(t *testing.T) {
	db := setupTestDB(t)

	// Set cache
	if err := db.SetCachedAPI("key1", `{"data":"hello"}`, time.Hour); err != nil {
		t.Fatalf("SetCachedAPI() error: %v", err)
	}

	// Get valid cache
	val, ok, err := db.GetCachedAPI("key1")
	if err != nil || !ok {
		t.Fatalf("GetCachedAPI() error: %v, ok: %v", err, ok)
	}
	if val != `{"data":"hello"}` {
		t.Errorf("unexpected cached value: %s", val)
	}

	// Get non-existent
	_, ok, err = db.GetCachedAPI("nonexistent")
	if err != nil || ok {
		t.Errorf("expected miss for nonexistent key, got ok=%v err=%v", ok, err)
	}

	// Expired cache
	db.SetCachedAPI("expired", "old", -time.Hour) // negative TTL = already expired
	_, ok, _ = db.GetCachedAPI("expired")
	if ok {
		t.Error("expected cache miss for expired entry")
	}
}

func TestSettings(t *testing.T) {
	db := setupTestDB(t)

	// Default settings
	s, err := db.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings() error: %v", err)
	}
	if s.InstancePath != "" {
		t.Errorf("expected empty instance path, got %q", s.InstancePath)
	}

	// Save and load
	s.InstancePath = "/some/path"
	s.CurseForgeAPIKey = "cf-key"
	s.ModrinthAPIKey = "mr-key"
	s.CustomModrinthRoot = "/modrinth"
	s.CustomCurseForgeRoot = "/curseforge"
	s.CustomFTBRoot = "/ftb"
	s.CustomLauncherRoots = "/custom-a\n/custom-b"
	if err := db.SaveSettings(s); err != nil {
		t.Fatalf("SaveSettings() error: %v", err)
	}

	got, _ := db.GetSettings()
	if got.InstancePath != "/some/path" {
		t.Errorf("instance path: got %q, want /some/path", got.InstancePath)
	}
	if got.CurseForgeAPIKey != "cf-key" {
		t.Errorf("cf key: got %q, want cf-key", got.CurseForgeAPIKey)
	}
	if got.CustomModrinthRoot != "/modrinth" || got.CustomCurseForgeRoot != "/curseforge" || got.CustomFTBRoot != "/ftb" || got.CustomLauncherRoots != "/custom-a\n/custom-b" {
		t.Errorf("custom launcher roots not preserved: %+v", got)
	}
}

func TestClearScanDataPreservesSettingsAndCache(t *testing.T) {
	db := setupTestDB(t)

	if err := db.UpsertMod(&Mod{ID: "mod-a", Name: "Mod A", JarFileName: "a.jar", LastScanned: time.Now()}); err != nil {
		t.Fatalf("UpsertMod() error: %v", err)
	}
	if err := db.UpsertDependency(&Dependency{ModID: "mod-a", DepModID: "mod-b", Type: "required", Source: "manifest"}); err != nil {
		t.Fatalf("UpsertDependency() error: %v", err)
	}
	if err := db.UpsertConfigMapping(&ConfigMapping{ConfigPath: "mod-a.toml", ModID: "mod-a", Confidence: 100, IsManual: true}); err != nil {
		t.Fatalf("UpsertConfigMapping() error: %v", err)
	}
	if err := db.SetCachedAPI("cache-key", `{"ok":true}`, time.Hour); err != nil {
		t.Fatalf("SetCachedAPI() error: %v", err)
	}
	if err := db.SaveSettings(&Settings{InstancePath: "/pack-a", CurseForgeAPIKey: "cf", ModrinthAPIKey: "mr", CacheTTLHours: 24}); err != nil {
		t.Fatalf("SaveSettings() error: %v", err)
	}

	if err := db.ClearScanData(); err != nil {
		t.Fatalf("ClearScanData() error: %v", err)
	}

	mods, _ := db.GetAllMods()
	if len(mods) != 0 {
		t.Fatalf("expected mods to be cleared, got %d", len(mods))
	}
	deps, _ := db.GetAllDependencies()
	if len(deps) != 0 {
		t.Fatalf("expected dependencies to be cleared, got %d", len(deps))
	}
	mappings, _ := db.GetAllConfigMappings()
	if len(mappings) != 0 {
		t.Fatalf("expected config mappings to be cleared, got %d", len(mappings))
	}
	settings, _ := db.GetSettings()
	if settings.InstancePath != "/pack-a" || settings.CurseForgeAPIKey != "cf" || settings.ModrinthAPIKey != "mr" {
		t.Fatalf("settings should be preserved, got %+v", settings)
	}
	if _, ok, err := db.GetCachedAPI("cache-key"); err != nil || !ok {
		t.Fatalf("api cache should be preserved, ok=%v err=%v", ok, err)
	}
}

func TestGetModFileNames(t *testing.T) {
	db := setupTestDB(t)

	db.UpsertMod(&Mod{ID: "mod-a", JarFileName: "a.jar"})
	db.UpsertMod(&Mod{ID: "mod-b", JarFileName: "b.jar"})

	names, err := db.GetModFileNames()
	if err != nil {
		t.Fatalf("GetModFileNames() error: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(names))
	}
	if names["a.jar"] != "mod-a" || names["b.jar"] != "mod-b" {
		t.Errorf("unexpected file names map: %v", names)
	}
}

func TestMixinCRUD(t *testing.T) {
	db := setupTestDB(t)

	m := &Mixin{
		OwnerModID:    "mod-a",
		MixinClass:    "com.example.mixin.PlayerMixin",
		TargetClass:   "net.minecraft.world.entity.player.Player",
		TargetModID:   "minecraft",
		TargetMembers: "tick,hurt",
	}
	if err := db.UpsertMixin(m); err != nil {
		t.Fatalf("UpsertMixin() error: %v", err)
	}

	// Query by owner
	mixins, err := db.GetMixinsByModID("mod-a")
	if err != nil {
		t.Fatalf("GetMixinsByModID() error: %v", err)
	}
	if len(mixins) != 1 {
		t.Fatalf("expected 1 mixin, got %d", len(mixins))
	}
	if mixins[0].TargetClass != "net.minecraft.world.entity.player.Player" {
		t.Errorf("unexpected target: %s", mixins[0].TargetClass)
	}
	if mixins[0].TargetMembers != "tick,hurt" {
		t.Errorf("unexpected members: %s", mixins[0].TargetMembers)
	}

	// Query by target
	incoming, err := db.GetMixinsTargetingMod("minecraft")
	if err != nil {
		t.Fatalf("GetMixinsTargetingMod() error: %v", err)
	}
	if len(incoming) != 1 {
		t.Fatalf("expected 1 incoming, got %d", len(incoming))
	}
	if incoming[0].OwnerModID != "mod-a" {
		t.Errorf("expected owner mod-a, got %s", incoming[0].OwnerModID)
	}

	// Upsert again with different members (should update)
	m.TargetMembers = "tick,hurt,attack"
	if err := db.UpsertMixin(m); err != nil {
		t.Fatalf("UpsertMixin() update error: %v", err)
	}
	mixins, _ = db.GetMixinsByModID("mod-a")
	if mixins[0].TargetMembers != "tick,hurt,attack" {
		t.Errorf("upsert didn't update members: %s", mixins[0].TargetMembers)
	}

	// Delete by owner
	if err := db.DeleteMixinsByModID("mod-a"); err != nil {
		t.Fatalf("DeleteMixinsByModID() error: %v", err)
	}
	mixins, _ = db.GetMixinsByModID("mod-a")
	if len(mixins) != 0 {
		t.Errorf("expected 0 mixins after delete, got %d", len(mixins))
	}
}

func TestClearScanDataClearsMixins(t *testing.T) {
	db := setupTestDB(t)

	db.UpsertMixin(&Mixin{
		OwnerModID:  "mod-a",
		MixinClass:  "com.example.mixin.TestMixin",
		TargetClass: "net.minecraft.Foo",
		TargetModID: "minecraft",
	})

	if err := db.ClearScanData(); err != nil {
		t.Fatalf("ClearScanData() error: %v", err)
	}

	mixins, _ := db.GetMixinsByModID("mod-a")
	if len(mixins) != 0 {
		t.Errorf("expected mixins to be cleared, got %d", len(mixins))
	}
}
