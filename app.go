package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"modpacktool/internal/api"
	"modpacktool/internal/db"
	"modpacktool/internal/embeddings"
	"modpacktool/internal/instance"
	"modpacktool/internal/resolver"
	"modpacktool/internal/scanner"
)

// App is the main application struct exposed to the frontend via Wails.
type App struct {
	ctx     context.Context
	db      *db.Database
	embeds  *embeddings.Engine
	watcher *scanner.Watcher

	cfClient *api.CurseForgeClient
	mrClient *api.ModrinthClient

	dataDir   string
	modsDir   string
	configDir string

	liveLogMu     sync.Mutex
	liveLogCancel context.CancelFunc

	scanGen atomic.Uint64 // incremented on each scan start; checked to abort stale scans

	libraryClassifierOnce    sync.Once
	libraryPositiveVecs      [][]float64
	libraryNegativeVecs      [][]float64
	mixedTagLibraryThreshold float64
	noTagLibraryThreshold    float64
}

const (
	defaultMixedTagLibraryThreshold = 0.18
	defaultNoTagLibraryThreshold    = 0.26
)

func NewApp() *App {
	return &App{
		embeds:                   embeddings.NewEngine(),
		mixedTagLibraryThreshold: defaultMixedTagLibraryThreshold,
		noTagLibraryThreshold:    defaultNoTagLibraryThreshold,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Initialize database in user data dir
	dataDir, err := os.UserConfigDir()
	if err != nil {
		dataDir = "."
	}
	dataDir = filepath.Join(dataDir, "ModpackTool")
	os.MkdirAll(dataDir, 0755)
	a.dataDir = dataDir

	database, err := db.New(dataDir)
	if err != nil {
		fmt.Println("Failed to open database:", err)
		return
	}
	a.db = database

	// Load settings and initialize API clients
	settings, _ := a.db.GetSettings()
	cacheTTL := 24 * time.Hour
	a.applyLibraryDetectionSettings(settings)

	a.cfClient = api.NewCurseForgeClient(settings.CurseForgeAPIKey, a.db, cacheTTL)
	a.mrClient = api.NewModrinthClient(settings.ModrinthAPIKey, a.db, cacheTTL)

	// Initialize embedding model (download if needed, non-fatal if unavailable)
	go a.initEmbeddingModel(settings.InstancePath)
}

func (a *App) initEmbeddingModel(instancePath string) {
	// Try downloading model files if needed
	runtime.EventsEmit(a.ctx, "scan:progress", "Setting up embedding model...")
	err := embeddings.EnsureModelFiles(a.dataDir, func(stage string, pct float64) {
		runtime.EventsEmit(a.ctx, "scan:progress", stage)
	})
	if err != nil {
		fmt.Println("Embedding model setup skipped:", err)
	} else {
		if err := a.embeds.Init(a.dataDir); err != nil {
			fmt.Println("Embedding model load failed:", err)
		} else {
			fmt.Println("Embedding model loaded successfully")
		}
	}

	// Start scan if instance is configured
	if instancePath != "" {
		a.setInstanceDirs(instancePath)
		a.performFullScan()
	}
}

func (a *App) shutdown(ctx context.Context) {
	a.StopLiveLog()
	if a.watcher != nil {
		a.watcher.Close()
	}
	a.embeds.Destroy()
	if a.db != nil {
		a.db.Close()
	}
}

func (a *App) setInstanceDirs(instancePath string) {
	a.StopLiveLog()

	if a.watcher != nil {
		a.watcher.Close()
		a.watcher = nil
	}

	if instancePath == "" {
		a.modsDir = ""
		a.configDir = ""
		return
	}

	a.modsDir = filepath.Join(instancePath, "mods")
	a.configDir = filepath.Join(instancePath, "config")

	// Start filesystem watcher
	w, err := scanner.NewWatcher(a.modsDir, a.configDir)
	if err != nil {
		fmt.Println("Watcher error:", err)
		return
	}
	a.watcher = w
	go a.watchForChanges()
}

func (a *App) watchForChanges() {
	for {
		select {
		case event, ok := <-a.watcher.Events:
			if !ok {
				return
			}
			switch event.Type {
			case "mod_added", "mod_changed":
				go a.scanSingleMod(event.Path)
			case "mod_removed":
				go a.removeSingleMod(event.Name)
			case "config_changed":
				runtime.EventsEmit(a.ctx, "configs:updated")
			}
		case _, ok := <-a.watcher.Errors:
			if !ok {
				return
			}
		}
	}
}

func (a *App) removeSingleMod(fileName string) {
	modFiles, err := a.db.GetModFileNames()
	if err == nil {
		if modID, ok := modFiles[fileName]; ok {
			a.db.DeleteDependenciesByModID(modID)
			a.db.DeleteMixinsByModID(modID)
			a.db.DeleteMod(modID)
		} else {
			a.db.DeleteModByFileName(fileName)
		}
	} else {
		a.db.DeleteModByFileName(fileName)
	}

	a.rescanConfigs()
	runtime.EventsEmit(a.ctx, "mods:updated")
	runtime.EventsEmit(a.ctx, "scan:complete")
}

func (a *App) scanSingleMod(jarPath string) {
	result, err := scanner.ScanJar(jarPath)
	if err != nil {
		return
	}

	// Enrich from APIs
	result.Deps = a.enrichMod(&result.Mod, result.Deps)

	// Generate embedding if model available
	if a.embeds.IsAvailable() {
		text := a.buildEmbeddingText(&result.Mod, result.Deps)
		vec := a.embeds.Embed(text)
		result.Mod.Embedding = embeddings.EmbedToBytes(vec)
	}

	a.db.UpsertMod(&result.Mod)
	a.db.DeleteDependenciesByModID(result.Mod.ID)
	for _, dep := range result.Deps {
		a.db.UpsertDependency(&dep)
	}

	// Save mixins (best-effort resolution without full package map)
	scanner.ResolveKnownTargets(result.Mixins, result.Mod.ID)
	a.db.DeleteMixinsByModID(result.Mod.ID)
	for i := range result.Mixins {
		a.db.UpsertMixin(&result.Mixins[i])
	}

	a.rescanConfigs()
	runtime.EventsEmit(a.ctx, "mods:updated")
	runtime.EventsEmit(a.ctx, "scan:complete")
}

func (a *App) enrichMod(mod *db.Mod, manifestDeps []db.Dependency) []db.Dependency {
	var categories []string
	deps := append([]db.Dependency{}, manifestDeps...)

	// Try CurseForge
	if cfInfo, err := a.cfClient.EnrichMod(mod.Fingerprint); err == nil && cfInfo != nil {
		mod.CurseForgeID = cfInfo.CurseForgeID
		mod.CurseForgeURL = cfInfo.CurseForgeURL
		if mod.IconURL == "" {
			mod.IconURL = cfInfo.IconURL
		}
		if cfInfo.Description != "" {
			mod.OnlineDesc = cfInfo.Description
		}
		categories = append(categories, cfInfo.Categories...)
		deps = mergeDependencies(deps, onlineDepsForMod(mod.ID, cfInfo.Dependencies)...)
		mod.LastAPICheck = time.Now()
	}

	// Try Modrinth
	if mrInfo, err := a.mrClient.EnrichMod(mod.JarSHA1); err == nil && mrInfo != nil {
		mod.ModrinthID = mrInfo.ModrinthID
		mod.ModrinthURL = mrInfo.ModrinthURL
		if mod.IconURL == "" {
			mod.IconURL = mrInfo.IconURL
		}
		if mod.OnlineDesc == "" && mrInfo.Description != "" {
			mod.OnlineDesc = mrInfo.Description
		}
		if mrInfo.ProjectType != "" {
			mod.ProjectType = mrInfo.ProjectType
		}
		categories = append(categories, mrInfo.Categories...)
		// Merge API-detected loaders
		if len(mrInfo.Loaders) > 0 {
			existing := splitNonEmpty(mod.Loaders)
			for _, l := range mrInfo.Loaders {
				existing = appendUniqueStr(existing, l)
			}
			mod.Loaders = strings.Join(existing, ",")
		}
		deps = mergeDependencies(deps, onlineDepsForMod(mod.ID, mrInfo.Dependencies)...)
		mod.LastAPICheck = time.Now()
	}

	// Deduplicate and store categories
	if len(categories) > 0 {
		seen := make(map[string]bool)
		var unique []string
		for _, c := range categories {
			cl := strings.ToLower(c)
			if !seen[cl] {
				seen[cl] = true
				unique = append(unique, c)
			}
		}
		mod.Categories = strings.Join(unique, ",")
	}

	// Library classification: global manual override takes precedence, then auto-detect
	if override, _ := a.db.GetLibraryOverride(mod.ID); override != 0 {
		mod.LibraryOverride = override
		mod.IsLibrary = override == 1
	} else if mod.Categories != "" {
		cats := splitNonEmpty(mod.Categories)
		if a.evaluateLibraryDetection(cats, mod.Name, mod.ID, mod.Description, mod.OnlineDesc).Detected {
			mod.IsLibrary = true
		}
	}

	return deps
}

var libraryPositivePrompts = []string{
	"shared code library dependency required by other minecraft mods",
	"developer api and framework providing hooks for minecraft modding",
	"core library providing reusable utility code for mod developers",
	"compatibility and abstraction layer library for minecraft mods",
}

var libraryNegativePrompts = []string{
	"standalone gameplay mod with new items blocks and progression for players",
	"player-facing adventure mod with quests exploration and rpg elements",
	"building and decoration mod adding cosmetic blocks and furniture",
	"technology and automation mod with machines energy and industrial systems",
	"world generation mod creating new biomes terrain and structures to explore",
	"combat and weapons mod adding new player abilities and equipment",
}

func (a *App) detectLibrary(categories []string, texts ...string) bool {
	return a.evaluateLibraryDetection(categories, texts...).Detected
}

type LibraryDetectionDebug struct {
	Detected                     bool    `json:"detected"`
	HasLibraryTag                bool    `json:"hasLibraryTag"`
	HasOnlyLibraryTags           bool    `json:"hasOnlyLibraryTags"`
	ContentTagCount              int     `json:"contentTagCount"`
	UsedSemantic                 bool    `json:"usedSemantic"`
	SemanticConfidence           float64 `json:"semanticConfidence"`
	PositiveSimilarity           float64 `json:"positiveSimilarity"`
	NegativeSimilarity           float64 `json:"negativeSimilarity"`
	Threshold                    float64 `json:"threshold"`
	UsedStrongDescription        bool    `json:"usedStrongDescription"`
	DescriptionHasLibraryKeyword bool    `json:"descriptionHasLibraryKeyword"`
	ManualOverride               int     `json:"manualOverride"`
	Reason                       string  `json:"reason"`
}

func (a *App) evaluateLibraryDetection(categories []string, texts ...string) LibraryDetectionDebug {
	semanticConfidence, positiveSimilarity, negativeSimilarity, hasSemanticConfidence := a.librarySemanticConfidence(texts...)
	return evaluateLibraryDetectionWithThresholds(categories, semanticConfidence, positiveSimilarity, negativeSimilarity, hasSemanticConfidence, a.mixedTagLibraryThreshold, a.noTagLibraryThreshold, texts...)
}

func evaluateLibraryDetectionWithThresholds(categories []string, semanticConfidence, positiveSimilarity, negativeSimilarity float64, hasSemanticConfidence bool, mixedTagThreshold, noTagThreshold float64, texts ...string) LibraryDetectionDebug {
	hasLibraryTag, contentTagCount := analyzeLibraryCategories(categories)
	strongDesc := hasStrongLibraryDescription(texts...)
	descKeyword := descriptionMentionsLibrary(texts...)

	debug := LibraryDetectionDebug{
		HasLibraryTag:                hasLibraryTag,
		HasOnlyLibraryTags:           hasLibraryTag && contentTagCount == 0,
		ContentTagCount:              contentTagCount,
		UsedStrongDescription:        strongDesc,
		DescriptionHasLibraryKeyword: descKeyword,
	}

	// Pure library tags (no content tags) → hard positive
	if hasLibraryTag && contentTagCount == 0 {
		debug.Detected = true
		debug.Reason = "pure-library-tags"
		return debug
	}

	// Narrow strong description → detected regardless of tags
	if strongDesc {
		debug.Detected = true
		debug.Reason = "strong-description"
	}

	// Library tag + description mentions library keyword → detected
	if hasLibraryTag && descKeyword && !debug.Detected {
		debug.Detected = true
		debug.Reason = "tagged-library-description"
	}

	// Semantic check
	debug.UsedSemantic = hasSemanticConfidence
	debug.SemanticConfidence = semanticConfidence
	debug.PositiveSimilarity = positiveSimilarity
	debug.NegativeSimilarity = negativeSimilarity

	if hasSemanticConfidence {
		debug.Threshold = noTagThreshold
		if hasLibraryTag {
			debug.Threshold = mixedTagThreshold
		}
		if semanticConfidence >= debug.Threshold {
			debug.Detected = true
			if debug.Reason == "" {
				debug.Reason = "semantic-match"
			}
		} else if debug.Reason == "" {
			debug.Reason = "semantic-below-threshold"
		}
		return debug
	}

	if debug.Reason == "" {
		debug.Reason = "no-model-no-match"
	}
	return debug
}

// neutralCategoryTags are tags that don't indicate content — they're compatible with being a library.
var neutralCategoryTags = map[string]bool{
	"utility":             true,
	"server utility":      true,
	"utility & qol":       true,
	"miscellaneous":       true,
	"addons":              true,
	"management":          true,
	"education":           true,
	"kubejs":              true,
	"map and information": true,
}

func analyzeLibraryCategories(categories []string) (hasLibraryTag bool, contentTagCount int) {
	for _, category := range categories {
		normalized := strings.ToLower(strings.TrimSpace(category))
		if normalized == "" {
			continue
		}
		switch normalized {
		case "api & library", "api and library", "libraries", "library":
			hasLibraryTag = true
		default:
			if !neutralCategoryTags[normalized] {
				contentTagCount++
			}
		}
	}
	return
}

func (a *App) librarySemanticConfidence(texts ...string) (float64, float64, float64, bool) {
	if a == nil || a.embeds == nil || !a.embeds.IsAvailable() {
		return 0, 0, 0, false
	}

	text := joinNonEmpty(texts...)
	if text == "" {
		return 0, 0, 0, false
	}

	a.initLibraryClassifier()
	if len(a.libraryPositiveVecs) == 0 || len(a.libraryNegativeVecs) == 0 {
		return 0, 0, 0, false
	}

	textVec := a.embeds.Embed(text)
	if len(textVec) == 0 {
		return 0, 0, 0, false
	}

	positive := maxCosineSimilarity(textVec, a.libraryPositiveVecs)
	negative := maxCosineSimilarity(textVec, a.libraryNegativeVecs)
	return positive - negative, positive, negative, true
}

func (a *App) applyLibraryDetectionSettings(settings *db.Settings) {
	if settings == nil {
		a.mixedTagLibraryThreshold = defaultMixedTagLibraryThreshold
		a.noTagLibraryThreshold = defaultNoTagLibraryThreshold
		return
	}
	a.mixedTagLibraryThreshold = clampLibraryThreshold(settings.MixedTagLibraryThreshold, defaultMixedTagLibraryThreshold)
	a.noTagLibraryThreshold = clampLibraryThreshold(settings.NoTagLibraryThreshold, defaultNoTagLibraryThreshold)
}

// SetLibraryOverride sets a manual library detection override for a mod.
// override: 1 = force library, -1 = force not library, 0 = auto-detect.
func (a *App) SetLibraryOverride(modID string, override int) error {
	if override < -1 || override > 1 {
		return fmt.Errorf("invalid override value: must be -1, 0, or 1")
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := a.db.SetLibraryOverride(modID, override); err != nil {
		return err
	}
	if err := a.db.UpdateModLibraryOverride(modID, override); err != nil {
		return err
	}
	if override != 0 {
		if err := a.db.UpdateModIsLibrary(modID, override == 1); err != nil {
			return err
		}
	} else {
		mod, err := a.db.GetModByID(modID)
		if err == nil && mod != nil {
			cats := splitNonEmpty(mod.Categories)
			detected := a.evaluateLibraryDetection(cats, mod.Name, mod.ID, mod.Description, mod.OnlineDesc).Detected
			if err := a.db.UpdateModIsLibrary(modID, detected); err != nil {
				return err
			}
		}
	}
	runtime.EventsEmit(a.ctx, "mods:updated")
	return nil
}

func clampLibraryThreshold(value, fallback float64) float64 {
	if value <= 0 {
		value = fallback
	}
	if value < 0.01 {
		return 0.01
	}
	if value > 0.95 {
		return 0.95
	}
	return value
}

func (a *App) initLibraryClassifier() {
	a.libraryClassifierOnce.Do(func() {
		if a == nil || a.embeds == nil || !a.embeds.IsAvailable() {
			return
		}
		for _, prompt := range libraryPositivePrompts {
			vec := a.embeds.Embed(prompt)
			if len(vec) > 0 {
				a.libraryPositiveVecs = append(a.libraryPositiveVecs, vec)
			}
		}
		for _, prompt := range libraryNegativePrompts {
			vec := a.embeds.Embed(prompt)
			if len(vec) > 0 {
				a.libraryNegativeVecs = append(a.libraryNegativeVecs, vec)
			}
		}
	})
}

func maxCosineSimilarity(target []float64, candidates [][]float64) float64 {
	best := -1.0
	for _, candidate := range candidates {
		score := embeddings.CosineSimilarity(target, candidate)
		if score > best {
			best = score
		}
	}
	if best < 0 {
		return 0
	}
	return best
}

func joinNonEmpty(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			filtered = append(filtered, part)
		}
	}
	return strings.Join(filtered, " ")
}

