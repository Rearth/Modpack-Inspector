package resolver

import (
	"strconv"
	"strings"

	"modpacktool/internal/db"
)

// Graph represents the mod dependency graph.
type Graph struct {
	Nodes []GraphNode `json:"nodes"`
	Links []GraphLink `json:"links"`
}

// GraphNode is a mod in the dependency graph.
type GraphNode struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ModLoader string `json:"modLoader"`
	Loaders   string `json:"loaders"`
	IsLibrary bool   `json:"isLibrary"`
	Group     string `json:"group"` // for coloring: "normal", "library", "missing", "unused"
	IconURL   string `json:"iconURL"`
}

// GraphLink is a dependency edge in the graph.
type GraphLink struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"` // required, optional, embedded
}

// MissingDep represents a required dependency that isn't in the mods folder.
type MissingDep struct {
	RequiredBy string `json:"requiredBy"`
	DepModID   string `json:"depModID"`
	DepName    string `json:"depName"`
}

// buildIDMap creates a mapping from direct IDs (manifest ID, Modrinth project ID,
// CurseForge numeric ID) to the canonical mod ID from the JAR manifest.
func buildIDMap(mods []db.Mod) map[string]string {
	m := make(map[string]string)
	for _, mod := range mods {
		m[mod.ID] = mod.ID
		if mod.ModrinthID != "" {
			addAlias(m, mod.ModrinthID, mod.ID)
		}
		if mod.CurseForgeID != 0 {
			addAlias(m, strconv.Itoa(mod.CurseForgeID), mod.ID)
		}
	}
	return m
}

func buildProviderMap(mods []db.Mod) map[string][]string {
	providers := make(map[string][]string)
	for _, mod := range mods {
		for _, alias := range splitProvidedIDs(mod.ProvidedIDs) {
			list := providers[alias]
			duplicate := false
			for _, existing := range list {
				if existing == mod.ID {
					duplicate = true
					break
				}
			}
			if !duplicate {
				providers[alias] = append(list, mod.ID)
			}
		}
	}
	return providers
}

func addAlias(idMap map[string]string, alias string, canonical string) {
	if alias == "" {
		return
	}
	if _, exists := idMap[alias]; exists {
		return
	}
	idMap[alias] = canonical
}

