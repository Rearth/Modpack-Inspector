package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"modpacktool/internal/db"
)

const curseForgeBaseURL = "https://api.curseforge.com"

// CurseForgeClient wraps the CurseForge API.
type CurseForgeClient struct {
	apiKey     string
	httpClient *http.Client
	cache      *db.Database
	cacheTTL   time.Duration
}

func NewCurseForgeClient(apiKey string, cache *db.Database, ttl time.Duration) *CurseForgeClient {
	return &CurseForgeClient{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		cache:      cache,
		cacheTTL:   ttl,
	}
}

// MatchByFingerprint finds a mod on CurseForge by its MurmurHash2 fingerprint.
func (c *CurseForgeClient) MatchByFingerprint(fingerprint uint32) (*CFingerprintMatch, error) {
	if c.apiKey == "" {
		return nil, nil
	}

	cacheKey := fmt.Sprintf("cf:fp:%d", fingerprint)
	if cached, ok, _ := c.cache.GetCachedAPI(cacheKey); ok {
		var match CFingerprintMatch
		if json.Unmarshal([]byte(cached), &match) == nil {
			return &match, nil
		}
	}

	body := fmt.Sprintf(`{"fingerprints":[%d]}`, fingerprint)
	req, err := http.NewRequest("POST", curseForgeBaseURL+"/v1/fingerprints", strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("curseforge fingerprint request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("curseforge API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result CFingerprintResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Data.ExactMatches) == 0 {
		return nil, nil
	}

	match := &result.Data.ExactMatches[0]
	if data, err := json.Marshal(match); err == nil {
		c.cache.SetCachedAPI(cacheKey, string(data), c.cacheTTL)
	}
	return match, nil
}

// GetMod fetches full mod details from CurseForge.
func (c *CurseForgeClient) GetMod(modID int) (*CFMod, error) {
	if c.apiKey == "" {
		return nil, nil
	}

	cacheKey := fmt.Sprintf("cf:mod:%d", modID)
	if cached, ok, _ := c.cache.GetCachedAPI(cacheKey); ok {
		var mod CFMod
		if json.Unmarshal([]byte(cached), &mod) == nil {
			return &mod, nil
		}
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/mods/%d", curseForgeBaseURL, modID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("curseforge mod %d: status %d", modID, resp.StatusCode)
	}

	var result CFModResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if data, err := json.Marshal(result.Data); err == nil {
		c.cache.SetCachedAPI(cacheKey, string(data), c.cacheTTL)
	}
	return &result.Data, nil
}

// GetFileDependencies fetches dependencies for a specific file on CurseForge.
func (c *CurseForgeClient) GetFileDependencies(modID, fileID int) ([]CFFileDep, error) {
	if c.apiKey == "" {
		return nil, nil
	}

	cacheKey := fmt.Sprintf("cf:filedeps:%d:%d", modID, fileID)
	if cached, ok, _ := c.cache.GetCachedAPI(cacheKey); ok {
		var deps []CFFileDep
		if json.Unmarshal([]byte(cached), &deps) == nil {
			return deps, nil
		}
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/mods/%d/files/%d", curseForgeBaseURL, modID, fileID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("curseforge file deps: status %d", resp.StatusCode)
	}

	var result struct {
		Data CFFileDetail `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	deps := result.Data.Dependencies
	if data, err := json.Marshal(deps); err == nil {
		c.cache.SetCachedAPI(cacheKey, string(data), c.cacheTTL)
	}
	return deps, nil
}

// EnrichMod fetches online info from CurseForge for a single mod.
func (c *CurseForgeClient) EnrichMod(fingerprint uint32) (*OnlineModInfo, error) {
	match, err := c.MatchByFingerprint(fingerprint)
	if err != nil || match == nil {
		return nil, err
	}

	mod, err := c.GetMod(match.File.ModID)
	if err != nil || mod == nil {
		return nil, err
	}

	info := &OnlineModInfo{
		CurseForgeID:  mod.ID,
		CurseForgeURL: mod.Links.WebsiteURL,
		Description:   mod.Summary,
	}
	if mod.Logo != nil {
		info.IconURL = mod.Logo.URL
	}

	// Extract categories
	for _, cat := range mod.Categories {
		info.Categories = append(info.Categories, cat.Name)
	}

	// Fetch file dependencies
	fileDeps, err := c.GetFileDependencies(match.File.ModID, match.File.ID)
	if err == nil {
		for _, fd := range fileDeps {
			depType := cfRelationToType(fd.RelationType)
			if depType == "" {
				continue
			}
			info.Dependencies = append(info.Dependencies, OnlineDep{
				ModID:  strconv.Itoa(fd.ModID),
				Name:   strconv.Itoa(fd.ModID), // will be resolved later
				Type:   depType,
				Source: "curseforge",
			})
		}
	}

	return info, nil
}

func cfRelationToType(rt int) string {
	switch rt {
	case 1:
		return "embedded"
	case 2:
		return "optional"
	case 3:
		return "required"
	default:
		return ""
	}
}

// BatchMatchFingerprints matches multiple fingerprints in one API call and
// caches each result individually so subsequent EnrichMod calls hit cache.
func (c *CurseForgeClient) BatchMatchFingerprints(fingerprints []uint32) map[uint32]*CFingerprintMatch {
	if c.apiKey == "" || len(fingerprints) == 0 {
		return nil
	}

	result := make(map[uint32]*CFingerprintMatch)
	uncached := make([]uint32, 0, len(fingerprints))

	for _, fp := range fingerprints {
		cacheKey := fmt.Sprintf("cf:fp:%d", fp)
		if cached, ok, _ := c.cache.GetCachedAPI(cacheKey); ok {
			var match CFingerprintMatch
			if json.Unmarshal([]byte(cached), &match) == nil {
				result[fp] = &match
				continue
			}
		}
		uncached = append(uncached, fp)
	}

	if len(uncached) == 0 {
		return result
	}

	fpStrs := make([]string, len(uncached))
	for i, fp := range uncached {
		fpStrs[i] = strconv.FormatUint(uint64(fp), 10)
	}
	body := fmt.Sprintf(`{"fingerprints":[%s]}`, strings.Join(fpStrs, ","))

	req, err := http.NewRequest("POST", curseForgeBaseURL+"/v1/fingerprints", strings.NewReader(body))
	if err != nil {
		return result
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return result
	}

	var fpResp CFingerprintResponse
	if err := json.NewDecoder(resp.Body).Decode(&fpResp); err != nil {
		return result
	}

	for i := range fpResp.Data.ExactMatches {
		match := fpResp.Data.ExactMatches[i]
		fp := match.File.FileFingerprint
		if fp == 0 {
			continue
		}
		result[fp] = &match
		if data, err := json.Marshal(match); err == nil {
			c.cache.SetCachedAPI(fmt.Sprintf("cf:fp:%d", fp), string(data), c.cacheTTL)
		}
	}

	return result
}

// BatchGetMods fetches multiple mod details in one API call (POST /v1/mods)
// and caches each result individually.
func (c *CurseForgeClient) BatchGetMods(modIDs []int) map[int]*CFMod {
	if c.apiKey == "" || len(modIDs) == 0 {
		return nil
	}

	result := make(map[int]*CFMod)
	uncached := make([]int, 0, len(modIDs))

	for _, id := range modIDs {
		cacheKey := fmt.Sprintf("cf:mod:%d", id)
		if cached, ok, _ := c.cache.GetCachedAPI(cacheKey); ok {
			var mod CFMod
			if json.Unmarshal([]byte(cached), &mod) == nil {
				result[id] = &mod
				continue
			}
		}
		uncached = append(uncached, id)
	}

	if len(uncached) == 0 {
		return result
	}

	idStrs := make([]string, len(uncached))
	for i, id := range uncached {
		idStrs[i] = strconv.Itoa(id)
	}
	body := fmt.Sprintf(`{"modIds":[%s]}`, strings.Join(idStrs, ","))

	req, err := http.NewRequest("POST", curseForgeBaseURL+"/v1/mods", strings.NewReader(body))
	if err != nil {
		return result
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return result
	}

	var modsResp struct {
		Data []CFMod `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&modsResp); err != nil {
		return result
	}

	for i := range modsResp.Data {
		mod := modsResp.Data[i]
		result[mod.ID] = &mod
		if data, err := json.Marshal(mod); err == nil {
			c.cache.SetCachedAPI(fmt.Sprintf("cf:mod:%d", mod.ID), string(data), c.cacheTTL)
		}
	}

	return result
}
