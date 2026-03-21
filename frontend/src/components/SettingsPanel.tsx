import { useState, useEffect } from 'react';
import { GetSettings, SaveSettings, GetInstancesForSettings, BrowseForFolder, ScanNow, ResetDatabase } from '../../wailsjs/go/main/App';
import { Folder, RefreshCw05, Check, Trash01 } from '@untitled-ui/icons-react';
import type { Settings, Instance } from '../lib/types';
import { AccuracyNotice } from './AccuracyNotice';
import { LauncherRootField } from './LauncherRootField';
import { clampLibraryThreshold, defaultSettings, launcherRootGuides, normalizeSettings, otherLauncherHelper, settingsEqual, type LauncherRootField as LauncherRootFieldKey } from '../lib/settings';

export function SettingsPanel() {
  const [settings, setSettings] = useState<Settings>(defaultSettings);
  const [savedSettings, setSavedSettings] = useState<Settings>(defaultSettings);
  const [instances, setInstances] = useState<Instance[]>([]);
  const [saved, setSaved] = useState(false);
  const [savedToastVisible, setSavedToastVisible] = useState(false);
  const [resetting, setResetting] = useState(false);

  useEffect(() => {
    GetSettings().then(s => {
      const normalized = normalizeSettings(s as Settings);
      setSettings(normalized);
      setSavedSettings(normalized);
      refreshInstances(normalized);
    }).catch(console.error);
  }, []);

  const hasUnsavedChanges = !settingsEqual(settings, savedSettings);

  const handleSave = async () => {
    try {
      const nextSettings = {
        ...settings,
        mixedTagLibraryThreshold: clampLibraryThreshold(settings.mixedTagLibraryThreshold, defaultSettings.mixedTagLibraryThreshold),
        noTagLibraryThreshold: clampLibraryThreshold(settings.noTagLibraryThreshold, defaultSettings.noTagLibraryThreshold),
      };
      await SaveSettings(nextSettings);
      setSettings(nextSettings);
      setSavedSettings(nextSettings);
      refreshInstances();
      setSaved(true);
      setSavedToastVisible(true);
      setTimeout(() => setSaved(false), 2000);
      setTimeout(() => setSavedToastVisible(false), 2600);
    } catch (e) {
      console.error('Failed to save settings:', e);
    }
  };

  const refreshInstances = (nextSettings: Settings = settings) => {
    GetInstancesForSettings(nextSettings).then(i => setInstances(i || [])).catch(console.error);
  };

  const handleBrowse = async () => {
    try {
      const path = await BrowseForFolder();
      if (path) {
        setSettings(s => ({ ...s, instancePath: path }));
      }
    } catch (e) {
      console.error('Browse failed:', e);
    }
  };

  const handleBrowseLauncherRoot = async (field: LauncherRootFieldKey) => {
    try {
      const path = await BrowseForFolder();
      if (path) {
        setSettings(s => ({ ...s, [field]: path }));
      }
    } catch (e) {
      console.error('Browse failed:', e);
    }
  };

  const handleAddOtherLauncherRoot = async () => {
    try {
      const path = await BrowseForFolder();
      if (!path) return;
      setSettings(s => {
        const existing = (s.customLauncherRoots || '').trim();
        const next = existing ? `${existing}\n${path}` : path;
        return { ...s, customLauncherRoots: next };
      });
    } catch (e) {
      console.error('Browse failed:', e);
    }
  };

  const handleReset = async () => {
    if (!confirm('Reset the entire database? All cached data will be cleared and mods will be re-scanned.')) return;
    setResetting(true);
    try {
      await ResetDatabase();
    } catch (e) {
      console.error('Reset failed:', e);
    } finally {
      setResetting(false);
    }
  };

  return (
    <div className="max-w-2xl mx-auto p-6 space-y-6">
      <h1 className="text-xl font-semibold text-gray-900">Settings</h1>

      <AccuracyNotice />

      {/* Instance Path */}
      <section className="space-y-3">
        <h2 className="text-sm font-medium text-gray-700">Minecraft Instance</h2>

        <div className="flex gap-2">
          <input
            type="text"
            value={settings.instancePath}
            onChange={e => setSettings(s => ({ ...s, instancePath: e.target.value }))}
            placeholder="Path to minecraft instance folder..."
            className="flex-1 px-3 py-2 bg-white border border-gray-200 rounded-lg text-sm text-gray-900 placeholder-gray-400 focus:outline-none focus:border-gray-400 focus:ring-1 focus:ring-gray-400"
          />
          <button
            onClick={handleBrowse}
            className="px-3 py-2 rounded-lg border border-gray-200 text-gray-600 hover:bg-gray-50 transition-colors"
            title="Browse for folder"
          >
            <Folder width={16} height={16} />
          </button>
        </div>

      </section>

      <section className="space-y-3">
        <div className="flex items-center justify-between gap-3">
          <div>
            <h2 className="text-sm font-medium text-gray-700">Launcher Roots</h2>
            <p className="text-xs text-gray-400">Add custom launcher root folders for auto-detection. Default locations are still scanned.</p>
          </div>
          <button
            onClick={() => refreshInstances()}
            className="inline-flex items-center gap-2 rounded-lg border border-gray-200 px-3 py-2 text-xs text-gray-600 transition-colors hover:bg-gray-50"
          >
            <RefreshCw05 width={14} height={14} /> Refresh found packs
          </button>
        </div>

        {launcherRootGuides.map(guide => (
          <LauncherRootField
            key={guide.field}
            label={guide.label}
            value={settings[guide.field] || ''}
            onChange={value => setSettings(s => ({ ...s, [guide.field]: value }))}
            onBrowse={() => handleBrowseLauncherRoot(guide.field)}
            placeholder={guide.placeholder}
            helper={guide.helper}
            defaults={guide.defaults}
          />
        ))}

        <div>
          <div className="mb-1 flex items-center justify-between gap-2">
            <span className="text-xs text-gray-500">{otherLauncherHelper.label}</span>
            <button
              onClick={handleAddOtherLauncherRoot}
              className="rounded-lg border border-gray-200 px-2 py-1 text-xs text-gray-600 transition-colors hover:bg-gray-50"
            >
              Add folder...
            </button>
          </div>
          <textarea
            value={settings.customLauncherRoots || ''}
            onChange={e => setSettings(s => ({ ...s, customLauncherRoots: e.target.value }))}
            placeholder={otherLauncherHelper.placeholder}
            rows={4}
            className="w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm text-gray-900 placeholder-gray-400 focus:outline-none focus:border-gray-400 focus:ring-1 focus:ring-gray-400"
          />
          <div className="mt-1 space-y-1">
            <p className="text-[11px] text-gray-500">{otherLauncherHelper.helper}</p>
            <p className="text-[11px] text-gray-400">Default: {otherLauncherHelper.defaults.join(' or ')}</p>
          </div>
        </div>

        {instances.length > 0 && (
          <div>
            <p className="text-xs text-gray-400 mb-2">Detected packs from all configured roots:</p>
            <div className="grid gap-1.5">
              {instances.filter(i => i.hasMods).map((inst, i) => (
                <button
                  key={`${inst.launcher}-${inst.path}-${i}`}
                  onClick={() => setSettings(s => ({ ...s, instancePath: inst.path }))}
                  className={`flex items-center gap-2 px-3 py-2 rounded-lg border text-left text-sm transition-colors ${
                    settings.instancePath === inst.path
                      ? 'bg-gray-50 border-gray-400 text-gray-900'
                      : 'bg-white border-gray-200 text-gray-700 hover:border-gray-300'
                  }`}
                >
                  <div className="flex-1 min-w-0">
                    <div className="font-medium truncate">{inst.name}</div>
                    <div className="text-xs text-gray-400 truncate">{inst.path}</div>
                  </div>
                  <span className="text-[10px] text-gray-400 uppercase font-medium">{inst.launcher}</span>
                </button>
              ))}
            </div>
          </div>
        )}
      </section>

      {/* API Keys */}
      <section className="space-y-3">
        <h2 className="text-sm font-medium text-gray-700">API Keys</h2>

        <div className="space-y-2">
          <label className="block">
            <span className="text-xs text-gray-500">CurseForge API Key</span>
            <input
              type="password"
              value={settings.curseForgeAPIKey}
              onChange={e => setSettings(s => ({ ...s, curseForgeAPIKey: e.target.value }))}
              placeholder="Enter your CurseForge API key..."
              className="mt-1 w-full px-3 py-2 bg-white border border-gray-200 rounded-lg text-sm text-gray-900 placeholder-gray-400 focus:outline-none focus:border-gray-400"
            />
            <span className="text-xs text-gray-400 mt-1 block">
              Get yours at{' '}
              <a href="https://console.curseforge.com" target="_blank" rel="noopener noreferrer" className="text-gray-600 underline hover:text-gray-900">
                console.curseforge.com
              </a>
            </span>
          </label>

          <label className="block">
            <span className="text-xs text-gray-500">Modrinth API Key (optional)</span>
            <input
              type="password"
              value={settings.modrinthAPIKey}
              onChange={e => setSettings(s => ({ ...s, modrinthAPIKey: e.target.value }))}
              placeholder="Optional — Modrinth works without an API key"
              className="mt-1 w-full px-3 py-2 bg-white border border-gray-200 rounded-lg text-sm text-gray-900 placeholder-gray-400 focus:outline-none focus:border-gray-400"
            />
          </label>
        </div>
      </section>

      <section className="space-y-3">
        <div>
          <h2 className="text-sm font-medium text-gray-700">Library Detection</h2>
          <p className="text-xs text-gray-400">These thresholds are compared against the semantic score shown in mod details. Save and rescan to apply them to stored library flags.</p>
        </div>

        <div className="grid gap-3 sm:grid-cols-2">
          <label className="block">
            <span className="text-xs text-gray-500">Mixed tags threshold</span>
            <input
              type="number"
              min={0.01}
              max={0.95}
              step={0.01}
              value={settings.mixedTagLibraryThreshold}
              onChange={e => setSettings(s => ({ ...s, mixedTagLibraryThreshold: Number(e.target.value) }))}
              className="mt-1 w-full px-3 py-2 bg-white border border-gray-200 rounded-lg text-sm text-gray-900 focus:outline-none focus:border-gray-400"
            />
            <span className="text-[11px] text-gray-400 mt-1 block">Used when a mod has a library tag plus other categories.</span>
          </label>

          <label className="block">
            <span className="text-xs text-gray-500">No-tag threshold</span>
            <input
              type="number"
              min={0.01}
              max={0.95}
              step={0.01}
              value={settings.noTagLibraryThreshold}
              onChange={e => setSettings(s => ({ ...s, noTagLibraryThreshold: Number(e.target.value) }))}
              className="mt-1 w-full px-3 py-2 bg-white border border-gray-200 rounded-lg text-sm text-gray-900 focus:outline-none focus:border-gray-400"
            />
            <span className="text-[11px] text-gray-400 mt-1 block">Used when a mod has no explicit library tag and must rely on semantic similarity alone.</span>
          </label>
        </div>
      </section>

      {/* Actions */}
      <div className="flex items-center gap-3 pt-4 border-t border-gray-200 pb-20">
        <button
          onClick={handleSave}
          className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-gray-900 text-white text-sm font-medium hover:bg-gray-800 transition-colors"
        >
          {saved ? <Check width={16} height={16} /> : null}
          {saved ? 'Saved!' : 'Save Settings'}
        </button>
        <button
          onClick={() => ScanNow()}
          className="inline-flex items-center gap-2 px-4 py-2 rounded-lg border border-gray-200 text-gray-700 text-sm hover:bg-gray-50 transition-colors"
        >
          <RefreshCw05 width={14} height={14} /> Rescan Mods
        </button>
      </div>

      {/* Danger zone */}
      <section className="pt-4 border-t border-gray-200 space-y-2">
        <h2 className="text-sm font-medium text-gray-700">Danger Zone</h2>
        <p className="text-xs text-gray-400">Clear all cached data and re-scan from scratch.</p>
        <button
          onClick={handleReset}
          disabled={resetting}
          className="inline-flex items-center gap-2 px-4 py-2 rounded-lg border border-red-200 text-red-600 text-sm hover:bg-red-50 disabled:opacity-50 transition-colors"
        >
          <Trash01 width={14} height={14} />
          {resetting ? 'Resetting...' : 'Reset Database'}
        </button>
      </section>

      {hasUnsavedChanges && (
        <div className="fixed bottom-5 right-6 z-20 rounded-xl border border-gray-200 bg-white/95 p-3 shadow-lg backdrop-blur-sm">
          <div className="flex items-center gap-3">
            <div>
              <div className="text-sm font-semibold text-gray-900">Unsaved settings</div>
              <div className="text-xs text-gray-500">Your changes are not applied yet.</div>
            </div>
            <button
              onClick={handleSave}
              className="inline-flex items-center gap-2 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800"
            >
              {saved ? <Check width={16} height={16} /> : null}
              {saved ? 'Saved!' : 'Save Settings'}
            </button>
          </div>
        </div>
      )}

      <div className={`pointer-events-none fixed right-6 top-5 z-30 transform rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 shadow-lg transition-all duration-300 ${savedToastVisible ? 'translate-y-0 opacity-100' : '-translate-y-2 opacity-0'}`}>
        <div className="flex items-center gap-2 text-sm text-emerald-800">
          <span className="inline-flex h-5 w-5 items-center justify-center rounded-full bg-emerald-600 text-xs font-bold text-white">S</span>
          <div>
            <div className="font-semibold">Settings saved</div>
            <div className="text-xs text-emerald-700">Launcher roots and API settings have been updated.</div>
          </div>
        </div>
      </div>
    </div>
  );
}
