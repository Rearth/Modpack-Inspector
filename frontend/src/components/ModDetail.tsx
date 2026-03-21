import { useEffect, useState } from 'react';
import { GetModDetail, GetReverseDependencies, SetLibraryOverride } from '../../wailsjs/go/main/App';
import { File06, Link04, XClose, ArrowRight, Plus, AlertTriangle, LinkExternal01, ChevronDown, ChevronRight, BookOpen01 } from '@untitled-ui/icons-react';
import type { ModDetail as ModDetailType, ConfigMapping, ReverseDep, DetailDependency, UnresolvedExternalDependency, MixinDetail, IncomingMixin } from '../lib/types';
import { SourceLinkPill } from './SourceLinkPill';


interface ModDetailProps {
  modId: string;
  onClose: () => void;
  onOpenConfig: (configPath: string) => void;
  onLinkConfig: () => void;
  onSelectMod: (id: string) => void;
  onModsChanged?: () => Promise<void> | void;
}

export function ModDetail({ modId, onClose, onOpenConfig, onLinkConfig, onSelectMod, onModsChanged }: ModDetailProps) {
  const [detail, setDetail] = useState<ModDetailType | null>(null);
  const [reverseDeps, setReverseDeps] = useState<ReverseDep[]>([]);
  const [loading, setLoading] = useState(true);
  const [showUnresolvedPopup, setShowUnresolvedPopup] = useState(false);
  const [mixinsExpanded, setMixinsExpanded] = useState(false);
  const [incomingMixinsExpanded, setIncomingMixinsExpanded] = useState(false);

  useEffect(() => {
    setLoading(true);
    setShowUnresolvedPopup(false);
    Promise.all([
      GetModDetail(modId),
      GetReverseDependencies(modId).catch(() => []),
    ]).then(([d, rd]) => {
      setDetail(d as ModDetailType);
      setReverseDeps(rd || []);
    }).catch(console.error)
      .finally(() => setLoading(false));
  }, [modId]);

  if (loading) {
    return (
      <div className="p-4 flex items-center justify-center h-full">
        <div className="w-6 h-6 border-2 border-gray-300 border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  if (!detail) return null;
  const { mod, dependencies, configs } = detail;
  const libraryDetection = detail.libraryDetection;
  const unresolvedExternal = detail.unresolvedExternal || [];
  const categories = mod.categories ? mod.categories.split(',').map(c => c.trim()).filter(Boolean) : [];
  const providedModules = detail.providedModules || [];
  const providedModuleSet = new Set(providedModules);
  const mixins = detail.mixins || [];
  const incomingMixins = detail.incomingMixins || [];

  return (
    <div className="relative flex flex-col h-full bg-white">
      {/* Header */}
      <div className="p-4 border-b border-gray-200 flex items-start gap-3">
        {mod.iconURL ? (
          <img src={mod.iconURL} alt="" className="w-12 h-12 rounded-lg" />
        ) : (
          <div className="w-12 h-12 rounded-lg bg-gray-100 flex items-center justify-center text-gray-400 text-lg font-bold">
            {(mod.name || mod.id).charAt(0).toUpperCase()}
          </div>
        )}
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <h2 className="text-lg font-semibold text-gray-900 truncate">{mod.name || mod.id}</h2>
            <button onClick={onClose} className="ml-auto text-gray-400 hover:text-gray-700 shrink-0">
              <XClose width={18} height={18} />
            </button>
          </div>
          <p className="text-xs text-gray-500 font-mono">{mod.id} · v{mod.version}</p>
          {mod.authors && <p className="text-xs text-gray-400 mt-0.5">by {mod.authors}</p>}
          {categories.length > 0 && (
            <div className="flex gap-1 mt-1.5 flex-wrap">
              {categories.map(cat => (
                <span key={cat} className="text-[10px] px-1.5 py-0.5 rounded-full bg-gray-100 text-gray-500 font-medium">
                  {cat}
                </span>
              ))}
              {mod.isLibrary && (
                <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-blue-50 text-blue-600 font-medium">
                  Detected Library
                </span>
              )}
              {mod.libraryOverride === 1 && (
                <span className="inline-flex items-center gap-0.5 text-[10px] px-1.5 py-0.5 rounded-full bg-blue-50 text-blue-700 font-medium" title="Manually forced as library">
                  <BookOpen01 width={10} height={10} />
                  Forced
                </span>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-auto p-4 space-y-5">
        {/* Description */}
        {(mod.description || mod.onlineDesc) && (
          <div>
            <h3 className="text-xs font-medium text-gray-500 uppercase tracking-wider mb-1">Description</h3>
            <p className="text-sm text-gray-700 leading-relaxed">{mod.onlineDesc || mod.description}</p>
          </div>
        )}

        <LibraryOverrideCard debug={libraryDetection} modId={modId} onOverrideChange={() => {
          Promise.all([
            GetModDetail(modId).then(d => {
              setDetail(d as ModDetailType);
            }),
            Promise.resolve(onModsChanged?.()),
          ]).catch(console.error);
        }} />

        {providedModules.length > 0 && (
          <div>
            <h3 className="text-xs font-medium text-gray-500 uppercase tracking-wider mb-2">
              Includes Modules ({providedModules.length})
            </h3>
            <div className="flex flex-wrap gap-1.5">
              {providedModules.map(moduleID => (
                <span key={moduleID} className="text-[11px] px-2 py-1 rounded-full bg-indigo-50 text-indigo-700 font-medium">
                  {moduleID}
                </span>
              ))}
            </div>
          </div>
        )}

        {/* Links */}
        <div className="flex gap-2 flex-wrap">
          {mod.curseForgeURL && (
            <SourceLinkPill href={mod.curseForgeURL} label="CurseForge" shortLabel="CF" tone="curseforge" />
          )}
          {mod.modrinthURL && (
            <SourceLinkPill href={mod.modrinthURL} label="Modrinth" shortLabel="MR" tone="modrinth" />
          )}
        </div>

        {/* Dependencies */}
        {dependencies && dependencies.length > 0 && (
          <div>
            <h3 className="text-xs font-medium text-gray-500 uppercase tracking-wider mb-2">
              Dependencies ({dependencies.length})
            </h3>
            <div className="space-y-1">
              {dependencies.map((dep, i) => (
                <DependencyRow
                  key={i}
                  dep={dep}
                  providedByCurrentMod={providedModuleSet.has(dep.depModID)}
                  onSelectMod={onSelectMod}
                />
              ))}
            </div>
          </div>
        )}

        {unresolvedExternal.length > 0 && (
          <div>
            <button
              onClick={() => setShowUnresolvedPopup(true)}
              className="w-full rounded-xl border border-amber-200 bg-amber-50/80 px-3 py-2 text-left transition-colors hover:bg-amber-100/80"
            >
              <div className="flex items-start gap-2">
                <AlertTriangle width={16} height={16} className="mt-0.5 shrink-0 text-amber-700" />
                <div className="min-w-0 flex-1">
                  <div className="text-sm font-medium text-amber-900">
                    {unresolvedExternal.length} online dependenc{unresolvedExternal.length === 1 ? 'y was' : 'ies were'} not found in this mod list
                  </div>
                  <div className="mt-0.5 text-xs text-amber-800">
                    Click to review the missing Modrinth and CurseForge entries and open them externally.
                  </div>
                </div>
                <ArrowRight width={14} height={14} className="shrink-0 text-amber-700" />
              </div>
            </button>
          </div>
        )}

        {/* Reverse dependencies (mods that depend on this one) */}
        {reverseDeps.length > 0 && (
          <div>
            <h3 className="text-xs font-medium text-gray-500 uppercase tracking-wider mb-2">
              Depended on by ({reverseDeps.length})
            </h3>
            <div className="space-y-1">
              {reverseDeps.map(dep => (
                <button
                  key={dep.modID}
                  onClick={() => onSelectMod(dep.modID)}
                  className="w-full flex items-center gap-2 text-sm px-2.5 py-1.5 rounded-lg bg-gray-50 hover:bg-gray-100 text-left transition-colors"
                >
                  {dep.iconURL ? (
                    <img src={dep.iconURL} alt="" className="w-5 h-5 rounded shrink-0" />
                  ) : (
                    <div className="w-5 h-5 rounded bg-gray-200 flex items-center justify-center shrink-0">
                      <span className="text-[8px] font-bold text-gray-400">{dep.name.charAt(0).toUpperCase()}</span>
                    </div>
                  )}
                  <div className="min-w-0 flex-1">
                    <div className="text-gray-800 truncate">{dep.name}</div>
                    {dep.via && <div className="text-[11px] text-indigo-700 truncate">via included module {dep.via}</div>}
                  </div>
                  <ArrowRight width={12} height={12} className="ml-auto text-gray-400 shrink-0" />
                </button>
              ))}
            </div>
          </div>
        )}

        {/* Config files */}
        <div>
          <div className="flex items-center justify-between mb-2 gap-2">
            <h3 className="text-xs font-medium text-gray-500 uppercase tracking-wider">
              Config Files
            </h3>
            <button
              onClick={onLinkConfig}
              className="inline-flex items-center gap-1 px-2 py-1 rounded-lg bg-gray-900 text-white text-[11px] font-medium hover:bg-gray-800 transition-colors"
            >
              <Plus width={12} height={12} /> {configs && configs.length > 0 ? 'Link Another' : 'Link Config'}
            </button>
          </div>
          {configs && configs.length > 0 ? (
            <div className="space-y-1">
              {configs.map((cfg, i) => (
                <ConfigItem key={i} config={cfg} onOpen={() => onOpenConfig(cfg.configPath)} />
              ))}
            </div>
          ) : (
            <div className="rounded-lg border border-dashed border-gray-200 bg-gray-50 px-3 py-2 text-xs text-gray-500">
              No config files are linked to this mod yet.
            </div>
          )}
        </div>

        {/* Outgoing Mixins */}
        {mixins.length > 0 && (
          <div>
            <button
              onClick={() => setMixinsExpanded(v => !v)}
              className="flex items-center gap-1 text-xs font-medium text-gray-500 uppercase tracking-wider mb-2 hover:text-gray-700 transition-colors"
            >
              {mixinsExpanded ? <ChevronDown width={14} height={14} /> : <ChevronRight width={14} height={14} />}
              Mixins ({mixins.length})
            </button>
            {mixinsExpanded && (
              <div className="space-y-2">
                {Object.entries(groupMixinsByTarget(mixins)).map(([targetModID, group]) => (
                  <MixinTargetGroup
                    key={targetModID}
                    targetModID={targetModID}
                    targetModName={group[0].targetModName}
                    mixins={group}
                    onSelectMod={onSelectMod}
                  />
                ))}
              </div>
            )}
          </div>
        )}

        {/* Incoming Mixins */}
        {incomingMixins.length > 0 && (
          <div>
            <button
              onClick={() => setIncomingMixinsExpanded(v => !v)}
              className="flex items-center gap-1 text-xs font-medium text-gray-500 uppercase tracking-wider mb-2 hover:text-gray-700 transition-colors"
            >
              {incomingMixinsExpanded ? <ChevronDown width={14} height={14} /> : <ChevronRight width={14} height={14} />}
              Modified by other mods ({incomingMixins.length} mixin{incomingMixins.length !== 1 ? 's' : ''})
            </button>
            {incomingMixinsExpanded && (
              <div className="space-y-1">
                {Object.entries(groupIncomingByOwner(incomingMixins)).map(([ownerModID, group]) => (
                  <IncomingMixinGroup
                    key={ownerModID}
                    ownerModID={ownerModID}
                    ownerModName={group[0].ownerModName}
                    ownerIconURL={group[0].ownerIconURL}
                    mixins={group}
                    onSelectMod={onSelectMod}
                  />
                ))}
              </div>
            )}
          </div>
        )}
      </div>

      {showUnresolvedPopup && unresolvedExternal.length > 0 && (
        <div className="absolute inset-0 z-20 flex items-start justify-center bg-slate-950/25 px-4 py-8 backdrop-blur-[1px]">
          <div className="w-full max-w-md rounded-2xl border border-slate-200 bg-white shadow-2xl">
            <div className="flex items-start gap-3 border-b border-slate-100 px-4 py-3">
              <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-amber-50 text-amber-700 ring-1 ring-amber-200">
                <AlertTriangle width={18} height={18} />
              </div>
              <div className="min-w-0 flex-1">
                <div className="text-sm font-semibold text-slate-900">Unresolved online dependencies</div>
                <div className="text-xs text-slate-500">These were reported by Modrinth or CurseForge but could not be matched to a loaded mod.</div>
              </div>
              <button onClick={() => setShowUnresolvedPopup(false)} className="rounded-lg p-1 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-700">
                <XClose width={16} height={16} />
              </button>
            </div>

            <div className="max-h-[60vh] space-y-2 overflow-auto px-4 py-3">
              {unresolvedExternal.map((dep, index) => (
                <UnresolvedExternalDependencyRow key={`${dep.source}-${dep.depModID}-${index}`} dep={dep} />
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function LibraryOverrideCard({ debug, modId, onOverrideChange }: { debug: ModDetailType['libraryDetection']; modId: string; onOverrideChange: () => void }) {
  const [saving, setSaving] = useState(false);
  const tone = debug.detected
    ? 'border-emerald-200 bg-emerald-50/70'
    : 'border-slate-200 bg-slate-50';

  const handleOverride = (value: number) => {
    setSaving(true);
    SetLibraryOverride(modId, value)
      .then(() => onOverrideChange())
      .catch(console.error)
      .finally(() => setSaving(false));
  };

  return (
    <div className={`rounded-xl border px-3 py-2.5 ${tone}`}>
      <div className="flex flex-wrap items-center gap-2.5">
        <div className="min-w-0 flex-1">
          <span className="text-sm font-medium text-slate-900">{debug.detected ? 'Detected as library' : 'Not detected as library'}</span>
          <span className="ml-2 text-[11px] text-slate-600">
            {debug.manualOverride === 1 ? 'Forced to library' : debug.manualOverride === -1 ? 'Forced to not library' : 'Automatic detection'}
          </span>
        </div>
        <span className="rounded-full bg-white/80 px-2 py-1 text-[10px] font-semibold uppercase tracking-[0.08em] text-slate-700">
          {debug.manualOverride !== 0 ? 'Override' : 'Auto'}
        </span>
        <div className="flex flex-wrap gap-1.5">
          {([
            { value: 0, label: 'Auto' },
            { value: 1, label: 'Force Library' },
            { value: -1, label: 'Force Not Library' },
          ] as const).map(opt => (
            <button
              key={opt.value}
              disabled={saving}
              onClick={() => handleOverride(opt.value)}
              className={`px-2.5 py-1 rounded-lg text-[11px] font-medium transition-colors ${
                debug.manualOverride === opt.value
                  ? 'bg-slate-900 text-white'
                  : 'bg-white text-slate-600 hover:bg-slate-100 border border-slate-200'
              }`}
            >
              {opt.label}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}

function UnresolvedExternalDependencyRow({ dep }: { dep: UnresolvedExternalDependency }) {
  const label = dep.depName || dep.depModID;
  const buttonTone = dep.source === 'curseforge'
    ? 'border-orange-200 bg-orange-50 text-orange-700 hover:bg-orange-100'
    : 'border-emerald-200 bg-emerald-50 text-emerald-700 hover:bg-emerald-100';

  return (
    <div className="rounded-xl border border-slate-200 bg-slate-50/80 px-3 py-2">
      <div className="flex items-start gap-3">
        <div className="min-w-0 flex-1">
          <div className="truncate text-sm font-medium text-slate-900">{label}</div>
          {dep.depName && dep.depName !== dep.depModID && (
            <div className="truncate text-[11px] text-slate-500">ID: {dep.depModID}</div>
          )}
          <div className="mt-1 flex flex-wrap items-center gap-1.5">
            <SourceChip source={dep.source} />
            <span className={`inline-flex items-center rounded-full px-1.5 py-0.5 text-[10px] font-medium ${
              dep.type === 'required' ? 'bg-blue-50 text-blue-600' : 'bg-gray-100 text-gray-600'
            }`}>
              {dep.type}
            </span>
          </div>
        </div>

        {dep.openURL && (
          <a
            href={dep.openURL}
            target="_blank"
            rel="noopener noreferrer"
            className={`inline-flex shrink-0 items-center gap-1 rounded-lg border px-2.5 py-1.5 text-[11px] font-medium transition-colors ${buttonTone}`}
            title={`Open ${label} on ${sourceLabel(dep.source)}`}
          >
            <span>Open</span>
            <LinkExternal01 width={12} height={12} />
          </a>
        )}
      </div>
    </div>
  );
}

function ConfigItem({ config, onOpen }: { config: ConfigMapping; onOpen: () => void }) {
  return (
    <button
      onClick={onOpen}
      className="w-full flex items-center gap-2 text-sm px-2.5 py-2 rounded-lg bg-gray-50 hover:bg-gray-100 text-left transition-colors"
    >
      <File06 width={14} height={14} className="text-gray-400 shrink-0" />
      <span className="text-gray-800 truncate flex-1">{config.configPath}</span>
      {config.isManual && (
        <span title="Manual override">
          <Link04 width={12} height={12} className="text-blue-500 shrink-0" />
        </span>
      )}
      <ConfidenceBadge confidence={config.confidence} />
    </button>
  );
}

function DependencyIcon({ dep }: { dep: DetailDependency }) {
  if (dep.resolvedIconURL) {
    return <img src={dep.resolvedIconURL} alt="" className="w-5 h-5 rounded shrink-0" />;
  }

  if (dep.resolvedName) {
    return (
      <div className="w-5 h-5 rounded bg-gray-200 flex items-center justify-center shrink-0">
        <span className="text-[8px] font-bold text-gray-500">{dep.resolvedName.charAt(0).toUpperCase()}</span>
      </div>
    );
  }

  return null;
}

function DependencyRow({
  dep,
  providedByCurrentMod,
  onSelectMod,
}: {
  dep: DetailDependency;
  providedByCurrentMod: boolean;
  onSelectMod: (id: string) => void;
}) {
  const isClickable = !!dep.resolvedModID;
  const rowClasses = `w-full flex items-center gap-2 text-sm px-2.5 py-1.5 rounded-lg text-left transition-colors ${
    isClickable ? 'bg-gray-50 hover:bg-gray-100 cursor-pointer' : 'bg-gray-50 cursor-default'
  }`;
  const content = (
    <>
      <DependencyTypeIcon type={dep.type} satisfied={dep.satisfied} />
      <DependencyIcon dep={dep} />
      <div className="min-w-0 flex-1">
        <div className="text-gray-800 truncate">{dep.depName || dep.depModID}</div>
        {providedByCurrentMod && (
          <div className="text-[11px] text-indigo-700">Satisfied by included module in this jar</div>
        )}
        {dep.sources && dep.sources.length > 0 && (
          <div className="mt-1 flex flex-wrap gap-1">
            {dep.sources.map(source => (
              <SourceChip key={source} source={source} />
            ))}
          </div>
        )}
      </div>
      <span className={`text-[10px] px-1.5 py-0.5 rounded-full font-medium ${
        dep.type === 'required' ? 'bg-blue-50 text-blue-600' :
        dep.type === 'optional' ? 'bg-gray-100 text-gray-500' :
        'bg-purple-50 text-purple-600'
      }`}>
        {dep.type}
      </span>
      {isClickable && <ArrowRight width={12} height={12} className="text-gray-400 shrink-0" />}
    </>
  );

  if (isClickable && dep.resolvedModID) {
    return (
      <button onClick={() => onSelectMod(dep.resolvedModID!)} className={rowClasses}>
        {content}
      </button>
    );
  }

  return <div className={rowClasses}>{content}</div>;
}

function SourceChip({ source }: { source: string }) {
  const cls = source === 'manifest'
    ? 'bg-slate-100 text-slate-600'
    : source === 'curseforge'
      ? 'bg-orange-50 text-orange-700'
      : source === 'modrinth'
        ? 'bg-emerald-50 text-emerald-700'
        : 'bg-gray-100 text-gray-600';

  return (
    <span className={`inline-flex items-center rounded-full px-1.5 py-0.5 text-[10px] font-medium ${cls}`}>
      {sourceLabel(source)}
    </span>
  );
}

function sourceLabel(source: string) {
  switch (source) {
    case 'manifest':
      return 'Manifest';
    case 'curseforge':
      return 'CurseForge';
    case 'modrinth':
      return 'Modrinth';
    default:
      return source;
  }
}

function DependencyTypeIcon({ type, satisfied }: { type: string; satisfied: boolean }) {
  const stroke = type === 'required'
    ? (satisfied ? '#2563eb' : '#d97706')
    : '#9ca3af';
  const arrowFill = type === 'required'
    ? (satisfied ? '#2563eb' : '#d97706')
    : '#9ca3af';

  if (type === 'optional') {
    return (
      <svg width="16" height="12" viewBox="0 0 24 12" className="shrink-0" aria-hidden="true">
        <line x1="1" y1="6" x2="16" y2="6" stroke={stroke} strokeWidth="1.5" strokeDasharray="3,3" />
        <polygon points="16,2 23,6 16,10" fill={arrowFill} />
      </svg>
    );
  }

  return (
    <svg width="16" height="12" viewBox="0 0 24 12" className="shrink-0" aria-hidden="true">
      <line x1="1" y1="6" x2="16" y2="6" stroke={stroke} strokeWidth="2" />
      <polygon points="16,1 23,6 16,11" fill={arrowFill} />
    </svg>
  );
}

function ConfidenceBadge({ confidence }: { confidence: number }) {
  let cls = 'bg-gray-100 text-gray-500';
  if (confidence >= 90) cls = 'bg-emerald-50 text-emerald-600';
  else if (confidence >= 50) cls = 'bg-yellow-50 text-yellow-600';
  else if (confidence >= 10) cls = 'bg-orange-50 text-orange-600';

  return (
    <span className={`text-[10px] px-1.5 py-0.5 rounded-full font-medium ${cls}`}>
      {Math.round(confidence)}%
    </span>
  );
}

// --- Mixin helpers & components ---

function groupMixinsByTarget(mixins: MixinDetail[]): Record<string, MixinDetail[]> {
  const groups: Record<string, MixinDetail[]> = {};
  for (const m of mixins) {
    const key = m.targetModID || '_unknown';
    if (!groups[key]) groups[key] = [];
    groups[key].push(m);
  }
  // Sort: "minecraft" first, then named mods, then unknown
  const sorted: Record<string, MixinDetail[]> = {};
  if (groups['minecraft']) { sorted['minecraft'] = groups['minecraft']; delete groups['minecraft']; }
  const keys = Object.keys(groups).filter(k => k !== '_unknown').sort();
  for (const k of keys) sorted[k] = groups[k];
  if (groups['_unknown']) sorted['_unknown'] = groups['_unknown'];
  return sorted;
}

function groupIncomingByOwner(mixins: IncomingMixin[]): Record<string, IncomingMixin[]> {
  const groups: Record<string, IncomingMixin[]> = {};
  for (const m of mixins) {
    if (!groups[m.ownerModID]) groups[m.ownerModID] = [];
    groups[m.ownerModID].push(m);
  }
  return groups;
}

function simpleName(fqn: string): string {
  const parts = fqn.split('.');
  return parts[parts.length - 1] || fqn;
}

function MixinTargetGroup({
  targetModID,
  targetModName,
  mixins,
  onSelectMod,
}: {
  targetModID: string;
  targetModName?: string;
  mixins: MixinDetail[];
  onSelectMod: (id: string) => void;
}) {
  const isClickable = targetModID !== 'minecraft' && targetModID !== '' && targetModID !== '_unknown';
  const label = targetModID === 'minecraft'
    ? 'Minecraft'
    : targetModID === '_unknown'
      ? 'Unknown'
      : (targetModName || targetModID);

  return (
    <div className="rounded-lg border border-gray-100 bg-gray-50/50 p-2">
      <div className="flex items-center gap-2 mb-1.5">
        {isClickable ? (
          <button
            onClick={() => onSelectMod(targetModID)}
            className="text-xs font-semibold text-blue-600 hover:text-blue-800 transition-colors flex items-center gap-1"
          >
            <ArrowRight width={10} height={10} />
            {label}
          </button>
        ) : (
          <span className="text-xs font-semibold text-gray-600">{label}</span>
        )}
        <span className="text-[10px] text-gray-400">{mixins.length} mixin{mixins.length !== 1 ? 's' : ''}</span>
      </div>
      <div className="space-y-1 ml-2">
        {mixins.map((mixin, i) => (
          <MixinRow key={i} mixin={mixin} />
        ))}
      </div>
    </div>
  );
}

function MixinRow({ mixin }: { mixin: MixinDetail | IncomingMixin }) {
  const mixinName = simpleName(mixin.mixinClass);
  const targetName = simpleName(mixin.targetClass);
  const members = mixin.targetMembers ? mixin.targetMembers.split(',').filter(Boolean) : [];

  return (
    <div className="text-xs">
      <div className="flex items-center gap-1.5">
        <span className="font-medium text-gray-700 truncate" title={mixin.mixinClass}>
          {mixinName}
        </span>
        {mixin.targetClass && (
          <>
            <span className="text-gray-400 shrink-0">&rarr;</span>
            <span className="text-gray-600 truncate" title={mixin.targetClass}>
              {targetName}
            </span>
          </>
        )}
      </div>
      {members.length > 0 && (
        <div className="flex flex-wrap gap-1 mt-0.5 ml-3">
          {members.map(member => (
            <span key={member} className="text-[10px] px-1.5 py-0.5 rounded-full bg-violet-50 text-violet-600 font-mono">
              {member}
            </span>
          ))}
        </div>
      )}
    </div>
  );
}

function IncomingMixinGroup({
  ownerModID,
  ownerModName,
  ownerIconURL,
  mixins,
  onSelectMod,
}: {
  ownerModID: string;
  ownerModName: string;
  ownerIconURL?: string;
  mixins: IncomingMixin[];
  onSelectMod: (id: string) => void;
}) {
  return (
    <button
      onClick={() => onSelectMod(ownerModID)}
      className="w-full rounded-lg border border-gray-100 bg-gray-50/50 p-2 text-left hover:bg-gray-100 transition-colors"
    >
      <div className="flex items-center gap-2 mb-1.5">
        {ownerIconURL ? (
          <img src={ownerIconURL} alt="" className="w-5 h-5 rounded shrink-0" />
        ) : (
          <div className="w-5 h-5 rounded bg-gray-200 flex items-center justify-center shrink-0">
            <span className="text-[8px] font-bold text-gray-400">
              {(ownerModName || ownerModID).charAt(0).toUpperCase()}
            </span>
          </div>
        )}
        <span className="text-xs font-semibold text-gray-800">{ownerModName || ownerModID}</span>
        <span className="text-[10px] text-gray-400">{mixins.length} mixin{mixins.length !== 1 ? 's' : ''}</span>
        <ArrowRight width={12} height={12} className="ml-auto text-gray-400 shrink-0" />
      </div>
      <div className="space-y-1 ml-7">
        {mixins.map((mixin, i) => (
          <MixinRow key={i} mixin={mixin} />
        ))}
      </div>
    </button>
  );
}
