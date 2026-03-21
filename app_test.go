package main

import (
	"strings"
	"testing"

	"modpacktool/internal/api"
	"modpacktool/internal/db"
	"modpacktool/internal/resolver"
)

func TestEnrichModMergesManifestAndOnlineDependencies(t *testing.T) {
	manifest := []db.Dependency{{ModID: "host", DepModID: "fabric-api", Type: "required", Source: "manifest"}}
	merged := mergeDependencies(manifest,
		onlineDepsForMod("host", []api.OnlineDep{{ModID: "P7dR8mSH", Name: "fabric-api", Type: "required", Source: "modrinth"}})...,
	)

	if len(merged) != 2 {
		t.Fatalf("expected manifest and online dependencies to coexist, got %d", len(merged))
	}
	if merged[0].Source != "manifest" {
		t.Fatalf("expected manifest dep first, got %q", merged[0].Source)
	}
	if merged[1].Source != "modrinth" {
		t.Fatalf("expected online dep to be preserved by source, got %q", merged[1].Source)
	}
}

func TestMergeDependenciesDeduplicatesWithinSource(t *testing.T) {
	deps := mergeDependencies(nil,
		db.Dependency{ModID: "host", DepModID: "dep", Type: "required", Source: "modrinth"},
		db.Dependency{ModID: "host", DepModID: "dep", Type: "required", Source: "modrinth"},
	)

	if len(deps) != 1 {
		t.Fatalf("expected duplicate online dependencies to be collapsed, got %d", len(deps))
	}
}

func TestShouldDisplayDependencyHidesUnresolvedOnlineOnlyIDs(t *testing.T) {
	if resolver.ShouldDisplayDependency(db.Dependency{Source: "modrinth"}, "P7dR8mSH", false) {
		t.Fatal("expected unresolved modrinth-only dependency to be hidden")
	}
	if resolver.ShouldDisplayDependency(db.Dependency{Source: "curseforge"}, "123456", false) {
		t.Fatal("expected unresolved curseforge-only dependency to be hidden")
	}
	if !resolver.ShouldDisplayDependency(db.Dependency{Source: "manifest"}, "missing-lib", false) {
		t.Fatal("expected unresolved manifest dependency to remain visible")
	}
	if !resolver.ShouldDisplayDependency(db.Dependency{Source: "modrinth"}, "installed-mod", true) {
		t.Fatal("expected resolved dependency to remain visible regardless of source")
	}
}

func TestBuildDetailDependenciesSeparatesUnresolvedOnlineOnlyDeps(t *testing.T) {
	mods := []db.Mod{{ID: "host", Name: "Host Mod"}, {ID: "fabric-api", Name: "Fabric API"}}
	deps := []db.Dependency{
		{ModID: "host", DepModID: "fabric-api", DepName: "Fabric API", Type: "required", Source: "manifest", Satisfied: true},
		{ModID: "host", DepModID: "P7dR8mSH", DepName: "Fabric API", Type: "required", Source: "modrinth"},
		{ModID: "host", DepModID: "moonlight-lib", DepName: "Moonlight Lib", Type: "optional", Source: "modrinth"},
		{ModID: "host", DepModID: "moonlight-lib", DepName: "Moonlight Lib", Type: "required", Source: "modrinth"},
		{ModID: "host", DepModID: "123456", DepName: "Supplementaries", Type: "optional", Source: "curseforge"},
	}

	visible, unresolved := buildDetailDependencies("host", deps, mods)
	if len(visible) != 1 {
		t.Fatalf("expected 1 visible dependency, got %d", len(visible))
	}
	if visible[0].DepName != "Fabric API" {
		t.Fatalf("expected resolved dependency to remain visible, got %q", visible[0].DepName)
	}
	if len(unresolved) != 2 {
		t.Fatalf("expected 2 unresolved external dependencies, got %d", len(unresolved))
	}
	if unresolved[0].Source != "modrinth" || unresolved[0].Type != "required" {
		t.Fatalf("expected merged modrinth unresolved dep to be required, got source=%q type=%q", unresolved[0].Source, unresolved[0].Type)
	}
	if unresolved[0].OpenURL == "" || !strings.Contains(unresolved[0].OpenURL, "modrinth.com") {
		t.Fatalf("expected modrinth unresolved dep to include a Modrinth URL, got %q", unresolved[0].OpenURL)
	}
	if unresolved[1].Source != "curseforge" || unresolved[1].OpenURL == "" || !strings.Contains(unresolved[1].OpenURL, "curseforge.com") {
		t.Fatalf("expected curseforge unresolved dep to include a CurseForge URL, got %#v", unresolved[1])
	}
	if unresolved[1].DepName != "Supplementaries" {
		t.Fatalf("expected unresolved dep name to be preserved, got %q", unresolved[1].DepName)
	}
}