func hasStrongLibraryDescription(texts ...string) bool {
	combined := strings.ToLower(joinNonEmpty(texts...))
	for _, phrase := range []string{
		"library containing",
		"shared code",
		"shared library",
		"common code",
		"core library",
		"reused between mods",
		"for other mods",
		"provides code, frameworks, and utilities for minecraft mods",
	} {
		if strings.Contains(combined, phrase) {
			return true
		}
	}

	if strings.Contains(combined, "provides code") && strings.Contains(combined, "framework") && strings.Contains(combined, "mods") {
		return true
	}
	if strings.Contains(combined, "library mod providing") {
		return true
	}
	if strings.Contains(combined, "open source library") || strings.Contains(combined, "opensource library") {
		return true
	}

	return false
}

// descriptionMentionsLibrary returns true if the combined texts contain library-related keywords.
// This is broader than hasStrongLibraryDescription and is designed to be used in combination
// with a library tag presence check.
func descriptionMentionsLibrary(texts ...string) bool {
	combined := strings.ToLower(joinNonEmpty(texts...))
	if strings.Contains(combined, "library") {
		return true
	}
	if containsWord(combined, "lib") || containsWord(combined, "api") {
		return true
	}
	// Compound words ending in "lib" or "api" (e.g. SmartBrainLib, lionfishapi)
	for _, w := range strings.Fields(combined) {
		if strings.HasSuffix(w, "lib") || strings.HasSuffix(w, "api") {
			return true
		}
	}
	return false
}

