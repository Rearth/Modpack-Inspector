package scanner

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"modpacktool/internal/db"

	"github.com/BurntSushi/toml"
)

// --- Manifest types ---

type modsToml struct {
	ModLoader     string                   `toml:"modLoader"`
	LoaderVersion string                   `toml:"loaderVersion"`
	Mods          []modsTomlEntry          `toml:"mods"`
	Dependencies  map[string][]modsTomlDep `toml:"dependencies"`
}

type modsTomlEntry struct {
	ModId       string `toml:"modId"`
	Version     string `toml:"version"`
	DisplayName string `toml:"displayName"`
	Description string `toml:"description"`
	Authors     string `toml:"authors"`
	LogoFile    string `toml:"logoFile"`
}

type modsTomlDep struct {
	ModId        string `toml:"modId"`
	Mandatory    bool   `toml:"mandatory"`
	Type         string `toml:"type"` // newer NeoForge format
	VersionRange string `toml:"versionRange"`
	Side         string `toml:"side"`
}

type fabricMod struct {
	ID          string            `json:"id"`
	Version     string            `json:"version"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Authors     []json.RawMessage `json:"authors"`
	Contact     map[string]string `json:"contact"`
	Depends     map[string]string `json:"depends"`
	Suggests    map[string]string `json:"suggests"`
	Provides    []string          `json:"provides"`
	Icon        string            `json:"icon"`
}

type quiltMod struct {
	SchemaVersion int `json:"schema_version"`
	QuiltLoader   struct {
		Group    string `json:"group"`
		ID       string `json:"id"`
		Version  string `json:"version"`
		Metadata struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"metadata"`
		Depends  []quiltDep `json:"depends"`
		Provides []string   `json:"provides"`
	} `json:"quilt_loader"`
}

type quiltDep struct {
	ID       string `json:"id"`
	Versions string `json:"versions"`
	Optional bool   `json:"optional"`
}

// ScanResult contains a parsed mod and its dependencies from the JAR manifest.
type ScanResult struct {
	Mod             db.Mod
	Deps            []db.Dependency
	Mixins          []db.Mixin
	PackagePrefixes []string
}

// ScanModsFolder scans all .jar files in the given directory and extracts metadata.
// The optional onProgress callback is called with (current, total) for each file processed.
func ScanModsFolder(modsDir string, onProgress func(current, total int)) ([]ScanResult, error) {
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		return nil, fmt.Errorf("reading mods directory: %w", err)
	}

	// Pre-filter to JARs so we can report accurate total
	var jars []os.DirEntry
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".jar") {
			jars = append(jars, entry)
		}
	}

	results := make([]ScanResult, 0, len(jars))
	for i, entry := range jars {
		if onProgress != nil {
			onProgress(i+1, len(jars))
		}
		jarPath := filepath.Join(modsDir, entry.Name())
		result, err := ScanJar(jarPath)
		if err != nil {
			fmt.Printf("Warning: failed to scan %s: %v\n", entry.Name(), err)
			continue
		}
		results = append(results, *result)
	}
	return results, nil
}

// ScanJar parses a single JAR file for mod metadata.
func ScanJar(jarPath string) (*ScanResult, error) {
	data, err := os.ReadFile(jarPath)
	if err != nil {
		return nil, fmt.Errorf("reading jar: %w", err)
	}

	// Compute hashes
	sha1Sum := sha1.Sum(data)
	sha512Sum := sha512.Sum512(data)
	fingerprint := ComputeFingerprint(data)

	// Open as zip
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("opening jar as zip: %w", err)
	}

	fileName := filepath.Base(jarPath)

	// Detect all supported loaders and use first successful parse for metadata
	var result *ScanResult
	var detectedLoaders []string

	if res, _ := tryForgeManifest(r, "META-INF/neoforge.mods.toml"); res != nil {
		detectedLoaders = appendUnique(detectedLoaders, "neoforge")
		if result == nil {
			result = res
		}
	}
	if res, _ := tryForgeManifest(r, "META-INF/mods.toml"); res != nil {
		detectedLoaders = appendUnique(detectedLoaders, res.Mod.ModLoader)
		if result == nil {
			result = res
		}
	}
	if res, _ := tryFabricManifest(r); res != nil {
		detectedLoaders = appendUnique(detectedLoaders, "fabric")
		if result == nil {
			result = res
		}
	}
	if res, _ := tryQuiltManifest(r); res != nil {
		detectedLoaders = appendUnique(detectedLoaders, "quilt")
		if result == nil {
			result = res
		}
	}

	if result == nil {
		// No recognized manifest; create a minimal entry from filename
		modID := strings.TrimSuffix(fileName, ".jar")
		result = &ScanResult{
			Mod: db.Mod{
				ID:   modID,
				Name: modID,
			},
		}
	}

	result.Mod.Loaders = strings.Join(detectedLoaders, ",")

	// Extract mixin information
	result.Mixins = ExtractMixins(r, result.Mod.ID)
	result.PackagePrefixes = CollectPackagePrefixes(r)

	// Fill in hash and file info
	result.Mod.JarFileName = fileName
	result.Mod.JarSHA1 = hex.EncodeToString(sha1Sum[:])
	result.Mod.JarSHA512 = hex.EncodeToString(sha512Sum[:])
	result.Mod.Fingerprint = fingerprint
	result.Mod.LastScanned = time.Now()

	return result, nil
}

