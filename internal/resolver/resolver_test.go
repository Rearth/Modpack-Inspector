package resolver

import (
	"testing"

	"modpacktool/internal/db"
)

func TestBuildGraph(t *testing.T) {
	mods := []db.Mod{
		{ID: "mod-a", Name: "Mod A", ModLoader: "fabric"},
		{ID: "mod-b", Name: "Mod B", ModLoader: "fabric", IsLibrary: true},
	}
	deps := []db.Dependency{
		{ModID: "mod-a", DepModID: "mod-b", Type: "required"},
	}

	graph := BuildGraph(mods, deps, nil)

	if len(graph.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(graph.Nodes))
	}
	if len(graph.Links) != 1 {
		t.Errorf("expected 1 link, got %d", len(graph.Links))
	}

	// Check node groups
	for _, n := range graph.Nodes {
		if n.ID == "mod-b" && n.Group != "library" {
			t.Errorf("expected mod-b group=library, got %q", n.Group)
		}
		if n.ID == "mod-a" && n.Group != "normal" {
			t.Errorf("expected mod-a group=normal, got %q", n.Group)
		}
	}
}

func TestFindMissingDependencies(t *testing.T) {
	mods := []db.Mod{
		{ID: "mod-a", Name: "Mod A"},
	}
	deps := []db.Dependency{
		{ModID: "mod-a", DepModID: "mod-b", Type: "required"},
		{ModID: "mod-a", DepModID: "mod-c", Type: "optional"}, // optional should NOT be missing
	}

	missing := FindMissingDependencies(mods, deps)
	if len(missing) != 1 {
		t.Fatalf("expected 1 missing dep, got %d", len(missing))
	}
	if missing[0].DepModID != "mod-b" {
		t.Errorf("expected missing dep mod-b, got %s", missing[0].DepModID)
	}
	if missing[0].RequiredBy != "mod-a" {
		t.Errorf("expected requiredBy mod-a, got %s", missing[0].RequiredBy)
	}
}

func TestFindMissingDependenciesDedup(t *testing.T) {
	mods := []db.Mod{{ID: "a"}}
	deps := []db.Dependency{
		{ModID: "a", DepModID: "b", Type: "required", Source: "manifest"},
		{ModID: "a", DepModID: "b", Type: "required", Source: "modrinth"}, // duplicate
	}

	missing := FindMissingDependencies(mods, deps)
	if len(missing) != 1 {
		t.Errorf("expected deduplication: got %d missing", len(missing))
	}
}

func TestFindMissingDependenciesAllSatisfied(t *testing.T) {
	mods := []db.Mod{
		{ID: "mod-a"},
		{ID: "mod-b"},
	}
	deps := []db.Dependency{
		{ModID: "mod-a", DepModID: "mod-b", Type: "required"},
	}

	missing := FindMissingDependencies(mods, deps)
	if len(missing) != 0 {
		t.Errorf("expected no missing deps, got %d", len(missing))
	}
}

func TestFindUnusedLibraries(t *testing.T) {
	mods := []db.Mod{
		{ID: "main-mod", Name: "Main Mod", IsLibrary: false},
		{ID: "used-lib", Name: "Used Lib", IsLibrary: true},
		{ID: "unused-lib", Name: "Unused Lib", IsLibrary: true},
	}
	deps := []db.Dependency{
		{ModID: "main-mod", DepModID: "used-lib", Type: "required"},
	}

	unused := FindUnusedLibraries(mods, deps)
	if len(unused) != 1 {
		t.Fatalf("expected 1 unused lib, got %d: %v", len(unused), unused)
	}
	if unused[0] != "unused-lib" {
		t.Errorf("expected unused-lib, got %s", unused[0])
	}
}

func TestFindUnusedLibrariesTransitive(t *testing.T) {
	// main -> lib-a -> lib-b
	// lib-b should NOT be unused (transitively needed)
	mods := []db.Mod{
		{ID: "main", IsLibrary: false},
		{ID: "lib-a", IsLibrary: true},
		{ID: "lib-b", IsLibrary: true},
		{ID: "orphan-lib", IsLibrary: true},
	}
	deps := []db.Dependency{
		{ModID: "main", DepModID: "lib-a", Type: "required"},
		{ModID: "lib-a", DepModID: "lib-b", Type: "required"},
	}

	unused := FindUnusedLibraries(mods, deps)
	if len(unused) != 1 || unused[0] != "orphan-lib" {
		t.Errorf("expected only orphan-lib as unused, got %v", unused)
	}
}

func TestFindUnusedLibrariesUsesProvidedAliases(t *testing.T) {
	mods := []db.Mod{
		{ID: "main", IsLibrary: false},
		{ID: "fabric-api", IsLibrary: true, ModrinthID: "P7dR8mSH", ProvidedIDs: "fabric-api-base,fabric-resource-loader-v0"},
	}
	deps := []db.Dependency{
		{ModID: "main", DepModID: "fabric-resource-loader-v0", Type: "required"},
	}

	unused := FindUnusedLibraries(mods, deps)
	if len(unused) != 0 {
		t.Fatalf("expected fabric-api to be treated as used via provided alias, got %v", unused)
	}
	updated := UpdateSatisfied(deps, mods)
	if !updated[0].Satisfied {
		t.Fatal("expected dependency on provided alias to be satisfied")
	}
	graph := BuildGraph(mods, deps, map[string]bool{})
	if len(graph.Links) != 1 || graph.Links[0].Target != "fabric-api" {
		t.Fatalf("expected graph link to resolve to fabric-api, got %+v", graph.Links)
	}
}