func containsWord(text, word string) bool {
	for i := 0; i <= len(text)-len(word); i++ {
		if text[i:i+len(word)] != word {
			continue
		}
		before := i == 0 || !isAlphanumeric(text[i-1])
		after := i+len(word) == len(text) || !isAlphanumeric(text[i+len(word)])
		if before && after {
			return true
		}
	}
	return false
}

func isAlphanumeric(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

func onlineDepsForMod(modID string, online []api.OnlineDep) []db.Dependency {
	deps := make([]db.Dependency, 0, len(online))
	for _, dep := range online {
		if dep.ModID == "" || dep.Type == "" {
			continue
		}
		name := dep.Name
		if name == "" {
			name = dep.ModID
		}
		deps = append(deps, db.Dependency{
			ModID:    modID,
			DepModID: dep.ModID,
			DepName:  name,
			Type:     dep.Type,
			Source:   dep.Source,
		})
	}
	return deps
}

func mergeDependencies(base []db.Dependency, extras ...db.Dependency) []db.Dependency {
	merged := append([]db.Dependency{}, base...)
	seen := make(map[string]bool, len(merged)+len(extras))
	for _, dep := range merged {
		seen[dependencyKey(dep)] = true
	}
	for _, dep := range extras {
		key := dependencyKey(dep)
		if seen[key] {
			continue
		}
		seen[key] = true
		merged = append(merged, dep)
	}
	return merged
}

func dependencyKey(dep db.Dependency) string {
	return dep.ModID + "|" + dep.DepModID + "|" + dep.Type + "|" + dep.Source
}

// buildEmbeddingText constructs the text used for embedding a mod.
// Includes name, description, online description, categories, and dependency mod IDs.
func (a *App) buildEmbeddingText(mod *db.Mod, deps []db.Dependency) string {
	parts := []string{mod.Name}
	if mod.Authors != "" {
		parts = append(parts, "by "+mod.Authors)
	}
	if mod.Description != "" {
		parts = append(parts, mod.Description)
	}
	if mod.OnlineDesc != "" {
		parts = append(parts, mod.OnlineDesc)
	}
	if mod.Categories != "" {
		parts = append(parts, mod.Categories)
	}
	for _, dep := range deps {
		if dep.DepName != "" {
			parts = append(parts, dep.DepName)
		} else {
			parts = append(parts, dep.DepModID)
		}
	}
	return strings.Join(parts, " ")
}

func splitNonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

func appendUniqueStr(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}

func (a *App) performFullScan() {
	if a.modsDir == "" {
		return
	}

	// Claim a generation number so stale scans can detect they've been superseded.
	myGen := a.scanGen.Add(1)

	runtime.EventsEmit(a.ctx, "scan:progress", "Scanning mods folder...")

	results, err := scanner.ScanModsFolder(a.modsDir, func(current, total int) {
		runtime.EventsEmit(a.ctx, "scan:progress", fmt.Sprintf("Scanning mod %d/%d...", current, total))
	})
	if err != nil {
		runtime.EventsEmit(a.ctx, "scan:error", err.Error())
		return
	}

	// Abort if a newer scan was started while we were reading JARs.
	if a.scanGen.Load() != myGen {
		return
	}

	total := len(results)

	// Pre-warm API caches with batch calls (turns ~400 individual requests into ~4)
	runtime.EventsEmit(a.ctx, "scan:progress", "Fetching mod info from APIs...")
	a.preWarmAPICache(results)

	if a.scanGen.Load() != myGen {
		return
	}

	// Enrich mods concurrently — most API calls now hit the pre-warmed cache,
	// only CurseForge file-dep lookups remain as live HTTP calls.
	enrichedDeps := make([][]db.Dependency, total)
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	var enrichedCount atomic.Int64

	for i := range results {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			enrichedDeps[idx] = a.enrichMod(&results[idx].Mod, results[idx].Deps)
			<-sem
			count := enrichedCount.Add(1)
			runtime.EventsEmit(a.ctx, "scan:progress", fmt.Sprintf("Enriching mod %d/%d: %s", count, total, results[idx].Mod.Name))
		}(i)
	}
	wg.Wait()

	if a.scanGen.Load() != myGen {
		return
	}

	// Delete mods in the DB that are no longer on disk
	activeFileNames := make(map[string]bool, len(results))
	for _, r := range results {
		activeFileNames[r.Mod.JarFileName] = true
	}
	a.db.DeleteStaleMods(activeFileNames)

	// Save to database and generate embeddings (sequential for SQLite safety)
	for i := range results {
		results[i].Deps = enrichedDeps[i]

		runtime.EventsEmit(a.ctx, "scan:progress", fmt.Sprintf("Indexing mod %d/%d: %s", i+1, total, results[i].Mod.Name))

		if a.embeds.IsAvailable() {
			text := a.buildEmbeddingText(&results[i].Mod, results[i].Deps)
			vec := a.embeds.Embed(text)
			results[i].Mod.Embedding = embeddings.EmbedToBytes(vec)
		}

		a.db.UpsertMod(&results[i].Mod)
		a.db.DeleteDependenciesByModID(results[i].Mod.ID)
		for _, dep := range results[i].Deps {
			a.db.UpsertDependency(&dep)
		}

		if (i+1)%5 == 0 || i == total-1 {
			runtime.EventsEmit(a.ctx, "mods:updated")
		}
	}

	// Resolve mixin targets across all mods and save
	runtime.EventsEmit(a.ctx, "scan:progress", "Analyzing mixin targets...")
	scanner.ResolveMixinTargets(results)
	for _, result := range results {
		a.db.DeleteMixinsByModID(result.Mod.ID)
		for i := range result.Mixins {
			a.db.UpsertMixin(&result.Mixins[i])
		}
	}

	// Scan configs and match
	runtime.EventsEmit(a.ctx, "scan:progress", "Matching config files...")
	a.rescanConfigs()

	runtime.EventsEmit(a.ctx, "scan:complete")
	runtime.EventsEmit(a.ctx, "mods:updated")
}

