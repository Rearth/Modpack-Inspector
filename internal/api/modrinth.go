package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"modpacktool/internal/db"
)

const modrinthBaseURL = "https://api.modrinth.com"

// ModrinthClient wraps the Modrinth API.
type ModrinthClient struct {
	apiKey     string
	httpClient *http.Client
	cache      *db.Database
	cacheTTL   time.Duration
}

func NewModrinthClient(apiKey string, cache *db.Database, ttl time.Duration) *ModrinthClient {
	return &ModrinthClient{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		cache:      cache,
		cacheTTL:   ttl,
	}
}

func (c *ModrinthClient) doRequest(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", "ModpackTool/1.0.0 (https://github.com/modpacktool)")
	if c.apiKey != "" {
		req.Header.Set("Authorization", c.apiKey)
	}
	return c.httpClient.Do(req)
}

// MatchByHash looks up a mod version by SHA-1 hash of the JAR file.
func (c *ModrinthClient) MatchByHash(sha1Hash string) (*MRVersionFromHash, error) {
	cacheKey := "mr:hash:" + sha1Hash
	if cached, ok, _ := c.cache.GetCachedAPI(cacheKey); ok {
		var v MRVersionFromHash
		if json.Unmarshal([]byte(cached), &v) == nil {
			return &v, nil
		}
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v2/version_file/%s", modrinthBaseURL, sha1Hash), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("modrinth hash lookup: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("modrinth API error %d: %s", resp.StatusCode, string(body))
	}

	var v MRVersionFromHash
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return nil, err
	}

	if data, err := json.Marshal(v); err == nil {
		c.cache.SetCachedAPI(cacheKey, string(data), c.cacheTTL)
	}
	return &v, nil
}

// GetProject fetches full project details from Modrinth.
func (c *ModrinthClient) GetProject(projectID string) (*MRProject, error) {
	cacheKey := "mr:project:" + projectID
	if cached, ok, _ := c.cache.GetCachedAPI(cacheKey); ok {
		var p MRProject
		if json.Unmarshal([]byte(cached), &p) == nil {
			return &p, nil
		}
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v2/project/%s", modrinthBaseURL, projectID), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("modrinth project %s: status %d", projectID, resp.StatusCode)
	}

	var p MRProject
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, err
	}

	if data, err := json.Marshal(p); err == nil {
		c.cache.SetCachedAPI(cacheKey, string(data), c.cacheTTL)
	}
	return &p, nil
}

// EnrichMod fetches online info from Modrinth for a single mod.
func (c *ModrinthClient) EnrichMod(sha1Hash string) (*OnlineModInfo, error) {
	version, err := c.MatchByHash(sha1Hash)
	if err != nil || version == nil {
		return nil, err
	}

	project, err := c.GetProject(version.ProjectID)
	if err != nil || project == nil {
		return nil, err
	}

	info := &OnlineModInfo{
		ModrinthID:  project.ID,
		ModrinthURL: fmt.Sprintf("https://modrinth.com/%s/%s", modrinthProjectType(project.ProjectType), project.Slug),
		Description: project.Description,
		IconURL:     project.IconURL,
		Categories:  project.Categories,
		ProjectType: project.ProjectType,
		Loaders:     version.Loaders,
	}

	// Add dependencies from the version
	for _, dep := range version.Dependencies {
		if dep.ProjectID == nil {
			continue
		}
		depType := dep.DependencyType
		if depType == "incompatible" {
			continue
		}
		info.Dependencies = append(info.Dependencies, OnlineDep{
			ModID:  *dep.ProjectID,
			Name:   *dep.ProjectID, // will be resolved later
			Type:   depType,
			Source: "modrinth",
		})
	}

	return info, nil
}

func modrinthProjectType(pt string) string {
	if pt != "" {
		return pt
	}
	return "mod"
}

// BatchMatchHashes looks up multiple SHA-1 hashes in one API call
// (POST /v2/version_files) and caches each result individually.
func (c *ModrinthClient) BatchMatchHashes(sha1Hashes []string) map[string]*MRVersionFromHash {
	if len(sha1Hashes) == 0 {
		return nil
	}

	result := make(map[string]*MRVersionFromHash)
	uncached := make([]string, 0, len(sha1Hashes))

	for _, h := range sha1Hashes {
		cacheKey := "mr:hash:" + h
		if cached, ok, _ := c.cache.GetCachedAPI(cacheKey); ok {
			var v MRVersionFromHash
			if json.Unmarshal([]byte(cached), &v) == nil {
				result[h] = &v
				continue
			}
		}
		uncached = append(uncached, h)
	}

	if len(uncached) == 0 {
		return result
	}

	hashesJSON, err := json.Marshal(uncached)
	if err != nil {
		return result
	}
	body := fmt.Sprintf(`{"hashes":%s,"algorithm":"sha1"}`, string(hashesJSON))

	req, err := http.NewRequest("POST", modrinthBaseURL+"/v2/version_files", strings.NewReader(body))
	if err != nil {
		return result
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(req)
	if err != nil {
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return result
	}

	var versions map[string]MRVersionFromHash
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return result
	}

	for hash, v := range versions {
		vCopy := v
		result[hash] = &vCopy
		if data, err := json.Marshal(vCopy); err == nil {
			c.cache.SetCachedAPI("mr:hash:"+hash, string(data), c.cacheTTL)
		}
	}

	return result
}

// BatchGetProjects fetches multiple project details in one API call
// (GET /v2/projects?ids=[...]) and caches each result individually.
func (c *ModrinthClient) BatchGetProjects(projectIDs []string) map[string]*MRProject {
	if len(projectIDs) == 0 {
		return nil
	}

	result := make(map[string]*MRProject)
	uncached := make([]string, 0, len(projectIDs))

	for _, id := range projectIDs {
		cacheKey := "mr:project:" + id
		if cached, ok, _ := c.cache.GetCachedAPI(cacheKey); ok {
			var p MRProject
			if json.Unmarshal([]byte(cached), &p) == nil {
				result[id] = &p
				continue
			}
		}
		uncached = append(uncached, id)
	}

	if len(uncached) == 0 {
		return result
	}

	idsJSON, err := json.Marshal(uncached)
	if err != nil {
		return result
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v2/projects?ids=%s", modrinthBaseURL, string(idsJSON)), nil)
	if err != nil {
		return result
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return result
	}

	var projects []MRProject
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return result
	}

	for i := range projects {
		p := projects[i]
		result[p.ID] = &p
		if data, err := json.Marshal(p); err == nil {
			c.cache.SetCachedAPI("mr:project:"+p.ID, string(data), c.cacheTTL)
		}
	}

	return result
}