func splitProvidedIDs(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// resolveDepTarget maps a dependency target to a concrete installed mod when possible.
// Embedded modules provided by the source mod resolve to the source mod itself so callers can
// treat them as satisfied without creating misleading cross-mod edges.
func resolveDepTarget(sourceModID, depModID string, idMap map[string]string, providerMap map[string][]string) (string, bool) {
	if canonical, ok := idMap[depModID]; ok {
		return canonical, true
	}

	providers := providerMap[depModID]
	if len(providers) == 0 {
		return depModID, false
	}

	for _, provider := range providers {
		if provider == sourceModID {
			return sourceModID, true
		}
	}

	if len(providers) == 1 {
		return providers[0], true
	}

	// Multiple providers exist and none is the source mod. It's satisfied, but ambiguous.
	return "", true
}

func ResolveDependencyTarget(mods []db.Mod, sourceModID, depModID string) (string, bool) {
	return resolveDepTarget(sourceModID, depModID, buildIDMap(mods), buildProviderMap(mods))
}

// ShouldDisplayDependency returns whether a dependency should be surfaced in the UI.
// Online-only unresolved dependencies often only contain provider IDs, which are too noisy to show.
func ShouldDisplayDependency(dep db.Dependency, resolvedID string, satisfied bool) bool {
	if satisfied && resolvedID != "" {
		return true
	}
	return dep.Source == "" || dep.Source == "manifest"
}

// BuildGraph constructs the dependency graph and returns analysis results.
func BuildGraph(mods []db.Mod, deps []db.Dependency, unusedIDs map[string]bool) *Graph {
	modSet := make(map[string]*db.Mod)
	for i := range mods {
		modSet[mods[i].ID] = &mods[i]
	}
	idMap := buildIDMap(mods)
	providerMap := buildProviderMap(mods)

	g := &Graph{}

	// Add nodes for all installed mods
	for _, m := range mods {
		group := "normal"
		if m.IsLibrary && unusedIDs[m.ID] {
			group = "unused"
		} else if m.IsLibrary {
			group = "library"
		}
		g.Nodes = append(g.Nodes, GraphNode{
			ID:        m.ID,
			Name:      m.Name,
			ModLoader: m.ModLoader,
			Loaders:   m.Loaders,
			IsLibrary: m.IsLibrary,
			Group:     group,
			IconURL:   m.IconURL,
		})
	}

	// Add links; resolve dep target IDs, deduplicate, skip self-references
	seen := make(map[string]bool)
	for _, dep := range deps {
		if _, ok := modSet[dep.ModID]; !ok {
			continue
		}
		targetID, satisfied := resolveDepTarget(dep.ModID, dep.DepModID, idMap, providerMap)
		if !ShouldDisplayDependency(dep, targetID, satisfied) {
			continue
		}
		if satisfied && (targetID == "" || targetID == dep.ModID) {
			continue
		}
		if targetID == dep.ModID {
			continue // skip self-references
		}
		linkKey := dep.ModID + "|" + targetID
		if seen[linkKey] {
			continue
		}
		seen[linkKey] = true

		if _, ok := modSet[targetID]; !ok {
			modSet[targetID] = nil
			name := dep.DepName
			if name == "" {
				name = dep.DepModID
			}
			g.Nodes = append(g.Nodes, GraphNode{
				ID:    targetID,
				Name:  name,
				Group: "missing",
			})
		}
		g.Links = append(g.Links, GraphLink{
			Source: dep.ModID,
			Target: targetID,
			Type:   dep.Type,
		})
	}

	return g
}

// FindMissingDependencies returns required dependencies not present in the mod list.
func FindMissingDependencies(mods []db.Mod, deps []db.Dependency) []MissingDep {
	modSet := make(map[string]bool)
	for _, m := range mods {
		modSet[m.ID] = true
	}
	idMap := buildIDMap(mods)
	providerMap := buildProviderMap(mods)

	var missing []MissingDep
	seen := make(map[string]bool)
	for _, dep := range deps {
		if dep.Type != "required" {
			continue
		}
		targetID, satisfied := resolveDepTarget(dep.ModID, dep.DepModID, idMap, providerMap)
		if !ShouldDisplayDependency(dep, targetID, satisfied) {
			continue
		}
		if satisfied {
			continue
		}
		if modSet[targetID] {
			continue
		}
		key := dep.ModID + "|" + targetID
		if seen[key] {
			continue
		}
		seen[key] = true
		missing = append(missing, MissingDep{
			RequiredBy: dep.ModID,
			DepModID:   dep.DepModID,
			DepName:    dep.DepName,
		})
	}
	return missing
}

// FindUnusedLibraries finds mods that are libraries but no non-library mod depends on them.
func FindUnusedLibraries(mods []db.Mod, deps []db.Dependency) []string {
	modSet := make(map[string]*db.Mod)
	for i := range mods {
		modSet[mods[i].ID] = &mods[i]
	}
	idMap := buildIDMap(mods)
	providerMap := buildProviderMap(mods)

	// Transitively mark all mods reachable from non-library mods as "needed"
	needed := make(map[string]bool)
	var markNeeded func(id string)
	markNeeded = func(id string) {
		canonical := id
		if resolved, ok := idMap[id]; ok {
			canonical = resolved
		}
		if needed[canonical] {
			return
		}
		needed[canonical] = true
		for _, dep := range deps {
			if dep.ModID == canonical {
				targetID, satisfied := resolveDepTarget(canonical, dep.DepModID, idMap, providerMap)
				if !satisfied || targetID == "" || targetID == canonical {
					continue
				}
				markNeeded(targetID)
			}
		}
	}

	for _, m := range mods {
		if !m.IsLibrary {
			markNeeded(m.ID)
		}
	}

	var unused []string
	for _, m := range mods {
		if m.IsLibrary && !needed[m.ID] {
			unused = append(unused, m.ID)
		}
	}
	return unused
}

// UpdateSatisfied updates the Satisfied field on dependencies based on installed mods.
func UpdateSatisfied(deps []db.Dependency, mods []db.Mod) []db.Dependency {
	idMap := buildIDMap(mods)
	providerMap := buildProviderMap(mods)
	modIDs := make(map[string]bool)
	for _, m := range mods {
		modIDs[m.ID] = true
	}
	result := make([]db.Dependency, len(deps))
	copy(result, deps)
	for i := range result {
		canonical, satisfied := resolveDepTarget(result[i].ModID, result[i].DepModID, idMap, providerMap)
		result[i].Satisfied = satisfied && (canonical == "" || modIDs[canonical])
	}
	return result
}
