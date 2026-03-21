package api

// --- CurseForge types ---

type CFingerprintResponse struct {
	Data struct {
		ExactMatches []CFingerprintMatch `json:"exactMatches"`
	} `json:"data"`
}

type CFingerprintMatch struct {
	ID   int    `json:"id"`
	File CFFile `json:"file"`
}

type CFFile struct {
	ID              int    `json:"id"`
	ModID           int    `json:"modId"`
	FileName        string `json:"fileName"`
	DisplayName     string `json:"displayName"`
	FileFingerprint uint32 `json:"fileFingerprint"`
}

type CFModResponse struct {
	Data CFMod `json:"data"`
}

type CFMod struct {
	ID         int          `json:"id"`
	Name       string       `json:"name"`
	Slug       string       `json:"slug"`
	Summary    string       `json:"summary"`
	Links      CFLinks      `json:"links"`
	Logo       *CFLogo      `json:"logo"`
	Categories []CFCategory `json:"categories"`
}

type CFLinks struct {
	WebsiteURL string `json:"websiteUrl"`
}

type CFLogo struct {
	URL string `json:"url"`
}

type CFCategory struct {
	Name string `json:"name"`
}

type CFFileDepsResponse struct {
	Data []CFFileDep `json:"data"`
}

type CFFileDep struct {
	ModID        int `json:"modId"`
	RelationType int `json:"relationType"` // 1=embedded, 2=optional, 3=required, 4=tool, 5=incompatible, 6=include
}

type CFModFilesResponse struct {
	Data []CFFileDetail `json:"data"`
}

type CFFileDetail struct {
	ID           int         `json:"id"`
	Dependencies []CFFileDep `json:"dependencies"`
}

// --- Modrinth types ---

type MRVersionFromHash struct {
	ProjectID    string   `json:"project_id"`
	ID           string   `json:"id"`
	Loaders      []string `json:"loaders"`
	Files        []MRFile `json:"files"`
	Dependencies []MRDep  `json:"dependencies"`
}

type MRFile struct {
	URL      string            `json:"url"`
	Filename string            `json:"filename"`
	Hashes   map[string]string `json:"hashes"`
}

type MRDep struct {
	ProjectID      *string `json:"project_id"`
	DependencyType string  `json:"dependency_type"` // required, optional, incompatible, embedded
}

type MRProject struct {
	ID          string   `json:"id"`
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Body        string   `json:"body"`
	IconURL     string   `json:"icon_url"`
	Categories  []string `json:"categories"`
	ProjectType string   `json:"project_type"`
}

// OnlineModInfo aggregates data from online sources.
type OnlineModInfo struct {
	CurseForgeID  int
	ModrinthID    string
	CurseForgeURL string
	ModrinthURL   string
	IconURL       string
	Description   string
	Dependencies  []OnlineDep
	Categories    []string
	ProjectType   string
	Loaders       []string
}

type OnlineDep struct {
	ModID  string
	Name   string
	Type   string // required, optional, embedded
	Source string // curseforge, modrinth
}
