import { useCallback } from 'react';
import { Save01, XClose, File06 } from '@untitled-ui/icons-react';
import Editor from '@monaco-editor/react';

interface ConfigEditorProps {
  configPath: string;
  fullPath: string;
  content: string;
  onChange: (value: string) => void;
  onSave: () => void;
  onClose: () => void;
  dirty: boolean;
}

export function ConfigEditor({ configPath, fullPath, content, onChange, onSave, onClose, dirty }: ConfigEditorProps) {
  const language = getLanguageFromPath(configPath);
  const fileName = configPath.split(/[\\/]/).pop() || configPath;

  const handleEditorChange = useCallback((value: string | undefined) => {
    if (value !== undefined) {
      onChange(value);
    }
  }, [onChange]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.ctrlKey && e.key === 's') {
      e.preventDefault();
      onSave();
    }
  }, [onSave]);

  return (
    <div className="flex flex-col h-full bg-white" onKeyDown={handleKeyDown}>
      {/* Toolbar */}
      <div className="flex items-center gap-2 px-3 py-2 bg-gradient-to-r from-emerald-50 via-white to-white border-b border-emerald-100">
        <div className="group relative flex min-w-0 flex-1 items-center gap-2">
          <span className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-lg border border-emerald-200 bg-white text-emerald-700 shadow-sm" title={fullPath || configPath}>
            <File06 width={14} height={14} />
          </span>
          <div className="min-w-0">
            <div className="truncate text-sm font-medium text-slate-800">{fileName}</div>
            <div className="truncate text-[11px] text-slate-500">Config editor</div>
          </div>
          <div className="pointer-events-none absolute left-0 top-full z-10 mt-2 hidden max-w-[520px] rounded-lg border border-slate-800 bg-slate-950 px-3 py-2 text-[11px] leading-relaxed text-slate-100 shadow-xl group-hover:block">
            {fullPath || configPath}
          </div>
        </div>
        {dirty && <span className="text-xs text-emerald-700 font-medium">Unsaved</span>}
        <button
          onClick={onSave}
          disabled={!dirty}
          className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-xs bg-slate-900 text-white hover:bg-slate-800 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
        >
          <Save01 width={12} height={12} /> Save
        </button>
        <button
          onClick={onClose}
          className="p-1 rounded-lg text-slate-400 hover:text-slate-700 hover:bg-slate-200 transition-colors"
        >
          <XClose width={16} height={16} />
        </button>
      </div>

      {/* Editor */}
      <div className="flex-1">
        <Editor
          height="100%"
          language={language}
          value={content}
          onChange={handleEditorChange}
          theme="vs"
          options={{
            fontSize: 13,
            minimap: { enabled: false },
            lineNumbers: 'on',
            scrollBeyondLastLine: false,
            wordWrap: 'on',
            padding: { top: 8 },
          }}
        />
      </div>
    </div>
  );
}

function getLanguageFromPath(path: string): string {
  const ext = path.split('.').pop()?.toLowerCase();
  switch (ext) {
    case 'toml': return 'ini'; // Monaco doesn't have native TOML, ini is close
    case 'json': case 'json5': return 'json';
    case 'yml': case 'yaml': return 'yaml';
    case 'cfg': case 'properties': case 'conf': return 'ini';
    default: return 'plaintext';
  }
}