func tryForgeManifest(r *zip.Reader, path string) (*ScanResult, error) {
	f := findFile(r, path)
	if f == nil {
		return nil, nil
	}

	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	content, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	var mt modsToml
	if err := toml.Unmarshal(content, &mt); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	if len(mt.Mods) == 0 {
		return nil, nil
	}

	entry := mt.Mods[0]
	loader := "forge"
	if strings.Contains(path, "neoforge") || strings.Contains(strings.ToLower(mt.ModLoader), "neoforge") {
		loader = "neoforge"
	}

	mod := db.Mod{
		ID:          entry.ModId,
		Name:        entry.DisplayName,
		Version:     entry.Version,
		Description: strings.TrimSpace(entry.Description),
		Authors:     entry.Authors,
		ModLoader:   loader,
	}

	var deps []db.Dependency
	if modDeps, ok := mt.Dependencies[entry.ModId]; ok {
		for _, d := range modDeps {
			if isMinecraftOrLoader(d.ModId) {
				continue
			}
			depType := "required"
			if d.Type != "" {
				depType = d.Type
			} else if !d.Mandatory {
				depType = "optional"
			}
			deps = append(deps, db.Dependency{
				ModID:    entry.ModId,
				DepModID: d.ModId,
				DepName:  d.ModId,
				Type:     depType,
				Source:   "manifest",
			})
		}
	}

	return &ScanResult{Mod: mod, Deps: deps}, nil
}

func tryFabricManifest(r *zip.Reader) (*ScanResult, error) {
	f := findFile(r, "fabric.mod.json")
	if f == nil {
		return nil, nil
	}

	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var fm fabricMod
	if err := json.NewDecoder(rc).Decode(&fm); err != nil {
		return nil, fmt.Errorf("parsing fabric.mod.json: %w", err)
	}

	mod := db.Mod{
		ID:          fm.ID,
		Name:        fm.Name,
		Version:     fm.Version,
		Description: fm.Description,
		Authors:     parseFabricAuthors(fm.Authors),
		ModLoader:   "fabric",
		ProvidedIDs: joinProvidedIDs(append(fm.Provides, collectEmbeddedModIDs(r)...)),
	}
	if hp, ok := fm.Contact["homepage"]; ok {
		mod.HomepageURL = hp
	}

	var deps []db.Dependency
	for depID := range fm.Depends {
		if isMinecraftOrLoader(depID) {
			continue
		}
		deps = append(deps, db.Dependency{
			ModID:    fm.ID,
			DepModID: depID,
			DepName:  depID,
			Type:     "required",
			Source:   "manifest",
		})
	}
	for depID := range fm.Suggests {
		if isMinecraftOrLoader(depID) {
			continue
		}
		deps = append(deps, db.Dependency{
			ModID:    fm.ID,
			DepModID: depID,
			DepName:  depID,
			Type:     "optional",
			Source:   "manifest",
		})
	}

	return &ScanResult{Mod: mod, Deps: deps}, nil
}

