import type { Settings } from './types';

export const defaultSettings: Settings = {
  instancePath: '',
  curseForgeAPIKey: '',
  modrinthAPIKey: '',
  cacheTTLHours: 24,
  appScale: 100,
  mixedTagLibraryThreshold: 0.18,
  noTagLibraryThreshold: 0.26,
  customModrinthRoot: '',
  customCurseForgeRoot: '',
  customFTBRoot: '',
  customLauncherRoots: '',
};

export const MIN_LIBRARY_THRESHOLD = 0.01;
export const MAX_LIBRARY_THRESHOLD = 0.95;

export function clampLibraryThreshold(value: number, fallback: number): number {
  if (!Number.isFinite(value)) return fallback;
  return Math.min(MAX_LIBRARY_THRESHOLD, Math.max(MIN_LIBRARY_THRESHOLD, value));
}

export type LauncherRootField = 'customModrinthRoot' | 'customCurseForgeRoot' | 'customFTBRoot';

export interface LauncherRootGuide {
  field: LauncherRootField;
  label: string;
  placeholder: string;
  helper: string;
  defaults: string[];
}

export const launcherRootGuides: LauncherRootGuide[] = [
  {
    field: 'customModrinthRoot',
    label: 'Custom Modrinth root',
    placeholder: 'Optional custom Modrinth profiles root',
    helper: 'Point this at the profiles directory. Expected layout: <root>/<pack>/mods.',
    defaults: [
      '%APPDATA%\\com.modrinth.theseus\\profiles',
      '%APPDATA%\\ModrinthApp\\profiles',
    ],
  },
  {
    field: 'customCurseForgeRoot',
    label: 'Custom CurseForge root',
    placeholder: 'Optional custom CurseForge instances root',
    helper: 'Point this at the instances directory. Expected layout: <root>/<pack>/mods.',
    defaults: [
      '%USERPROFILE%\\curseforge\\minecraft\\Instances',
    ],
  },
  {
    field: 'customFTBRoot',
    label: 'Custom FTB root',
    placeholder: 'Optional custom FTB instances root',
    helper: 'Point this at the instances directory. Expected layout: <root>/<pack>/mods.',
    defaults: [
      '%APPDATA%\\FTBA\\instances',
      '%LOCALAPPDATA%\\FTBApp\\instances',
    ],
  },
];

export const otherLauncherHelper = {
  label: 'Other launcher roots',
  placeholder: 'One launcher root per line',
  helper: 'Point each entry at a folder whose immediate child folders are modpacks. Expected layout: <root>/<pack>/mods.',
  defaults: ['No default path'],
};

export function normalizeSettings(settings?: Partial<Settings> | null): Settings {
  return {
    instancePath: settings?.instancePath || '',
    curseForgeAPIKey: settings?.curseForgeAPIKey || '',
    modrinthAPIKey: settings?.modrinthAPIKey || '',
    cacheTTLHours: settings?.cacheTTLHours || 24,
    appScale: settings?.appScale || 100,
    mixedTagLibraryThreshold: settings?.mixedTagLibraryThreshold ?? 0.18,
    noTagLibraryThreshold: settings?.noTagLibraryThreshold ?? 0.26,
    customModrinthRoot: settings?.customModrinthRoot || '',
    customCurseForgeRoot: settings?.customCurseForgeRoot || '',
    customFTBRoot: settings?.customFTBRoot || '',
    customLauncherRoots: settings?.customLauncherRoots || '',
  };
}

export function settingsEqual(a: Settings, b: Settings) {
  return a.instancePath === b.instancePath
    && a.curseForgeAPIKey === b.curseForgeAPIKey
    && a.modrinthAPIKey === b.modrinthAPIKey
    && a.cacheTTLHours === b.cacheTTLHours
    && a.appScale === b.appScale
    && a.mixedTagLibraryThreshold === b.mixedTagLibraryThreshold
    && a.noTagLibraryThreshold === b.noTagLibraryThreshold
    && (a.customModrinthRoot || '') === (b.customModrinthRoot || '')
    && (a.customCurseForgeRoot || '') === (b.customCurseForgeRoot || '')
    && (a.customFTBRoot || '') === (b.customFTBRoot || '')
    && (a.customLauncherRoots || '') === (b.customLauncherRoots || '');
}

// App-scale constants and helpers (consumed by uiScale.ts)
export const DEFAULT_APP_SCALE = 100;
export const MIN_APP_SCALE = 50;
export const MAX_APP_SCALE = 200;
export const APP_SCALE_STEP = 10;

export function clampAppScale(v: number): number {
  return Math.min(MAX_APP_SCALE, Math.max(MIN_APP_SCALE, Math.round(v / APP_SCALE_STEP) * APP_SCALE_STEP));
}

export function getAppScaleFactor(scale: number): number {
  return scale / 100;
}

export function applyAppScale(scale: number): void {
  document.documentElement.style.fontSize = `${getAppScaleFactor(scale) * 16}px`;
}