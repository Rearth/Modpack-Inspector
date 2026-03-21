import { AlertTriangle, BookOpen01 } from '@untitled-ui/icons-react';
import type { Mod } from '../lib/types';
import { SourceLinkPill } from './SourceLinkPill';

interface ModListProps {
  mods: Mod[];
  selectedModId: string | null;
  onSelectMod: (id: string) => void;
  unusedLibraries: string[];
  scanStatus: string;
  onCategoryFilter: (cat: string) => void;
  onDetectedLibraryFilter: () => void;
  categoryFilter: string;
  detectedLibraryFilter: boolean;
}

export function ModList({ mods, selectedModId, onSelectMod, unusedLibraries, scanStatus, onCategoryFilter, onDetectedLibraryFilter, categoryFilter, detectedLibraryFilter }: ModListProps) {
  if (mods.length === 0 && !scanStatus) {
    return (
      <div className="flex items-center justify-center h-full text-gray-400">
        <p className="text-sm">No mods found. Configure an instance path in Settings.</p>
      </div>
    );
  }

  const unusedSet = new Set(unusedLibraries);

  return (
    <div className="overflow-auto h-full px-3 py-2">
      <div className="space-y-2">
        {mods.map(mod => {
          const isUnused = unusedSet.has(mod.id);
          const isLibrary = mod.isLibrary;
          const isForcedLibrary = mod.libraryOverride === 1;
          const isSelected = selectedModId === mod.id;
          const desc = mod.onlineDesc || mod.description || '';
          const categories = mod.categories ? mod.categories.split(',').map(c => c.trim()).filter(Boolean) : [];
          const loaders = getLoaders(mod);

          return (
            <div
              key={mod.id}
              onClick={() => onSelectMod(mod.id)}
              className={`rounded-lg border cursor-pointer transition-all
                ${isSelected
                  ? 'bg-gray-50 border-gray-300 shadow-sm'
                  : 'bg-white border-gray-200 hover:border-gray-300 hover:shadow-sm'}
                ${isUnused ? 'opacity-60' : ''}`}
            >
              <div className="flex items-start gap-3 px-3 py-2.5">
                {/* Icon */}
                {mod.iconURL ? (
                  <img src={mod.iconURL} alt="" className="w-10 h-10 rounded-lg shrink-0 mt-0.5" />
                ) : (
                  <div className="w-10 h-10 rounded-lg bg-gray-100 flex items-center justify-center shrink-0 mt-0.5">
                    <span className="text-sm font-semibold text-gray-400">
                      {(mod.name || mod.id).charAt(0).toUpperCase()}
                    </span>
                  </div>
                )}

                {/* Content */}
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-1.5 flex-wrap">
                    <span className="font-medium text-sm text-gray-900 truncate">{mod.name || mod.id}</span>
                    <span className="text-xs text-gray-400 font-mono">{mod.version}</span>
                    {isLibrary && (
                      <button
                        onClick={(e) => { e.stopPropagation(); onDetectedLibraryFilter(); }}
                        className={`inline-flex items-center gap-0.5 text-[10px] px-1.5 py-0.5 rounded-full shrink-0 font-medium transition-colors ${
                          detectedLibraryFilter
                            ? 'bg-slate-300 text-slate-900'
                            : 'bg-slate-100 text-slate-600 hover:bg-slate-200'
                        }`}
                        title="Filter detected libraries"
                      >
                        <BookOpen01 width={10} height={10} />
                        Detected Library
                      </button>
                    )}
                    {isForcedLibrary && (
                      <span
                        className="inline-flex items-center gap-0.5 text-[10px] px-1.5 py-0.5 rounded-full shrink-0 font-medium bg-blue-50 text-blue-700"
                        title="Manually forced as library"
                      >
                        <BookOpen01 width={10} height={10} />
                        Forced
                      </span>
                    )}
                    {isUnused && (
                      <span className="inline-flex items-center gap-0.5 text-[10px] px-1.5 py-0.5 rounded-full bg-amber-50 text-amber-600 shrink-0 font-medium" title="Unused — no mods depend on this library">
                        <AlertTriangle width={10} height={10} />
                        Unused
                      </span>
                    )}
                  </div>
                  {desc && (
                    <p className="text-xs text-gray-500 mt-0.5 line-clamp-2 break-words">{desc}</p>
                  )}
                  {/* Tags row: categories + loaders + links */}
                  <div className="flex items-center gap-1 mt-1.5 flex-wrap">
                    {categories.slice(0, 3).map(cat => (
                      <button
                        key={cat}
                        onClick={(e) => { e.stopPropagation(); onCategoryFilter(cat); }}
                        className={`text-[10px] px-1.5 py-0.5 rounded-full font-medium transition-colors
                          ${categoryFilter === cat
                            ? 'bg-blue-100 text-blue-700'
                            : 'bg-gray-100 text-gray-500 hover:bg-blue-50 hover:text-blue-600'}`}
                      >
                        {cat}
                      </button>
                    ))}
                    {loaders.map(loader => (
                      <LoaderBadge key={loader} loader={loader} />
                    ))}
                    {mod.modrinthURL && (
                      <SourceLinkPill
                        href={mod.modrinthURL}
                        label="Modrinth"
                        shortLabel="MR"
                        tone="modrinth"
                        compact
                        showShortLabel={false}
                        onClick={e => e.stopPropagation()}
                      />
                    )}
                    {mod.curseForgeURL && (
                      <SourceLinkPill
                        href={mod.curseForgeURL}
                        label="CurseForge"
                        shortLabel="CF"
                        tone="curseforge"
                        compact
                        showShortLabel={false}
                        onClick={e => e.stopPropagation()}
                      />
                    )}
                  </div>
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function getLoaders(mod: Mod): string[] {
  if (mod.loaders) {
    return mod.loaders.split(',').filter(Boolean);
  }
  return mod.modLoader ? [mod.modLoader] : [];
}

function LoaderBadge({ loader }: { loader: string }) {
  const colors: Record<string, string> = {
    forge: 'bg-orange-50 text-orange-600',
    neoforge: 'bg-orange-50 text-orange-600',
    fabric: 'bg-indigo-50 text-indigo-600',
    quilt: 'bg-purple-50 text-purple-600',
  };
  const cls = colors[loader] || 'bg-gray-100 text-gray-600';
  return (
    <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${cls}`}>
      {loader}
    </span>
  );
}
