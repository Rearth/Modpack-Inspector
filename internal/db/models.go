package db

import "time"

// Mod represents a parsed Minecraft mod with all metadata.
type Mod struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Version       string    `json:"version"`
	Description   string    `json:"description"`
	Authors       string    `json:"authors"`
	ModLoader     string    `json:"modLoader"`
	JarFileName   string    `json:"jarFileName"`
	JarSHA1       string    `json:"jarSHA1"`
	JarSHA512     string    `json:"jarSHA512"`
	Fingerprint   uint32    `json:"fingerprint"`
	HomepageURL   string    `json:"homepageURL"`
	CurseForgeID  int       `json:"curseForgeID"`
	ModrinthID    string    `json:"modrinthID"`
	CurseForgeURL string    `json:"curseForgeURL"`
	ModrinthURL   string    `json:"modrinthURL"`
	IconURL       string    `json:"iconURL"`
	ProvidedIDs   string    `json:"providedIDs"`
	Embedding     []byte    `json:"-"`
	IsLibrary     bool      `json:"isLibrary"`
	LastScanned   time.Time `json:"lastScanned"`
	LastAPICheck  time.Time `json:"lastAPICheck"`
	OnlineDesc    string    `json:"onlineDesc"`
	Loaders       string    `json:"loaders"`
	Categories    string    `json:"categories"`
	ProjectType   string    `json:"projectType"`
}

// Dependency represents a relationship between two mods.
type Dependency struct {
	ModID     string `json:"modID"`
	DepModID  string `json:"depModID"`
	DepName   string `json:"depName"`
	Type      string `json:"type"` // required, optional, embedded
	Satisfied bool   `json:"satisfied"`
	Source    string `json:"source"` // manifest, curseforge, modrinth
}

// ConfigMapping links a config file to a mod with confidence scoring.
type ConfigMapping struct {
	ConfigPath string  `json:"configPath"`
	ModID      string  `json:"modID"`
	Confidence float64 `json:"confidence"`
	IsManual   bool    `json:"isManual"`
}

// Mixin represents a mixin class declared by a mod and its injection target.
type Mixin struct {
	OwnerModID    string `json:"ownerModID"`
	MixinClass    string `json:"mixinClass"`
	TargetClass   string `json:"targetClass"`
	TargetModID   string `json:"targetModID"`
	TargetMembers string `json:"targetMembers"` // comma-separated method/field names
}

// Settings stores user-configurable application settings.
type Settings struct {
	InstancePath         string `json:"instancePath"`
	CurseForgeAPIKey     string `json:"curseForgeAPIKey"`
	ModrinthAPIKey       string `json:"modrinthAPIKey"`
	CacheTTLHours        int    `json:"cacheTTLHours"`
	AppScale             int    `json:"appScale"`
	CustomModrinthRoot   string `json:"customModrinthRoot"`
	CustomCurseForgeRoot string `json:"customCurseForgeRoot"`
	CustomFTBRoot        string `json:"customFTBRoot"`
	CustomLauncherRoots  string `json:"customLauncherRoots"`
}
