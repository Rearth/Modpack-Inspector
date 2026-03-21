import { useState, useCallback, useEffect, useMemo } from 'react';
import { ScanNow, GetSettings, GetInstanceName } from '../wailsjs/go/main/App';
import { Sidebar } from './components/Sidebar';
import { SearchBar } from './components/SearchBar';
import { ModList } from './components/ModList';
import { ModDetail } from './components/ModDetail';
import { ModGraph } from './components/ModGraph';
import { ConfigEditor } from './components/ConfigEditor';
import { ConfigPicker } from './components/ConfigPicker';
import { SettingsPanel } from './components/SettingsPanel';
import { SetupWizard } from './components/SetupWizard';
import { TitleBar } from './components/TitleBar';
import { useMods, useSearch } from './hooks/useMods';
import { useConfig } from './hooks/useConfig';
import type { View, Mod } from './lib/types';

export default function App() {
  const [view, setView] = useState<View>('mods');
  const [selectedModId, setSelectedModId] = useState<string | null>(null);
  const [showConfigPicker, setShowConfigPicker] = useState(false);
  const [showSetup, setShowSetup] = useState(false);
  const [setupChecked, setSetupChecked] = useState(false);
  const [instanceName, setInstanceName] = useState('');
  const [categoryFilter, setCategoryFilter] = useState('');
  const [showDetectedLibrariesOnly, setShowDetectedLibrariesOnly] = useState(false);
  const [showUnusedOnly, setShowUnusedOnly] = useState(false);

  const { mods, loading, scanStatus, scanProgress, unusedLibraries, refresh } = useMods();
  const { query, setQuery, results, searching } = useSearch();
  const config = useConfig();

  // Extract unique categories from all mods
  const allCategories = useMemo(() => {
    const cats = new Set<string>();
    mods.forEach(m => {
      if (m.categories) {
        m.categories.split(',').forEach(c => {
          const trimmed = c.trim();
          if (trimmed) cats.add(trimmed);
        });
      }
    });
    return Array.from(cats).sort();
  }, [mods]);

  // Apply search + filters
  const displayMods = useMemo(() => {
    let list: Mod[] = query
      ? results.map(r => mods.find(m => m.id === r.mod.id) ?? r.mod)
      : mods;
    if (categoryFilter) {
      list = list.filter(m =>
        m.categories?.toLowerCase().split(',').some(c => c.trim() === categoryFilter.toLowerCase())
      );
    }
    if (showDetectedLibrariesOnly) {
      list = list.filter(m => m.isLibrary);
    }
    if (showUnusedOnly) {
      list = list.filter(m => unusedLibraries.includes(m.id));
    }
    return list;
  }, [query, results, mods, categoryFilter, showDetectedLibrariesOnly, showUnusedOnly, unusedLibraries]);

  const selectedMod = mods.find(m => m.id === selectedModId);

  useEffect(() => {
    GetSettings().then(s => {
      if (!s || !s.instancePath) {
        setShowSetup(true);
      }
      setSetupChecked(true);
    }).catch(() => setSetupChecked(true));

    GetInstanceName().then(name => {
      if (name) setInstanceName(name);
    }).catch(() => {});
  }, []);

  useEffect(() => {
    GetInstanceName().then(name => {
      if (name) setInstanceName(name);
    }).catch(() => {});
  }, [mods]);

  const handleSelectMod = useCallback((id: string) => {
    setSelectedModId(id);
    config.loadConfigsForMod(id);
  }, [config]);

  const handleOpenConfig = useCallback((configPath: string) => {
    config.openConfig(configPath);
  }, [config]);

  const handleConfigSelect = useCallback(async (configPath: string) => {
    if (!selectedModId) return;
    await config.setOverride(selectedModId, configPath);
    await config.loadConfigsForMod(selectedModId);
    setShowConfigPicker(false);
    config.openConfig(configPath);
  }, [config, selectedModId]);

  const handleCategoryFilter = useCallback((cat: string) => {
    setCategoryFilter(prev => prev === cat ? '' : cat);
  }, []);

  const handleDetectedLibraryFilter = useCallback(() => {
    setShowDetectedLibrariesOnly(prev => !prev);
  }, []);

  return (
    <div className="flex h-screen flex-col bg-[radial-gradient(circle_at_top_left,_rgba(52,211,153,0.06),_transparent_28%),linear-gradient(180deg,_#fbfefd_0%,_#f8fafc_100%)] text-gray-900 overflow-hidden">
      <TitleBar instanceName={instanceName} />

      <div className="flex min-h-0 flex-1">
        <Sidebar
          activeView={view}
          onViewChange={setView}
          onScan={() => ScanNow()}
          scanStatus={scanStatus}
        />

        <div className="flex-1 flex flex-col min-w-0">
        {view === 'mods' && (
          <>
            {/* Header */}
            <div className="px-4 py-4 border-b border-emerald-100/80 bg-white/80 backdrop-blur-sm shadow-[0_1px_0_0_rgba(16,185,129,0.06)]">
              {instanceName && (
                <div className="mb-3 flex items-center gap-2">
                  <span className="inline-flex items-center rounded-full bg-emerald-50 px-2 py-1 text-[10px] font-semibold uppercase tracking-[0.12em] text-emerald-700 ring-1 ring-emerald-200">
                    Active Pack
                  </span>
                  <h1 className="min-w-0 truncate text-lg font-semibold text-slate-900">{instanceName}</h1>
                </div>
              )}
              <SearchBar query={query} onChange={setQuery} searching={searching} />

              <div className="flex items-center gap-3 mt-3 text-xs text-slate-500">
                <span>{displayMods.length} mod{displayMods.length !== 1 ? 's' : ''}</span>
                {unusedLibraries.length > 0 && !showUnusedOnly && (
                  <button
                    onClick={() => setShowUnusedOnly(true)}
                    className="text-emerald-700 hover:text-emerald-800 transition-colors"
                  >
                    {unusedLibraries.length} unused librar{unusedLibraries.length !== 1 ? 'ies' : 'y'}
                  </button>
                )}
              </div>

              {/* Active filters */}
              {(categoryFilter || showDetectedLibrariesOnly || showUnusedOnly) && (
                <div className="flex items-center gap-1.5 mt-2 flex-wrap">
                  {showUnusedOnly && (
                    <button
                      onClick={() => setShowUnusedOnly(false)}
                      className="text-[11px] px-2 py-0.5 rounded-full bg-emerald-100 text-emerald-800 font-medium hover:bg-emerald-200 transition-colors"
                    >
                      Unused Libraries ×
                    </button>
                  )}
                  {showDetectedLibrariesOnly && (
                    <button
                      onClick={() => setShowDetectedLibrariesOnly(false)}
                      className="text-[11px] px-2 py-0.5 rounded-full bg-slate-200 text-slate-800 font-medium hover:bg-slate-300 transition-colors"
                    >
                      Detected Library ×
                    </button>
                  )}
                  {categoryFilter && (
                    <button
                      onClick={() => setCategoryFilter('')}
                      className="text-[11px] px-2 py-0.5 rounded-full bg-teal-100 text-teal-800 font-medium hover:bg-teal-200 transition-colors"
                    >
                      {categoryFilter} ×
                    </button>
                  )}
                </div>
              )}

              {/* Scan progress bar */}
              {scanStatus && (
                <div className="mt-2">
                  <div className="flex items-center gap-2">
                    <div className="flex-1 h-1.5 bg-emerald-100/70 rounded-full overflow-hidden">
                      <div
                        className="h-full rounded-full bg-gradient-to-r from-emerald-400 via-teal-400 to-cyan-400 transition-[width] duration-300"
                        style={{ width: `${Math.max(6, scanProgress)}%` }}
                      />
                    </div>
                    <span className="text-[10px] text-slate-600 shrink-0 w-10 text-right tabular-nums">{scanProgress}%</span>
                  </div>
                  <div className="mt-1 text-[11px] text-slate-600 truncate">{scanStatus}</div>
                </div>
              )}
            </div>

            {/* Split view: list + detail/editor */}
            <div className="flex-1 flex min-h-0">
              <div className={`${selectedModId ? 'w-1/2 border-r border-emerald-100/80' : 'w-full'} min-h-0`}>
                <ModList
                  mods={displayMods}
                  selectedModId={selectedModId}
                  onSelectMod={handleSelectMod}
                  unusedLibraries={unusedLibraries}
                  scanStatus={scanStatus}
                  onCategoryFilter={handleCategoryFilter}
                  onDetectedLibraryFilter={handleDetectedLibraryFilter}
                  categoryFilter={categoryFilter}
                  detectedLibraryFilter={showDetectedLibrariesOnly}
                />
              </div>

              {selectedModId && (
                <div className="w-1/2 min-h-0 flex flex-col">
                  {config.activePath ? (
                    <ConfigEditor
                      configPath={config.activePath}
                      fullPath={config.activeFullPath}
                      content={config.content}
                      onChange={(v) => { config.setContent(v); config.setDirty(true); }}
                      onSave={config.saveConfig}
                      onClose={() => { config.openConfig(''); }}
                      dirty={config.dirty}
                    />
                  ) : (
                    <ModDetail
                      modId={selectedModId}
                      onClose={() => setSelectedModId(null)}
                      onOpenConfig={handleOpenConfig}
                      onLinkConfig={() => setShowConfigPicker(true)}
                      onSelectMod={handleSelectMod}
                      onModsChanged={refresh}
                    />
                  )}
                </div>
              )}
            </div>
          </>
        )}

        {view === 'graph' && (
          <div className="relative flex-1 min-h-0 overflow-hidden">
            <ModGraph onSelectMod={(id) => { setSelectedModId(id); setView('mods'); }} />
          </div>
        )}

        {view === 'settings' && (
          <div className="flex-1 overflow-auto">
            <SettingsPanel />
          </div>
        )}
        </div>
      </div>
      {showConfigPicker && selectedMod && (
        <ConfigPicker
          configs={config.configs}
          onSelect={handleConfigSelect}
          onClose={() => setShowConfigPicker(false)}
          modName={selectedMod.name || selectedMod.id}
          modId={selectedMod.id}
        />
      )}

      {/* First-startup setup wizard */}
      {setupChecked && showSetup && (
        <SetupWizard onComplete={() => setShowSetup(false)} />
      )}
    </div>
  );
}
