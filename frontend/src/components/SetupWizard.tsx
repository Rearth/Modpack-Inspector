import { useState, useEffect } from 'react';
import { GetSettings, SaveSettings, GetInstancesForSettings, BrowseForFolder } from '../../wailsjs/go/main/App';
import { Folder, Package, RefreshCw05 } from '@untitled-ui/icons-react';
import type { Settings, Instance } from '../lib/types';
import { AccuracyNotice } from './AccuracyNotice';
import { LauncherRootField } from './LauncherRootField';
import { defaultSettings, launcherRootGuides, normalizeSettings, otherLauncherHelper, type LauncherRootField as LauncherRootFieldKey } from '../lib/settings';

interface SetupWizardProps {
  onComplete: () => void;
}

export function SetupWizard({ onComplete }: SetupWizardProps) {
  const [instances, setInstances] = useState<Instance[]>([]);
  const [selected, setSelected] = useState('');
  const [saving, setSaving] = useState(false);
  const [settings, setSettings] = useState<Settings>(defaultSettings);
  const [showCustomRoots, setShowCustomRoots] = useState(false);

  useEffect(() => {
    GetSettings()
      .then(current => {
        const normalized = normalizeSettings(current as Settings);
        setSettings(normalized);
        return GetInstancesForSettings(normalized);
      })
      .then(i => setInstances((i || []).filter(inst => inst.hasMods)))
      .catch(console.error);
  }, []);

  const handleSelect = async () => {
    if (!selected) return;
    setSaving(true);
    try {
      await SaveSettings({
        ...settings,
        instancePath: selected,
      } as Settings);
      onComplete();
    } catch (e) {
      console.error('Failed to save:', e);
      setSaving(false);
    }
  };

  const handleBrowse = async () => {
    try {
      const path = await BrowseForFolder();
      if (path) {
        setSelected(path);
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

  const refreshInstances = async (nextSettings: Settings = settings) => {
    try {
      const found = await GetInstancesForSettings(nextSettings);
      setInstances((found || []).filter(inst => inst.hasMods));
    } catch (e) {
      console.error('Failed to refresh instances:', e);
    }
  };

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto bg-black/20 backdrop-blur-sm">
      <div className="flex min-h-full items-start justify-center p-4 sm:items-center">
        <div className="max-h-[calc(100vh-2rem)] w-full max-w-lg overflow-y-auto rounded-xl border border-gray-200 bg-white p-6 shadow-xl">
        <div className="flex items-center gap-3 mb-5">
          <div className="p-2.5 bg-gray-900 rounded-lg">
            <Package width={20} height={20} className="text-white" />
          </div>
          <div>
            <h1 className="text-lg font-semibold text-gray-900">Welcome to ModpackTool</h1>
            <p className="text-sm text-gray-500">Select a modpack to get started</p>
          </div>
        </div>

        {instances.length > 0 && (
          <div className="space-y-1.5 mb-4 max-h-64 overflow-auto">
            {instances.map((inst, i) => (
              <button
                key={i}
                onClick={() => setSelected(inst.path)}
                className={`w-full flex items-center gap-2 px-3 py-2.5 rounded-lg border text-left text-sm transition-colors ${
                  selected === inst.path
                    ? 'bg-gray-50 border-gray-400 text-gray-900'
                    : 'bg-white border-gray-200 text-gray-700 hover:border-gray-300'
                }`}
              >
                <div className="flex-1 min-w-0">
                  <div className="font-medium truncate">{inst.name}</div>
                  <div className="text-xs text-gray-400 truncate">{inst.path}</div>
                </div>
                <span className="text-[10px] text-gray-400 uppercase font-medium shrink-0">{inst.launcher}</span>
              </button>
            ))}
          </div>
        )}

        {instances.length === 0 && (
          <p className="text-sm text-gray-500 mb-4">
            No Minecraft instances with mods were detected. Use the button below to select a folder manually.
          </p>
        )}

        <div className="mb-4">
          <AccuracyNotice />
        </div>

        <div className="mb-4 rounded-xl border border-gray-200 bg-gray-50/80 p-4">
          <div className="flex items-center justify-between gap-3">
            <div>
              <div className="text-sm font-semibold text-gray-900">Can&apos;t find your launcher?</div>
              <div className="text-xs text-gray-500">Add custom roots and refresh the detected pack list before picking one.</div>
            </div>
            <button
              onClick={() => setShowCustomRoots(prev => !prev)}
              className="rounded-lg border border-gray-200 bg-white px-3 py-2 text-xs text-gray-600 transition-colors hover:bg-gray-100"
            >
              {showCustomRoots ? 'Hide roots' : 'Edit roots'}
            </button>
          </div>

          {showCustomRoots && (
            <div className="mt-4 space-y-3">
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
                    className="rounded-lg border border-gray-200 bg-white px-2 py-1 text-xs text-gray-600 transition-colors hover:bg-gray-100"
                  >
                    Add folder...
                  </button>
                </div>
                <textarea
                  value={settings.customLauncherRoots || ''}
                  onChange={e => setSettings(s => ({ ...s, customLauncherRoots: e.target.value }))}
                  placeholder={otherLauncherHelper.placeholder}
                  rows={3}
                  className="w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm text-gray-900 placeholder-gray-400 focus:outline-none focus:border-gray-400 focus:ring-1 focus:ring-gray-400"
                />
                <div className="mt-1 space-y-1">
                  <p className="text-[11px] text-gray-500">{otherLauncherHelper.helper}</p>
                  <p className="text-[11px] text-gray-400">Default: {otherLauncherHelper.defaults.join(' or ')}</p>
                </div>
              </div>

              <button
                onClick={() => refreshInstances()}
                className="inline-flex items-center gap-2 rounded-lg border border-gray-200 bg-white px-3 py-2 text-xs text-gray-600 transition-colors hover:bg-gray-100"
              >
                <RefreshCw05 width={14} height={14} /> Refresh found packs
              </button>
            </div>
          )}
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={handleBrowse}
            className="flex items-center gap-2 px-3 py-2 rounded-lg border border-gray-200 text-gray-700 hover:bg-gray-50 transition-colors text-sm"
          >
            <Folder width={14} height={14} />
            Browse...
          </button>

          {selected && (
            <div className="flex-1 text-xs text-gray-400 truncate px-2">{selected}</div>
          )}

          <button
            onClick={handleSelect}
            disabled={!selected || saving}
            className="ml-auto px-4 py-2 rounded-lg bg-gray-900 text-white text-sm font-medium hover:bg-gray-800 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
          >
            {saving ? 'Loading...' : 'Get Started'}
          </button>
        </div>
        </div>
      </div>
    </div>
  );
}
