package db

import (
	"database/sql"
	"fmt"
	"time"
)

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// --- Mods ---

func (d *Database) UpsertMod(m *Mod) error {
	_, err := d.db.Exec(`
		INSERT INTO mods (id, name, version, description, authors, mod_loader,
			jar_file_name, jar_sha1, jar_sha512, fingerprint, homepage_url,
			curseforge_id, modrinth_id, curseforge_url, modrinth_url, icon_url,
			provided_ids,
			embedding, is_library, last_scanned, last_api_check, online_desc,
			loaders, categories, project_type)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name, version=excluded.version, description=excluded.description,
			authors=excluded.authors, mod_loader=excluded.mod_loader,
			jar_file_name=excluded.jar_file_name, jar_sha1=excluded.jar_sha1,
			jar_sha512=excluded.jar_sha512, fingerprint=excluded.fingerprint,
			homepage_url=excluded.homepage_url, curseforge_id=excluded.curseforge_id,
			modrinth_id=excluded.modrinth_id, curseforge_url=excluded.curseforge_url,
			modrinth_url=excluded.modrinth_url, icon_url=excluded.icon_url,
			provided_ids=excluded.provided_ids,
			embedding=excluded.embedding, is_library=excluded.is_library,
			last_scanned=excluded.last_scanned, last_api_check=excluded.last_api_check,
			online_desc=excluded.online_desc,
			loaders=excluded.loaders, categories=excluded.categories,
			project_type=excluded.project_type`,
		m.ID, m.Name, m.Version, m.Description, m.Authors, m.ModLoader,
		m.JarFileName, m.JarSHA1, m.JarSHA512, m.Fingerprint, m.HomepageURL,
		m.CurseForgeID, m.ModrinthID, m.CurseForgeURL, m.ModrinthURL, m.IconURL,
		m.ProvidedIDs,
		m.Embedding, boolToInt(m.IsLibrary), m.LastScanned.Format(time.RFC3339),
		m.LastAPICheck.Format(time.RFC3339), m.OnlineDesc,
		m.Loaders, m.Categories, m.ProjectType)
	return err
}