func tryQuiltManifest(r *zip.Reader) (*ScanResult, error) {
	f := findFile(r, "quilt.mod.json")
	if f == nil {
		return nil, nil
	}

	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var qm quiltMod
	if err := json.NewDecoder(rc).Decode(&qm); err != nil {
		return nil, fmt.Errorf("parsing quilt.mod.json: %w", err)
	}

	mod := db.Mod{
		ID:          qm.QuiltLoader.ID,
		Name:        qm.QuiltLoader.Metadata.Name,
		Version:     qm.QuiltLoader.Version,
		Description: qm.QuiltLoader.Metadata.Description,
		ModLoader:   "quilt",
		ProvidedIDs: joinProvidedIDs(append(qm.QuiltLoader.Provides, collectEmbeddedModIDs(r)...)),
	}

	var deps []db.Dependency
	for _, d := range qm.QuiltLoader.Depends {
		if isMinecraftOrLoader(d.ID) {
			continue
		}
		depType := "required"
		if d.Optional {
			depType = "optional"
		}
		deps = append(deps, db.Dependency{
			ModID:    qm.QuiltLoader.ID,
			DepModID: d.ID,
			DepName:  d.ID,
			Type:     depType,
			Source:   "manifest",
		})
	}

	return &ScanResult{Mod: mod, Deps: deps}, nil
}

func findFile(r *zip.Reader, name string) *zip.File {
	for _, f := range r.File {
		if f.Name == name {
			return f
		}
	}
	return nil
}

func parseFabricAuthors(raw []json.RawMessage) string {
	var names []string
	for _, r := range raw {
		var s string
		if json.Unmarshal(r, &s) == nil {
			names = append(names, s)
			continue
		}
		var obj struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(r, &obj) == nil && obj.Name != "" {
			names = append(names, obj.Name)
		}
	}
	return strings.Join(names, ", ")
}

func joinProvidedIDs(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	seen := make(map[string]bool)
	filtered := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || isMinecraftOrLoader(id) || seen[id] {
			continue
		}
		seen[id] = true
		filtered = append(filtered, id)
	}
	return strings.Join(filtered, ",")
}

func collectEmbeddedModIDs(r *zip.Reader) []string {
	var ids []string
	seen := make(map[string]bool)
	for _, file := range r.File {
		if !strings.HasPrefix(file.Name, "META-INF/jars/") || !strings.HasSuffix(file.Name, ".jar") {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}
		inner, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			continue
		}
		for _, id := range embeddedModIDs(inner) {
			if seen[id] {
				continue
			}
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

func embeddedModIDs(r *zip.Reader) []string {
	if f := findFile(r, "fabric.mod.json"); f != nil {
		rc, err := f.Open()
		if err == nil {
			defer rc.Close()
			var fm fabricMod
			if json.NewDecoder(rc).Decode(&fm) == nil && fm.ID != "" && !isMinecraftOrLoader(fm.ID) {
				return []string{fm.ID}
			}
		}
	}
	if f := findFile(r, "quilt.mod.json"); f != nil {
		rc, err := f.Open()
		if err == nil {
			defer rc.Close()
			var qm quiltMod
			if json.NewDecoder(rc).Decode(&qm) == nil && qm.QuiltLoader.ID != "" && !isMinecraftOrLoader(qm.QuiltLoader.ID) {
				return []string{qm.QuiltLoader.ID}
			}
		}
	}
	return nil
}

func isMinecraftOrLoader(id string) bool {
	skip := map[string]bool{
		"minecraft": true, "forge": true, "neoforge": true,
		"fabricloader": true, "fabric-api": true, "fabric": true,
		"quilt_loader": true, "quilted_fabric_api": true,
		"java": true,
	}
	return skip[id]
}

func appendUnique(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}

// ComputeFingerprint computes the CurseForge MurmurHash2 fingerprint.
func ComputeFingerprint(data []byte) uint32 {
	// Strip whitespace bytes (9=tab, 10=LF, 13=CR, 32=space)
	stripped := make([]byte, 0, len(data))
	for _, b := range data {
		if b != 9 && b != 10 && b != 13 && b != 32 {
			stripped = append(stripped, b)
		}
	}
	return murmurHash2(stripped, 1)
}

func murmurHash2(data []byte, seed uint32) uint32 {
	const m uint32 = 0x5bd1e995
	const r = 24

	length := len(data)
	h := seed ^ uint32(length)

	i := 0
	for length >= 4 {
		k := uint32(data[i]) | uint32(data[i+1])<<8 | uint32(data[i+2])<<16 | uint32(data[i+3])<<24
		k *= m
		k ^= k >> r
		k *= m
		h *= m
		h ^= k
		i += 4
		length -= 4
	}

	switch length {
	case 3:
		h ^= uint32(data[i+2]) << 16
		fallthrough
	case 2:
		h ^= uint32(data[i+1]) << 8
		fallthrough
	case 1:
		h ^= uint32(data[i])
		h *= m
	}

	h ^= h >> 13
	h *= m
	h ^= h >> 15
	return h
}
