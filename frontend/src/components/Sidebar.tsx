import { Package, RefreshCw05, Settings02, Share07 } from '@untitled-ui/icons-react';
import type { View } from '../lib/types';

interface SidebarProps {
  activeView: View;
  onViewChange: (view: View) => void;
  onScan: () => void;
  scanStatus: string;
}

const navItems: { view: View; label: string; icon: typeof Package }[] = [
  { view: 'mods', label: 'Mods', icon: Package },
  { view: 'graph', label: 'Graph', icon: Share07 },
  { view: 'settings', label: 'Settings', icon: Settings02 },
];

export function Sidebar({ activeView, onViewChange, onScan, scanStatus }: SidebarProps) {
  return (
    <aside className="w-[64px] shrink-0 border-r border-emerald-100/80 bg-white/88 backdrop-blur-sm flex flex-col items-center py-3 gap-1">
      {navItems.map(({ view, label, icon: Icon }) => (
        <button
          key={view}
          onClick={() => onViewChange(view)}
          className={`relative w-11 h-11 rounded-xl flex flex-col items-center justify-center gap-0.5 transition-colors text-xs ${
            activeView === view
              ? 'bg-emerald-50 text-emerald-700 ring-1 ring-emerald-200'
              : 'text-slate-500 hover:text-slate-900 hover:bg-slate-100'
          }`}
          title={label}
        >
          {activeView === view && <span className="absolute -left-1 top-1/2 h-6 w-1 -translate-y-1/2 rounded-full bg-gradient-to-b from-emerald-400 to-teal-400" />}
          <Icon width={18} height={18} />
          <span className="text-[10px] font-medium leading-none">{label}</span>
        </button>
      ))}

      <div className="flex-1" />

      <button
        onClick={onScan}
        className="w-11 h-11 rounded-xl flex flex-col items-center justify-center gap-0.5 text-xs text-emerald-700 hover:text-emerald-800 hover:bg-emerald-50 transition-colors disabled:opacity-70 ring-1 ring-transparent hover:ring-emerald-200"
        title={scanStatus || 'Rescan mods. Usually not needed: ModpackTool auto-watches file changes.'}
        disabled={!!scanStatus}
      >
        <RefreshCw05 width={18} height={18} className={scanStatus ? 'animate-spin' : ''} />
        <span className="text-[10px] font-medium leading-none">Scan</span>
      </button>
    </aside>
  );
}
