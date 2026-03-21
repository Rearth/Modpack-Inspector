import { useEffect, useMemo, useState } from 'react';
import { File06, Link04, AlertTriangle, SearchLg, Plus } from '@untitled-ui/icons-react';
import { GetAllConfigFiles } from '../../wailsjs/go/main/App';
import type { ConfigMapping, ConfigFile } from '../lib/types';

interface ConfigPickerProps {
  configs: ConfigMapping[];
  onSelect: (configPath: string) => void;
  onClose: () => void;
  modName: string;
  modId: string;
}

export function ConfigPicker({ configs, onSelect, onClose, modName }: ConfigPickerProps) {
  const [allConfigs, setAllConfigs] = useState<ConfigFile[]>([]);
  const [query, setQuery] = useState('');
  const [customPath, setCustomPath] = useState('');

  useEffect(() => {
    GetAllConfigFiles().then(files => setAllConfigs(files || [])).catch(console.error);
  }, []);

  const linkedPaths = new Set(configs.map(cfg => cfg.configPath));
  const filteredConfigs = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    const base = allConfigs.filter(file => !linkedPaths.has(file.path));
    if (!normalized) return base.slice(0, 100);
    return base.filter(file => file.path.toLowerCase().includes(normalized) || file.fileName.toLowerCase().includes(normalized)).slice(0, 100);
  }, [allConfigs, linkedPaths, query]);

  return (
    <div className="fixed inset-0 bg-black/30 flex items-center justify-center z-50 backdrop-blur-sm" onClick={onClose}>
      <div className="bg-white border border-gray-200 rounded-xl shadow-xl w-[620px] max-h-[85vh] overflow-hidden"
        onClick={e => e.stopPropagation()}>
        <div className="p-4 border-b border-gray-200">
          <h3 className="text-gray-900 font-semibold">Link Config File</h3>
          <p className="text-sm text-gray-500 mt-1">
            Open listed configs directly from the detail view. Use this dialog to link a different file for <span className="text-gray-900 font-medium">{modName}</span>.
          </p>
        </div>

        <div className="overflow-auto max-h-[65vh] p-3 space-y-4">
          {configs.length > 0 && (
            <div>
              <div className="text-[11px] text-gray-500 uppercase tracking-wider font-semibold mb-2">Already linked</div>
              <div className="space-y-1">
                {configs.map((cfg, i) => (
                  <div key={i} className="flex items-center gap-2 px-3 py-2 rounded-lg bg-gray-50">
                    <File06 width={14} height={14} className="text-gray-500 shrink-0" />
                    <span className="text-sm text-gray-800 truncate flex-1">{cfg.configPath}</span>
                    {cfg.isManual && <Link04 width={12} height={12} className="text-blue-600 shrink-0" />}
                  </div>
                ))}
              </div>
            </div>
          )}

          <div>
            <div className="text-[11px] text-gray-500 uppercase tracking-wider font-semibold mb-2">Search existing config files</div>
            <div className="flex items-center gap-2 px-3 py-2 rounded-lg border border-gray-200 bg-white mb-2">
              <SearchLg width={14} height={14} className="text-gray-500 shrink-0" />
              <input
                value={query}
                onChange={e => setQuery(e.target.value)}
                placeholder="Search by file name or path..."
                className="flex-1 text-sm text-gray-800 placeholder:text-gray-400 outline-none"
              />
            </div>
            <div className="space-y-1 max-h-64 overflow-auto rounded-lg border border-gray-100 p-1 bg-gray-50/50">
              {filteredConfigs.map(file => (
                <button
                  key={file.path}
                  onClick={() => onSelect(file.path)}
                  className="w-full flex items-center gap-2 px-3 py-2 rounded-lg hover:bg-white text-left transition-colors"
                >
                  <File06 width={14} height={14} className="text-gray-500 shrink-0" />
                  <div className="min-w-0 flex-1">
                    <div className="text-sm text-gray-800 truncate">{file.fileName}</div>
                    <div className="text-[11px] text-gray-600 truncate">{file.path}</div>
                  </div>
                </button>
              ))}
              {filteredConfigs.length === 0 && (
                <div className="px-3 py-4 text-sm text-gray-500">No unlinked config files match.</div>
              )}
            </div>
          </div>

          <div>
            <div className="text-[11px] text-gray-500 uppercase tracking-wider font-semibold mb-2">Custom relative path</div>
            <div className="flex gap-2">
              <input
                value={customPath}
                onChange={e => setCustomPath(e.target.value)}
                placeholder="examplemod/config.toml"
                className="flex-1 px-3 py-2 rounded-lg border border-gray-200 text-sm text-gray-800 placeholder:text-gray-400 outline-none focus:border-gray-400"
              />
              <button
                onClick={() => customPath.trim() && onSelect(customPath.trim())}
                className="inline-flex items-center gap-1 px-3 py-2 rounded-lg bg-gray-900 text-white text-sm hover:bg-gray-800 transition-colors"
              >
                <Plus width={14} height={14} /> Link
              </button>
            </div>
            <div className="flex items-center gap-2 px-3 py-2 mt-2 rounded-lg bg-amber-50 text-amber-800 text-xs">
              <AlertTriangle width={14} height={14} />
              <span>If the file does not exist yet, it will open as a new editable config inside the pack config directory.</span>
            </div>
          </div>
        </div>

        <div className="p-3 border-t border-gray-200 flex justify-end">
          <button onClick={onClose} className="px-3 py-1.5 rounded-lg text-sm text-gray-500 hover:text-gray-900 hover:bg-gray-100 transition-colors">
            Cancel
          </button>
        </div>
      </div>
    </div>
  );
}