// preWarmAPICache performs batch API lookups and stores results in the cache,
// so individual enrichMod calls hit cache instead of making HTTP requests.
func (a *App) preWarmAPICache(results []scanner.ScanResult) {
	fingerprints := make([]uint32, 0, len(results))
	sha1Hashes := make([]string, 0, len(results))
	for _, r := range results {
		if r.Mod.Fingerprint != 0 {
			fingerprints = append(fingerprints, r.Mod.Fingerprint)
		}
		if r.Mod.JarSHA1 != "" {
			sha1Hashes = append(sha1Hashes, r.Mod.JarSHA1)
		}
	}

	var wg sync.WaitGroup

	// CurseForge: batch fingerprint match → batch mod details
	wg.Add(1)
	go func() {
		defer wg.Done()
		cfMatches := a.cfClient.BatchMatchFingerprints(fingerprints)
		if len(cfMatches) > 0 {
			modIDs := make([]int, 0, len(cfMatches))
			for _, m := range cfMatches {
				modIDs = append(modIDs, m.File.ModID)
			}
			a.cfClient.BatchGetMods(modIDs)
		}
	}()

	// Modrinth: batch hash match → batch project details
	wg.Add(1)
	go func() {
		defer wg.Done()
		mrVersions := a.mrClient.BatchMatchHashes(sha1Hashes)
		if len(mrVersions) > 0 {
			projectIDs := make([]string, 0, len(mrVersions))
			seen := make(map[string]bool)
			for _, v := range mrVersions {
				if !seen[v.ProjectID] {
					seen[v.ProjectID] = true
					projectIDs = append(projectIDs, v.ProjectID)
				}
			}
			a.mrClient.BatchGetProjects(projectIDs)
		}
	}()

	wg.Wait()
}