func TestBuildGraphSkipsAmbiguousAliasProviderCollision(t *testing.T) {
	mods := []db.Mod{
		{ID: "fabric-api", IsLibrary: true, ProvidedIDs: "fabric-api-base"},
		{ID: "modmenu", IsLibrary: false, ProvidedIDs: "fabric-api-base"},
		{ID: "immersive_aircraft", IsLibrary: false},
	}
	deps := []db.Dependency{{ModID: "immersive_aircraft", DepModID: "fabric-api-base", Type: "required"}}

	updated := UpdateSatisfied(deps, mods)
	if !updated[0].Satisfied {
		t.Fatal("expected ambiguous embedded alias to still count as satisfied")
	}
	graph := BuildGraph(mods, deps, map[string]bool{})
	if len(graph.Links) != 0 {
		t.Fatalf("expected no fabricated link for ambiguous provider collision, got %+v", graph.Links)
	}
}

func TestFindUnusedLibrariesNoLibraries(t *testing.T) {
	mods := []db.Mod{
		{ID: "a", IsLibrary: false},
		{ID: "b", IsLibrary: false},
	}
	unused := FindUnusedLibraries(mods, nil)
	if len(unused) != 0 {
		t.Errorf("expected 0 unused when no libraries exist, got %d", len(unused))
	}
}

func TestUpdateSatisfied(t *testing.T) {
	deps := []db.Dependency{
		{ModID: "a", DepModID: "b", Satisfied: false},
		{ModID: "a", DepModID: "c", Satisfied: false},
	}
	mods := []db.Mod{{ID: "b"}}

	updated := UpdateSatisfied(deps, mods)
	if !updated[0].Satisfied {
		t.Error("dep on b should be satisfied")
	}
	if updated[1].Satisfied {
		t.Error("dep on c should not be satisfied")
	}

	// Original should be unchanged
	if deps[0].Satisfied {
		t.Error("original deps should not be modified")
	}
}

func TestBuildGraphEmpty(t *testing.T) {
	graph := BuildGraph(nil, nil, nil)
	if graph.Nodes != nil {
		t.Errorf("expected nil nodes for empty input, got %d", len(graph.Nodes))
	}
	if graph.Links != nil {
		t.Errorf("expected nil links for empty input, got %d", len(graph.Links))
	}
}

func TestBuildGraphMissingNodes(t *testing.T) {
	mods := []db.Mod{
		{ID: "mod-a", Name: "Mod A"},
	}
	deps := []db.Dependency{
		{ModID: "mod-a", DepModID: "mod-missing", DepName: "Missing Lib", Type: "required"},
	}
	graph := BuildGraph(mods, deps, nil)

	if len(graph.Nodes) != 2 {
		t.Fatalf("expected 2 nodes (1 installed + 1 missing), got %d", len(graph.Nodes))
	}
	found := false
	for _, n := range graph.Nodes {
		if n.ID == "mod-missing" {
			found = true
			if n.Group != "missing" {
				t.Errorf("expected missing node group=missing, got %q", n.Group)
			}
			if n.Name != "Missing Lib" {
				t.Errorf("expected missing node name='Missing Lib', got %q", n.Name)
			}
		}
	}
	if !found {
		t.Error("missing node not added to graph")
	}
	if len(graph.Links) != 1 {
		t.Errorf("expected 1 link, got %d", len(graph.Links))
	}
}

func TestBuildGraphUnusedLibrary(t *testing.T) {
	mods := []db.Mod{
		{ID: "mod-a", Name: "Mod A"},
		{ID: "lib-unused", Name: "Unused Lib", IsLibrary: true},
	}
	unused := map[string]bool{"lib-unused": true}
	graph := BuildGraph(mods, nil, unused)

	for _, n := range graph.Nodes {
		if n.ID == "lib-unused" && n.Group != "unused" {
			t.Errorf("expected lib-unused group=unused, got %q", n.Group)
		}
	}
}

func TestBuildGraphSkipsUnresolvedOnlineOnlyDependencies(t *testing.T) {
	mods := []db.Mod{{ID: "mod-a", Name: "Mod A"}}
	deps := []db.Dependency{
		{ModID: "mod-a", DepModID: "P7dR8mSH", DepName: "P7dR8mSH", Type: "required", Source: "modrinth"},
		{ModID: "mod-a", DepModID: "123456", DepName: "123456", Type: "optional", Source: "curseforge"},
	}

	graph := BuildGraph(mods, deps, nil)

	if len(graph.Nodes) != 1 {
		t.Fatalf("expected only the installed mod node, got %d nodes", len(graph.Nodes))
	}
	if len(graph.Links) != 0 {
		t.Fatalf("expected unresolved online-only deps to be hidden, got %+v", graph.Links)
	}
}

func TestBuildGraphKeepsUnresolvedManifestDependencies(t *testing.T) {
	mods := []db.Mod{{ID: "mod-a", Name: "Mod A"}}
	deps := []db.Dependency{{ModID: "mod-a", DepModID: "missing-lib", DepName: "missing-lib", Type: "required", Source: "manifest"}}

	graph := BuildGraph(mods, deps, nil)

	if len(graph.Nodes) != 2 {
		t.Fatalf("expected manifest missing dependency to stay visible, got %d nodes", len(graph.Nodes))
	}
	if len(graph.Links) != 1 {
		t.Fatalf("expected manifest missing dependency link to remain, got %d", len(graph.Links))
	}
}