func (d *Database) GetAllMods() ([]Mod, error) {
	rows, err := d.db.Query(`
		SELECT id, name, version, description, authors, mod_loader, jar_file_name,
			jar_sha1, jar_sha512, fingerprint, homepage_url, curseforge_id, modrinth_id,
			curseforge_url, modrinth_url, icon_url, provided_ids, embedding, is_library,
			last_scanned, last_api_check, online_desc, loaders, categories, project_type
		FROM mods ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMods(rows)
}

func (d *Database) GetModByID(id string) (*Mod, error) {
	row := d.db.QueryRow(`
		SELECT id, name, version, description, authors, mod_loader, jar_file_name,
			jar_sha1, jar_sha512, fingerprint, homepage_url, curseforge_id, modrinth_id,
			curseforge_url, modrinth_url, icon_url, provided_ids, embedding, is_library,
			last_scanned, last_api_check, online_desc, loaders, categories, project_type
		FROM mods WHERE id = ?`, id)

	m, err := scanMod(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return m, err
}

func (d *Database) DeleteMod(id string) error {
	_, err := d.db.Exec("DELETE FROM mods WHERE id = ?", id)
	return err
}

func (d *Database) DeleteModByFileName(fileName string) error {
	_, err := d.db.Exec("DELETE FROM mods WHERE jar_file_name = ?", fileName)
	return err
}

// DeleteStaleMods removes mods whose jar_file_name is not in the given set.
// Also cleans up their dependencies and mixins.
func (d *Database) DeleteStaleMods(activeFileNames map[string]bool) (int, error) {
	rows, err := d.db.Query("SELECT id, jar_file_name FROM mods")
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var staleIDs []string
	for rows.Next() {
		var id, fn string
		if err := rows.Scan(&id, &fn); err != nil {
			continue
		}
		if !activeFileNames[fn] {
			staleIDs = append(staleIDs, id)
		}
	}

	for _, id := range staleIDs {
		d.db.Exec("DELETE FROM dependencies WHERE mod_id = ?", id)
		d.db.Exec("DELETE FROM mixins WHERE owner_mod_id = ?", id)
		d.db.Exec("DELETE FROM config_mappings WHERE mod_id = ?", id)
		d.db.Exec("DELETE FROM mods WHERE id = ?", id)
	}
	return len(staleIDs), nil
}

func (d *Database) GetModFileNames() (map[string]string, error) {
	rows, err := d.db.Query("SELECT jar_file_name, id FROM mods")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var fileName, id string
		if err := rows.Scan(&fileName, &id); err != nil {
			return nil, err
		}
		result[fileName] = id
	}
	return result, nil
}

func scanMods(rows *sql.Rows) ([]Mod, error) {
	var mods []Mod
	for rows.Next() {
		var m Mod
		var isLib int
		var lastScanned, lastAPI string
		err := rows.Scan(&m.ID, &m.Name, &m.Version, &m.Description, &m.Authors,
			&m.ModLoader, &m.JarFileName, &m.JarSHA1, &m.JarSHA512, &m.Fingerprint,
			&m.HomepageURL, &m.CurseForgeID, &m.ModrinthID, &m.CurseForgeURL,
			&m.ModrinthURL, &m.IconURL, &m.ProvidedIDs, &m.Embedding, &isLib, &lastScanned,
			&lastAPI, &m.OnlineDesc, &m.Loaders, &m.Categories, &m.ProjectType)
		if err != nil {
			return nil, err
		}
		m.IsLibrary = isLib != 0
		m.LastScanned, _ = time.Parse(time.RFC3339, lastScanned)
		m.LastAPICheck, _ = time.Parse(time.RFC3339, lastAPI)
		mods = append(mods, m)
	}
	return mods, rows.Err()
}

func scanMod(row *sql.Row) (*Mod, error) {
	var m Mod
	var isLib int
	var lastScanned, lastAPI string
	err := row.Scan(&m.ID, &m.Name, &m.Version, &m.Description, &m.Authors,
		&m.ModLoader, &m.JarFileName, &m.JarSHA1, &m.JarSHA512, &m.Fingerprint,
		&m.HomepageURL, &m.CurseForgeID, &m.ModrinthID, &m.CurseForgeURL,
		&m.ModrinthURL, &m.IconURL, &m.ProvidedIDs, &m.Embedding, &isLib, &lastScanned,
		&lastAPI, &m.OnlineDesc, &m.Loaders, &m.Categories, &m.ProjectType)
	if err != nil {
		return nil, err
	}
	m.IsLibrary = isLib != 0
	m.LastScanned, _ = time.Parse(time.RFC3339, lastScanned)
	m.LastAPICheck, _ = time.Parse(time.RFC3339, lastAPI)
	return &m, nil
}

// --- Dependencies ---

func (d *Database) UpsertDependency(dep *Dependency) error {
	_, err := d.db.Exec(`
		INSERT INTO dependencies (mod_id, dep_mod_id, dep_name, type, satisfied, source)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(mod_id, dep_mod_id, source) DO UPDATE SET
			dep_name=excluded.dep_name, type=excluded.type,
			satisfied=excluded.satisfied`,
		dep.ModID, dep.DepModID, dep.DepName, dep.Type, boolToInt(dep.Satisfied), dep.Source)
	return err
}

func (d *Database) GetDependenciesByModID(modID string) ([]Dependency, error) {
	rows, err := d.db.Query(`
		SELECT mod_id, dep_mod_id, dep_name, type, satisfied, source
		FROM dependencies WHERE mod_id = ?`, modID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDeps(rows)
}

func (d *Database) GetAllDependencies() ([]Dependency, error) {
	rows, err := d.db.Query(`SELECT mod_id, dep_mod_id, dep_name, type, satisfied, source FROM dependencies`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDeps(rows)
}

func (d *Database) DeleteDependenciesByModID(modID string) error {
	_, err := d.db.Exec("DELETE FROM dependencies WHERE mod_id = ?", modID)
	return err
}

func scanDeps(rows *sql.Rows) ([]Dependency, error) {
	var deps []Dependency
	for rows.Next() {
		var dep Dependency
		var satisfied int
		err := rows.Scan(&dep.ModID, &dep.DepModID, &dep.DepName, &dep.Type, &satisfied, &dep.Source)
		if err != nil {
			return nil, err
		}
		dep.Satisfied = satisfied != 0
		deps = append(deps, dep)
	}
	return deps, rows.Err()
}

// --- Config Mappings ---

func (d *Database) UpsertConfigMapping(cm *ConfigMapping) error {
	_, err := d.db.Exec(`
		INSERT INTO config_mappings (config_path, mod_id, confidence, is_manual)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(config_path, mod_id) DO UPDATE SET
			confidence=excluded.confidence, is_manual=excluded.is_manual`,
		cm.ConfigPath, cm.ModID, cm.Confidence, boolToInt(cm.IsManual))
	return err
}

func (d *Database) GetConfigMappingsByModID(modID string) ([]ConfigMapping, error) {
	rows, err := d.db.Query(`
		SELECT config_path, mod_id, confidence, is_manual
		FROM config_mappings WHERE mod_id = ? ORDER BY confidence DESC`, modID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanConfigMappings(rows)
}

func (d *Database) GetAllConfigMappings() ([]ConfigMapping, error) {
	rows, err := d.db.Query(`SELECT config_path, mod_id, confidence, is_manual FROM config_mappings ORDER BY confidence DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanConfigMappings(rows)
}

func (d *Database) DeleteNonManualMappings() error {
	_, err := d.db.Exec("DELETE FROM config_mappings WHERE is_manual = 0")
	return err
}

func (d *Database) DeleteConfigMapping(configPath, modID string) error {
	_, err := d.db.Exec("DELETE FROM config_mappings WHERE config_path = ? AND mod_id = ?", configPath, modID)
	return err
}

// ClearScanData removes instance-scoped scan results while preserving settings and API cache.
func (d *Database) ClearScanData() error {
	tables := []string{"mods", "dependencies", "config_mappings", "mixins"}
	for _, t := range tables {
		if _, err := d.db.Exec("DELETE FROM " + t); err != nil {
			return fmt.Errorf("clearing %s: %w", t, err)
		}
	}
	return nil
}

// ResetAll drops all data from mods, dependencies, config_mappings, and api_cache tables.
func (d *Database) ResetAll() error {
	tables := []string{"mods", "dependencies", "config_mappings", "api_cache", "mixins"}
	for _, t := range tables {
		if _, err := d.db.Exec("DELETE FROM " + t); err != nil {
			return fmt.Errorf("clearing %s: %w", t, err)
		}
	}
	return nil
}

func scanConfigMappings(rows *sql.Rows) ([]ConfigMapping, error) {
	var mappings []ConfigMapping
	for rows.Next() {
		var cm ConfigMapping
		var isManual int
		err := rows.Scan(&cm.ConfigPath, &cm.ModID, &cm.Confidence, &isManual)
		if err != nil {
			return nil, err
		}
		cm.IsManual = isManual != 0
		mappings = append(mappings, cm)
	}
	return mappings, rows.Err()
}

// --- API Cache ---

func (d *Database) GetCachedAPI(key string) (string, bool, error) {
	var value, expiresAt string
	err := d.db.QueryRow("SELECT value, expires_at FROM api_cache WHERE key = ?", key).
		Scan(&value, &expiresAt)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	t, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil || time.Now().After(t) {
		d.db.Exec("DELETE FROM api_cache WHERE key = ?", key)
		return "", false, nil
	}
	return value, true, nil
}

func (d *Database) SetCachedAPI(key, value string, ttl time.Duration) error {
	expiresAt := time.Now().Add(ttl).Format(time.RFC3339)
	_, err := d.db.Exec(`
		INSERT INTO api_cache (key, value, expires_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value, expires_at=excluded.expires_at`,
		key, value, expiresAt)
	return err
}

func (d *Database) ClearExpiredCache() error {
	_, err := d.db.Exec("DELETE FROM api_cache WHERE expires_at < ?", time.Now().Format(time.RFC3339))
	return err
}

// --- Settings ---

func (d *Database) GetSetting(key string) (string, error) {
	var value string
	err := d.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (d *Database) SetSetting(key, value string) error {
	_, err := d.db.Exec(`
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

func (d *Database) GetSettings() (*Settings, error) {
	s := &Settings{CacheTTLHours: 24}
	if v, err := d.GetSetting("instance_path"); err == nil {
		s.InstancePath = v
	}
	if v, err := d.GetSetting("curseforge_api_key"); err == nil {
		s.CurseForgeAPIKey = v
	}
	if v, err := d.GetSetting("modrinth_api_key"); err == nil {
		s.ModrinthAPIKey = v
	}
	if v, err := d.GetSetting("custom_modrinth_root"); err == nil {
		s.CustomModrinthRoot = v
	}
	if v, err := d.GetSetting("custom_curseforge_root"); err == nil {
		s.CustomCurseForgeRoot = v
	}
	if v, err := d.GetSetting("custom_ftb_root"); err == nil {
		s.CustomFTBRoot = v
	}
	if v, err := d.GetSetting("custom_launcher_roots"); err == nil {
		s.CustomLauncherRoots = v
	}
	return s, nil
}

func (d *Database) SaveSettings(s *Settings) error {
	if err := d.SetSetting("instance_path", s.InstancePath); err != nil {
		return err
	}
	if err := d.SetSetting("curseforge_api_key", s.CurseForgeAPIKey); err != nil {
		return err
	}
	if err := d.SetSetting("modrinth_api_key", s.ModrinthAPIKey); err != nil {
		return err
	}
	if err := d.SetSetting("custom_modrinth_root", s.CustomModrinthRoot); err != nil {
		return err
	}
	if err := d.SetSetting("custom_curseforge_root", s.CustomCurseForgeRoot); err != nil {
		return err
	}
	if err := d.SetSetting("custom_ftb_root", s.CustomFTBRoot); err != nil {
		return err
	}
	return d.SetSetting("custom_launcher_roots", s.CustomLauncherRoots)
}

// --- Mixins ---

func (d *Database) UpsertMixin(m *Mixin) error {
	_, err := d.db.Exec(`
		INSERT INTO mixins (owner_mod_id, mixin_class, target_class, target_mod_id, target_members)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(owner_mod_id, mixin_class, target_class) DO UPDATE SET
			target_mod_id=excluded.target_mod_id, target_members=excluded.target_members`,
		m.OwnerModID, m.MixinClass, m.TargetClass, m.TargetModID, m.TargetMembers)
	return err
}

func (d *Database) GetMixinsByModID(ownerModID string) ([]Mixin, error) {
	rows, err := d.db.Query(`
		SELECT owner_mod_id, mixin_class, target_class, target_mod_id, target_members
		FROM mixins WHERE owner_mod_id = ? ORDER BY target_mod_id, mixin_class`, ownerModID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMixins(rows)
}

func (d *Database) GetMixinsTargetingMod(targetModID string) ([]Mixin, error) {
	rows, err := d.db.Query(`
		SELECT owner_mod_id, mixin_class, target_class, target_mod_id, target_members
		FROM mixins WHERE target_mod_id = ? ORDER BY owner_mod_id, mixin_class`, targetModID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMixins(rows)
}

func (d *Database) DeleteMixinsByModID(ownerModID string) error {
	_, err := d.db.Exec("DELETE FROM mixins WHERE owner_mod_id = ?", ownerModID)
	return err
}

func scanMixins(rows *sql.Rows) ([]Mixin, error) {
	var mixins []Mixin
	for rows.Next() {
		var m Mixin
		err := rows.Scan(&m.OwnerModID, &m.MixinClass, &m.TargetClass, &m.TargetModID, &m.TargetMembers)
		if err != nil {
			return nil, err
		}
		mixins = append(mixins, m)
	}
	return mixins, rows.Err()
}