func (a *App) rescanConfigs() {
	if a.configDir == "" {
		return
	}

	configs, err := scanner.ScanConfigFolder(a.configDir)
	if err != nil {
		return
	}

	mods, _ := a.db.GetAllMods()
	manualOverrides, _ := a.db.GetAllConfigMappings()

	// Keep only manual overrides
	var manual []db.ConfigMapping
	for _, m := range manualOverrides {
		if m.IsManual {
			manual = append(manual, m)
		}
	}

	// Clear non-manual mappings, then re-compute
	a.db.DeleteNonManualMappings()
	mappings := scanner.MatchConfigsToMods(configs, mods, manual, func(current, total int) {
		if total <= 0 {
			return
		}
		runtime.EventsEmit(a.ctx, "scan:progress", fmt.Sprintf("Matching config files %d/%d", current, total))
	})
	for _, m := range mappings {
		a.db.UpsertConfigMapping(&m)
	}

	runtime.EventsEmit(a.ctx, "configs:updated")
}

// --- Wails-bound methods (called from frontend) ---

// GetMods returns all mods from the database.
func (a *App) GetMods() ([]db.Mod, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return a.db.GetAllMods()
}

// GetModDetail returns a single mod by ID with its dependencies and configs.
func (a *App) GetModDetail(modID string) (*ModDetail, error) {
	mod, err := a.db.GetModByID(modID)
	if err != nil || mod == nil {
		return nil, err
	}
	deps, _ := a.db.GetDependenciesByModID(modID)
	configs, _ := a.db.GetConfigMappingsByModID(modID)
	mods, _ := a.db.GetAllMods()
	visibleDeps, unresolvedExternal := buildDetailDependencies(modID, deps, mods)
	libraryDetection := a.evaluateLibraryDetection(splitNonEmpty(mod.Categories), mod.Name, mod.ID, mod.Description, mod.OnlineDesc)
	libraryDetection.ManualOverride = mod.LibraryOverride
	if mod.LibraryOverride != 0 {
		libraryDetection.Detected = mod.LibraryOverride == 1
		if mod.LibraryOverride == 1 {
			libraryDetection.Reason = "manual-override-library"
		} else {
			libraryDetection.Reason = "manual-override-not-library"
		}
	}

	// Build mixin data
	rawMixins, _ := a.db.GetMixinsByModID(modID)
	mixinDetails := buildMixinDetails(rawMixins, mods)
	rawIncoming, _ := a.db.GetMixinsTargetingMod(modID)
	incomingDetails := buildIncomingMixins(rawIncoming, mods)

	return &ModDetail{
		Mod:                *mod,
		Dependencies:       visibleDeps,
		Configs:            configs,
		ProvidedModules:    splitProvidedModules(mod.ProvidedIDs),
		LibraryDetection:   libraryDetection,
		UnresolvedExternal: unresolvedExternal,
		Mixins:             mixinDetails,
		IncomingMixins:     incomingDetails,
	}, nil
}

// ModDetail bundles a mod with its related data.
type ModDetail struct {
	Mod                db.Mod                      `json:"mod"`
	Dependencies       []DetailDependency          `json:"dependencies"`
	Configs            []db.ConfigMapping          `json:"configs"`
	ProvidedModules    []string                    `json:"providedModules"`
	LibraryDetection   LibraryDetectionDebug       `json:"libraryDetection"`
	UnresolvedExternal []UnresolvedExternalDepLink `json:"unresolvedExternal,omitempty"`
	Mixins             []MixinDetail               `json:"mixins,omitempty"`
	IncomingMixins     []IncomingMixin             `json:"incomingMixins,omitempty"`
}

type MixinDetail struct {
	MixinClass    string `json:"mixinClass"`
	TargetClass   string `json:"targetClass"`
	TargetModID   string `json:"targetModID"`
	TargetModName string `json:"targetModName,omitempty"`
	TargetMembers string `json:"targetMembers"`
}

type IncomingMixin struct {
	OwnerModID    string `json:"ownerModID"`
	OwnerModName  string `json:"ownerModName"`
	OwnerIconURL  string `json:"ownerIconURL,omitempty"`
	MixinClass    string `json:"mixinClass"`
	TargetClass   string `json:"targetClass"`
	TargetMembers string `json:"targetMembers"`
}

type DetailDependency struct {
	db.Dependency
	ResolvedModID   string   `json:"resolvedModID,omitempty"`
	ResolvedName    string   `json:"resolvedName,omitempty"`
	ResolvedIconURL string   `json:"resolvedIconURL,omitempty"`
	Sources         []string `json:"sources,omitempty"`
}

