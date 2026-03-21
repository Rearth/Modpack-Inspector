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

func TestEvaluateLibraryDetectionWithThresholdsMarksPureLibraries(t *testing.T) {
	debug := evaluateLibraryDetectionWithThresholds([]string{"library"}, -0.40, 0.2, 0.6, true, 0.18, 0.26, "Questing Mod by FTB")
	if !debug.Detected {
		t.Fatal("expected pure library category to remain a hard positive")
	}
	if debug.ContentTagCount != 0 {
		t.Fatal("expected no content tags")
	}
}

func TestEvaluateLibraryDetectionWithThresholdsAllowsMixedTagLibraries(t *testing.T) {
	categories := []string{"Utility & QoL", "API and Library", "library", "utility"}
	description := "A loader independent library containing shared code reused between mods."

	debug := evaluateLibraryDetectionWithThresholds(categories, 0.24, 0.73, 0.49, true, 0.18, 0.26, description)
	if !debug.Detected {
		t.Fatal("expected mod with all-neutral other tags + library tags to be detected as pure-library-tags")
	}
	if debug.Reason != "pure-library-tags" {
		t.Fatalf("expected reason pure-library-tags, got %s", debug.Reason)
	}
}

func TestEvaluateLibraryDetectionWithThresholdsRejectsMixedTagContentMods(t *testing.T) {
	categories := []string{"Map and Information", "Adventure and RPG", "Server Utility", "API and Library", "Library"}
	description := "Questing Mod by FTB"

	debug := evaluateLibraryDetectionWithThresholds(categories, -0.12, 0.31, 0.43, true, 0.18, 0.26, description)
	if debug.Detected {
		t.Fatal("expected gameplay mod with mixed tags to be rejected")
	}
	if debug.ContentTagCount != 1 {
		t.Fatalf("expected 1 content tag (Adventure and RPG), got %d", debug.ContentTagCount)
	}
}

func TestEvaluateLibraryDetectionWithThresholdsAllowsNoTagSharedCoreMods(t *testing.T) {
	categories := []string{"Utility & QoL"}
	description := "Cupboard provides code, frameworks, and utilities for minecraft mods"

	debug := evaluateLibraryDetectionWithThresholds(categories, 0.32, 0.71, 0.39, true, 0.18, 0.26, description)
	if !debug.Detected {
		t.Fatal("expected strong semantic match to allow no-tag shared-code mod")
	}
}

func TestEvaluateLibraryDetectionWithThresholdsFallsBackToStrongDescription(t *testing.T) {
	categories := []string{"Utility & QoL"}
	description := "Cupboard provides code, frameworks, and utilities for minecraft mods"

	debug := evaluateLibraryDetectionWithThresholds(categories, 0, 0, 0, false, 0.18, 0.26, description)
	if !debug.Detected {
		t.Fatal("expected strong description fallback when semantic classifier is unavailable")
	}
}

func TestEvaluateLibraryDetectionWithThresholdsAllowsExplicitLibraryDescriptions(t *testing.T) {
	for _, tc := range []struct {
		name        string
		categories  []string
		description string
	}{
		{name: "Apothic Attributes", categories: []string{"API and Library", "Adventure and RPG"}, description: "A library mod providing Attributes and related things."},
		{name: "Bookshelf", categories: []string{"Miscellaneous", "Server Utility", "API and Library", "library", "utility"}, description: "An open source library for other mods!"},
		{name: "CBMultipart", categories: []string{"API and Library", "decoration", "library"}, description: "An opensource library for having multiple things in the one block space"},
	} {
		debug := evaluateLibraryDetectionWithThresholds(tc.categories, -0.05, 0.41, 0.46, true, 0.18, 0.26, tc.description)
		if !debug.Detected {
			t.Fatalf("expected %s to be detected via explicit library wording", tc.name)
		}
	}
}

func TestTagWeightNeutralTagsTreatedAsPureLibrary(t *testing.T) {
	// Mods with library tags and only neutral other tags should be pure library
	for _, tc := range []struct {
		name       string
		categories []string
	}{
		{name: "CommonCapabilities", categories: []string{"API and Library", "Addons"}},
		{name: "Cyclops Core", categories: []string{"Server Utility", "API and Library"}},
		{name: "Jupiter", categories: []string{"Server Utility", "API and Library", "Utility & QoL", "library"}},
		{name: "PrickleMC", categories: []string{"Server Utility", "API and Library", "Utility & QoL", "library"}},
		{name: "YetAnotherConfigLib", categories: []string{"API and Library", "library", "management", "utility"}},
		{name: "Rhino", categories: []string{"KubeJS", "API and Library"}},
	} {
		debug := evaluateLibraryDetectionWithThresholds(tc.categories, 0.0, 0.0, 0.0, false, 0.18, 0.26, "some description")
		if !debug.Detected || debug.Reason != "pure-library-tags" {
			t.Fatalf("expected %s to be detected as pure-library-tags, got detected=%v reason=%s", tc.name, debug.Detected, debug.Reason)
		}
	}
}

func TestTaggedLibraryDescriptionDetection(t *testing.T) {
	// Mods with library tags + content tags that mention "library"/"api"/"lib" in description
	for _, tc := range []struct {
		name       string
		categories []string
		texts      []string
	}{
		{name: "ConnectedTexturesMod", categories: []string{"Cosmetic", "API and Library"}, texts: []string{"ctm", "A resource pack extension library"}},
		{name: "Caelus API", categories: []string{"API and Library", "game-mechanics", "library", "transportation"}, texts: []string{"caelus", "A coremod and API to provide developers access to elytra flight mechanics"}},
		{name: "TerraBlender", categories: []string{"World Gen", "Biomes", "API and Library"}, texts: []string{"terrablender", "A library mod for adding biomes in a simple and compatible manner!"}},
		{name: "SmartBrainLib", categories: []string{"API and Library", "library", "mobs", "utility"}, texts: []string{"SmartBrainLib", "smartbrainlib", "A smarter brain system for Minecraft"}},
		{name: "Curios API", categories: []string{"API and Library", "Adventure and RPG", "Armor", "Tools", "equipment", "library", "utility"}, texts: []string{"curios", "A flexible and expandable accessory/equipment API for users and developers."}},
		{name: "Fzzy Config", categories: []string{"API and Library", "Utility & QoL", "game-mechanics", "library"}, texts: []string{"fzzy_config", "Config API with automatic GUIs, powerful validation options"}},
		{name: "lionfishapi", categories: []string{"API and Library", "library", "utility"}, texts: []string{"lionfishapi", "Very Light Animation API"}},
	} {
		debug := evaluateLibraryDetectionWithThresholds(tc.categories, 0.0, 0.3, 0.3, true, 0.18, 0.26, tc.texts...)
		if !debug.Detected {
			t.Fatalf("expected %s to be detected via tagged-library-description, got reason=%s", tc.name, debug.Reason)
		}
	}
}

func TestDescriptionMentionsLibrary(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"A library mod for adding biomes", true},
		{"player animation library", true},
		{"A Minecraft lib used for all mods", true},
		{"Very Light Animation API", true},
		{"SmartBrainLib", true},
		{"lionfishapi", true},
		{"A great mod for building", false},
		{"Adds new items and blocks", false},
	}
	for _, tc := range tests {
		got := descriptionMentionsLibrary(tc.text)
		if got != tc.want {
			t.Errorf("descriptionMentionsLibrary(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}
