import { Folder } from '@untitled-ui/icons-react';

export function LauncherRootField({
  label,
  value,
  onChange,
  onBrowse,
  placeholder,
  helper,
  defaults,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  onBrowse: () => void;
  placeholder: string;
  helper: string;
  defaults: string[];
}) {
  return (
    <label className="block">
      <span className="text-xs text-gray-500">{label}</span>
      <div className="mt-1 flex gap-2">
        <input
          type="text"
          value={value}
          onChange={e => onChange(e.target.value)}
          placeholder={placeholder}
          className="flex-1 rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm text-gray-900 placeholder-gray-400 focus:outline-none focus:border-gray-400 focus:ring-1 focus:ring-gray-400"
        />
        <button
          type="button"
          onClick={onBrowse}
          className="rounded-lg border border-gray-200 px-3 py-2 text-gray-600 transition-colors hover:bg-gray-50"
          title="Browse for folder"
        >
          <Folder width={16} height={16} />
        </button>
      </div>
      <div className="mt-1 space-y-1">
        <p className="text-[11px] text-gray-500">{helper}</p>
        <p className="text-[11px] text-gray-400">Default: {defaults.join(' or ')}</p>
      </div>
    </label>
  );
}