type UnresolvedExternalDepLink struct {
	DepModID string `json:"depModID"`
	DepName  string `json:"depName,omitempty"`
	Type     string `json:"type"`
	Source   string `json:"source"`
	OpenURL  string `json:"openURL,omitempty"`
}

func buildDetailDependencies(modID string, deps []db.Dependency, mods []db.Mod) ([]DetailDependency, []UnresolvedExternalDepLink) {
	modByID := make(map[string]db.Mod, len(mods))
	for _, candidate := range mods {
		modByID[candidate.ID] = candidate
	}

	visible := make([]DetailDependency, 0, len(deps))
	visibleByKey := make(map[string]*DetailDependency)
	visibleOrder := make([]string, 0, len(deps))
	unresolved := make([]UnresolvedExternalDepLink, 0)
	unresolvedByKey := make(map[string]*UnresolvedExternalDepLink)
	unresolvedOrder := make([]string, 0)

	for _, dep := range deps {
		resolvedID, satisfied := resolver.ResolveDependencyTarget(mods, modID, dep.DepModID)
		if !resolver.ShouldDisplayDependency(dep, resolvedID, satisfied) {
			if isUnresolvedExternalDependency(dep, satisfied) {
				key := unresolvedExternalKey(dep)
				item, exists := unresolvedByKey[key]
				if !exists {
					item = &UnresolvedExternalDepLink{
						DepModID: dep.DepModID,
						DepName:  dep.DepName,
						Type:     dep.Type,
						Source:   dep.Source,
						OpenURL:  unresolvedExternalURL(dep),
					}
					if item.DepName == "" {
						item.DepName = dep.DepModID
					}
					unresolvedByKey[key] = item
					unresolvedOrder = append(unresolvedOrder, key)
				} else {
					item.Type = mergeDependencyType(item.Type, dep.Type)
					if item.DepName == "" && dep.DepName != "" {
						item.DepName = dep.DepName
					}
					if item.OpenURL == "" {
						item.OpenURL = unresolvedExternalURL(dep)
					}
				}
			}
			continue
		}

		resolvedName := ""
		resolvedIconURL := ""
		if satisfied && resolvedID != "" && resolvedID != modID {
			if resolvedMod, ok := modByID[resolvedID]; ok {
				resolvedID = resolvedMod.ID
				resolvedName = resolvedMod.Name
				resolvedIconURL = resolvedMod.IconURL
			}
		}

		key := detailDependencyKey(dep, resolvedID)
		detailDep, exists := visibleByKey[key]
		if !exists {
			copyDep := dep
			detailDep = &DetailDependency{
				Dependency:      copyDep,
				ResolvedModID:   resolvedID,
				ResolvedName:    resolvedName,
				ResolvedIconURL: resolvedIconURL,
				Sources:         []string{},
			}
			if resolvedName != "" {
				detailDep.DepName = resolvedName
			}
			visibleByKey[key] = detailDep
			visibleOrder = append(visibleOrder, key)
		} else {
			detailDep.Type = mergeDependencyType(detailDep.Type, dep.Type)
			if detailDep.DepName == "" && dep.DepName != "" {
				detailDep.DepName = dep.DepName
			}
			if detailDep.ResolvedModID == "" && resolvedID != "" {
				detailDep.ResolvedModID = resolvedID
			}
			if detailDep.ResolvedName == "" && resolvedName != "" {
				detailDep.ResolvedName = resolvedName
				detailDep.DepName = resolvedName
			}
			if detailDep.ResolvedIconURL == "" && resolvedIconURL != "" {
				detailDep.ResolvedIconURL = resolvedIconURL
			}
		}
		appendDetailSource(detailDep, dep.Source)
		if detailDep.DepName == "" {
			detailDep.DepName = dep.DepModID
		}
	}

	for _, key := range visibleOrder {
		dep := visibleByKey[key]
		sortDetailSources(dep)
		visible = append(visible, *dep)
	}
	for _, key := range unresolvedOrder {
		candidate := unresolvedByKey[key]
		if shouldSurfaceUnresolvedExternal(*candidate, visibleByKey, mods) {
			unresolved = append(unresolved, *candidate)
		}
	}

	return visible, unresolved
}

func detailDependencyKey(dep db.Dependency, resolvedID string) string {
	if resolvedID != "" {
		return "resolved|" + resolvedID
	}
	if dep.DepName != "" {
		return "name|" + strings.ToLower(dep.DepName)
	}
	return "id|" + strings.ToLower(dep.DepModID)
}

func mergeDependencyType(current, next string) string {
	rank := map[string]int{"embedded": 1, "optional": 2, "required": 3}
	if rank[next] > rank[current] {
		return next
	}
	if current == "" {
		return next
	}
	return current
}

func appendDetailSource(dep *DetailDependency, source string) {
	if source == "" {
		return
	}
	for _, existing := range dep.Sources {
		if existing == source {
			return
		}
	}
	dep.Sources = append(dep.Sources, source)
}

// sortDetailSources orders sources as: manifest, curseforge, modrinth (then any others alphabetically).
func sortDetailSources(dep *DetailDependency) {
	rank := map[string]int{"manifest": 0, "curseforge": 1, "modrinth": 2}
	sort.SliceStable(dep.Sources, func(i, j int) bool {
		ri, oki := rank[dep.Sources[i]]
		rj, okj := rank[dep.Sources[j]]
		if !oki {
			ri = 99
		}
		if !okj {
			rj = 99
		}
		if ri != rj {
			return ri < rj
		}
		return dep.Sources[i] < dep.Sources[j]
	})
}

func isUnresolvedExternalDependency(dep db.Dependency, satisfied bool) bool {
	if satisfied {
		return false
	}
	return dep.Source == "modrinth" || dep.Source == "curseforge"
}

func unresolvedExternalKey(dep db.Dependency) string {
	nameKey := strings.ToLower(strings.TrimSpace(dep.DepName))
	if nameKey == "" {
		nameKey = strings.ToLower(strings.TrimSpace(dep.DepModID))
	}
	return dep.Source + "|" + nameKey
}

func unresolvedExternalURL(dep db.Dependency) string {
	query := strings.TrimSpace(dep.DepName)
	if query == "" {
		query = strings.TrimSpace(dep.DepModID)
	}
	if query == "" {
		return ""
	}

	encoded := url.QueryEscape(query)
	switch dep.Source {
	case "modrinth":
		return "https://modrinth.com/mods?q=" + encoded
	case "curseforge":
		return "https://www.curseforge.com/minecraft/search?page=1&pageSize=20&sortBy=relevancy&class=mc-mods&search=" + encoded
	default:
		return ""
	}
}

func shouldSurfaceUnresolvedExternal(dep UnresolvedExternalDepLink, visible map[string]*DetailDependency, mods []db.Mod) bool {
	depName := strings.ToLower(strings.TrimSpace(dep.DepName))
	depID := strings.ToLower(strings.TrimSpace(dep.DepModID))

	for _, item := range visible {
		if depName != "" && depName == strings.ToLower(strings.TrimSpace(item.DepName)) {
			return false
		}
		if depID != "" && depID == strings.ToLower(strings.TrimSpace(item.DepModID)) {
			return false
		}
	}

	for _, mod := range mods {
		if depName != "" && depName == strings.ToLower(strings.TrimSpace(mod.Name)) {
			return false
		}
		switch dep.Source {
		case "modrinth":
			if depID != "" && depID == strings.ToLower(strings.TrimSpace(mod.ModrinthID)) {
				return false
			}
		case "curseforge":
			if mod.CurseForgeID != 0 && depID == strings.ToLower(fmt.Sprintf("%d", mod.CurseForgeID)) {
				return false
			}
		}
	}

	return true
}

// SearchMods performs hybrid text + semantic search.
func (a *App) SearchMods(query string) ([]embeddings.SearchResult, error) {
	mods, err := a.db.GetAllMods()
	if err != nil {
		return nil, err
	}
	return embeddings.Search(query, mods, a.embeds), nil
}

// GetDependencyGraph returns the full dependency graph.
func (a *App) GetDependencyGraph() (*resolver.Graph, error) {
	mods, err := a.db.GetAllMods()
	if err != nil {
		return nil, err
	}
	deps, err := a.db.GetAllDependencies()
	if err != nil {
		return nil, err
	}

	// Update satisfied status using ID-aware resolution
	deps = resolver.UpdateSatisfied(deps, mods)

	unused := resolver.FindUnusedLibraries(mods, deps)
	unusedSet := make(map[string]bool, len(unused))
	for _, id := range unused {
		unusedSet[id] = true
	}
	return resolver.BuildGraph(mods, deps, unusedSet), nil
}

// GetMissingDependencies returns required deps that are not installed.
func (a *App) GetMissingDependencies() ([]resolver.MissingDep, error) {
	mods, err := a.db.GetAllMods()
	if err != nil {
		return nil, err
	}
	deps, err := a.db.GetAllDependencies()
	if err != nil {
		return nil, err
	}
	return resolver.FindMissingDependencies(mods, deps), nil
}

// GetUnusedLibraries returns library mods that no other mod depends on.
func (a *App) GetUnusedLibraries() ([]string, error) {
	mods, err := a.db.GetAllMods()
	if err != nil {
		return nil, err
	}
	deps, err := a.db.GetAllDependencies()
	if err != nil {
		return nil, err
	}
	return resolver.FindUnusedLibraries(mods, deps), nil
}

// GetConfigsForMod returns config mappings for a specific mod.
func (a *App) GetConfigsForMod(modID string) ([]db.ConfigMapping, error) {
	return a.db.GetConfigMappingsByModID(modID)
}

// ReadConfigFile reads a config file from disk.
func (a *App) ReadConfigFile(configPath string) (string, error) {
	// Resolve relative to config dir
	absPath := configPath
	if a.configDir != "" && !filepath.IsAbs(configPath) {
		absPath = filepath.Join(a.configDir, configPath)
	}

	// Security: ensure the path is within the config directory
	absPath, err := filepath.Abs(absPath)
	if err != nil {
		return "", err
	}
	configAbs, _ := filepath.Abs(a.configDir)
	if !strings.HasPrefix(strings.ToLower(absPath), strings.ToLower(configAbs)) {
		return "", fmt.Errorf("path is outside config directory")
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SaveConfigFile writes content to a config file.
func (a *App) SaveConfigFile(configPath, content string) error {
	absPath := configPath
	if a.configDir != "" && !filepath.IsAbs(configPath) {
		absPath = filepath.Join(a.configDir, configPath)
	}

	// Security check
	absPath, err := filepath.Abs(absPath)
	if err != nil {
		return err
	}
	configAbs, _ := filepath.Abs(a.configDir)
	if !strings.HasPrefix(strings.ToLower(absPath), strings.ToLower(configAbs)) {
		return fmt.Errorf("path is outside config directory")
	}

	return os.WriteFile(absPath, []byte(content), 0644)
}

// SetConfigOverride manually links a config file to a mod.
func (a *App) SetConfigOverride(modID, configPath string) error {
	return a.db.UpsertConfigMapping(&db.ConfigMapping{
		ConfigPath: configPath,
		ModID:      modID,
		Confidence: 100,
		IsManual:   true,
	})
}

// RemoveConfigOverride removes a manual config mapping.
func (a *App) RemoveConfigOverride(modID, configPath string) error {
	return a.db.DeleteConfigMapping(configPath, modID)
}

// GetSettings returns current app settings.
func (a *App) GetSettings() (*db.Settings, error) {
	if a.db == nil {
		return &db.Settings{}, nil
	}
	return a.db.GetSettings()
}

// SaveSettings persists settings and reinitializes clients.
func (a *App) SaveSettings(settings db.Settings) error {
	current, _ := a.db.GetSettings()
	instanceChanged := current == nil || current.InstancePath != settings.InstancePath

	if err := a.db.SaveSettings(&settings); err != nil {
		return err
	}

	// Reinitialize API clients
	cacheTTL := 24 * time.Hour
	a.cfClient = api.NewCurseForgeClient(settings.CurseForgeAPIKey, a.db, cacheTTL)
	a.mrClient = api.NewModrinthClient(settings.ModrinthAPIKey, a.db, cacheTTL)
	a.applyLibraryDetectionSettings(&settings)
	a.setInstanceDirs(settings.InstancePath)

	if instanceChanged {
		if err := a.db.ClearScanData(); err != nil {
			return err
		}
		runtime.EventsEmit(a.ctx, "mods:updated")
		runtime.EventsEmit(a.ctx, "configs:updated")
	}

	if settings.InstancePath != "" {
		go a.performFullScan()
	}

	return nil
}

// ScanNow triggers a full re-scan of the mods and config folders.
func (a *App) ScanNow() {
	go a.performFullScan()
}

func (a *App) detectInstancesForSettings(settings db.Settings) []instance.Instance {
	return instance.DetectInstances(&instance.DetectOptions{
		ModrinthRoots:   instance.ParseRoots(settings.CustomModrinthRoot),
		CurseForgeRoots: instance.ParseRoots(settings.CustomCurseForgeRoot),
		FTBRoots:        instance.ParseRoots(settings.CustomFTBRoot),
		OtherRoots:      instance.ParseRoots(settings.CustomLauncherRoots),
	})
}

// GetInstances returns auto-detected Minecraft instances.
func (a *App) GetInstances() []instance.Instance {
	settings, err := a.db.GetSettings()
	if err != nil {
		return instance.DetectInstances(nil)
	}
	return a.detectInstancesForSettings(*settings)
}

// GetInstancesForSettings returns auto-detected instances using the provided unsaved settings.
func (a *App) GetInstancesForSettings(settings db.Settings) []instance.Instance {
	return a.detectInstancesForSettings(settings)
}

// GetAllConfigFiles returns all config files found in the config directory.
func (a *App) GetAllConfigFiles() ([]scanner.ConfigFile, error) {
	if a.configDir == "" {
		return nil, nil
	}
	return scanner.ScanConfigFolder(a.configDir)
}

// GetAbsoluteConfigPath resolves a config mapping path to its absolute filesystem location.
func (a *App) GetAbsoluteConfigPath(configPath string) (string, error) {
	absPath := configPath
	if a.configDir != "" && !filepath.IsAbs(configPath) {
		absPath = filepath.Join(a.configDir, configPath)
	}

	absPath, err := filepath.Abs(absPath)
	if err != nil {
		return "", err
	}
	configAbs, _ := filepath.Abs(a.configDir)
	if !strings.HasPrefix(strings.ToLower(absPath), strings.ToLower(configAbs)) {
		return "", fmt.Errorf("path is outside config directory")
	}

	return absPath, nil
}

// BrowseForFolder opens a native folder picker dialog.
func (a *App) BrowseForFolder() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Minecraft Instance Folder",
	})
}

// ResetDatabase clears all data and triggers a fresh scan.
func (a *App) ResetDatabase() error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := a.db.ResetAll(); err != nil {
		return err
	}
	runtime.EventsEmit(a.ctx, "mods:updated")
	go a.performFullScan()
	return nil
}

// GetInstanceName returns the display name of the current modpack folder.
func (a *App) GetInstanceName() string {
	if a.modsDir == "" {
		return ""
	}
	return filepath.Base(filepath.Dir(a.modsDir))
}

// ReverseDep represents a mod that depends on a given mod.
type ReverseDep struct {
	ModID   string `json:"modID"`
	Name    string `json:"name"`
	IconURL string `json:"iconURL"`
	Type    string `json:"type"`
	Via     string `json:"via,omitempty"`
}

// GetReverseDependencies returns all mods that depend on the given modID.
func (a *App) GetReverseDependencies(modID string) ([]ReverseDep, error) {
	mod, err := a.db.GetModByID(modID)
	if err != nil || mod == nil {
		return nil, err
	}

	deps, err := a.db.GetAllDependencies()
	if err != nil {
		return nil, err
	}
	mods, err := a.db.GetAllMods()
	if err != nil {
		return nil, err
	}

	modNames := make(map[string]string)
	modIcons := make(map[string]string)
	for _, m := range mods {
		modNames[m.ID] = m.Name
		modIcons[m.ID] = m.IconURL
	}

	var result []ReverseDep
	seen := make(map[string]bool)
	for _, dep := range deps {
		if dep.ModID == mod.ID || seen[dep.ModID] {
			continue
		}
		resolvedID, satisfied := resolver.ResolveDependencyTarget(mods, dep.ModID, dep.DepModID)
		if !satisfied || resolvedID != mod.ID {
			continue
		}
		seen[dep.ModID] = true
		name := modNames[dep.ModID]
		if name == "" {
			name = dep.ModID
		}
		via := ""
		if dep.DepModID != mod.ID && dep.DepModID != mod.ModrinthID {
			if mod.CurseForgeID == 0 || dep.DepModID != fmt.Sprintf("%d", mod.CurseForgeID) {
				via = dep.DepModID
			}
		}
		result = append(result, ReverseDep{
			ModID:   dep.ModID,
			Name:    name,
			IconURL: modIcons[dep.ModID],
			Type:    dep.Type,
			Via:     via,
		})
	}

	return result, nil
}

func splitProvidedModules(raw string) []string {
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

func buildMixinDetails(mixins []db.Mixin, mods []db.Mod) []MixinDetail {
	modNames := make(map[string]string, len(mods))
	for _, m := range mods {
		modNames[m.ID] = m.Name
	}

	details := make([]MixinDetail, 0, len(mixins))
	for _, m := range mixins {
		targetModName := ""
		if m.TargetModID == "minecraft" {
			targetModName = "Minecraft"
		} else if name, ok := modNames[m.TargetModID]; ok {
			targetModName = name
		}
		details = append(details, MixinDetail{
			MixinClass:    m.MixinClass,
			TargetClass:   m.TargetClass,
			TargetModID:   m.TargetModID,
			TargetModName: targetModName,
			TargetMembers: m.TargetMembers,
		})
	}
	return details
}

func buildIncomingMixins(mixins []db.Mixin, mods []db.Mod) []IncomingMixin {
	modMap := make(map[string]db.Mod, len(mods))
	for _, m := range mods {
		modMap[m.ID] = m
	}

	incoming := make([]IncomingMixin, 0, len(mixins))
	for _, m := range mixins {
		ownerName := m.OwnerModID
		ownerIcon := ""
		if mod, ok := modMap[m.OwnerModID]; ok {
			ownerName = mod.Name
			ownerIcon = mod.IconURL
		}
		incoming = append(incoming, IncomingMixin{
			OwnerModID:    m.OwnerModID,
			OwnerModName:  ownerName,
			OwnerIconURL:  ownerIcon,
			MixinClass:    m.MixinClass,
			TargetClass:   m.TargetClass,
			TargetMembers: m.TargetMembers,
		})
	}
	return incoming
}
