import { useEffect, useState } from 'react';
import { Environment, Quit, WindowIsMaximised, WindowMinimise, WindowToggleMaximise } from '../../wailsjs/runtime/runtime';
import appIcon from '../assets/appicon.png';

interface TitleBarProps {
  instanceName: string;
}

function isWailsRuntimeAvailable() {
  return typeof window !== 'undefined' && typeof (window as any).runtime !== 'undefined';
}

export function TitleBar({ instanceName }: TitleBarProps) {
  const [isDesktop, setIsDesktop] = useState(false);
  const [isMaximised, setIsMaximised] = useState(false);

  useEffect(() => {
    if (!isWailsRuntimeAvailable()) {
      return;
    }

    setIsDesktop(true);
    Environment()
      .then(info => setIsDesktop(info.platform === 'windows'))
      .catch(() => setIsDesktop(true));

    WindowIsMaximised()
      .then(setIsMaximised)
      .catch(() => setIsMaximised(false));
  }, []);

  const handleToggleMaximise = () => {
    if (!isWailsRuntimeAvailable()) {
      return;
    }
    WindowToggleMaximise();
    WindowIsMaximised()
      .then(setIsMaximised)
      .catch(() => setIsMaximised(false));
  };

  return (
    <div className="relative h-12 shrink-0 border-b border-emerald-100/80 bg-white/92 backdrop-blur-sm select-none">
      <div className="absolute inset-x-0 top-0 h-[2px] bg-gradient-to-r from-emerald-400 via-teal-400 to-cyan-400" />
      <div className="flex h-full items-center justify-between gap-3 pl-4 pr-2">
        <div
          className="flex min-w-0 flex-1 items-center gap-3"
          style={{ ['--wails-draggable' as string]: 'drag' }}
          onDoubleClick={handleToggleMaximise}
        >
          <img src={appIcon} alt="Modpack Inspector" className="h-8 w-8 rounded-xl shadow-sm ring-1 ring-emerald-200" />
          <div className="min-w-0">
            <div className="truncate text-sm font-semibold text-slate-900">Modpack Inspector</div>
            <div className="truncate text-[11px] text-slate-500">{instanceName || 'Minecraft modpack analysis'}</div>
          </div>
        </div>

        {isDesktop && (
          <div className="flex items-center gap-1">
            <button
              type="button"
              onClick={() => WindowMinimise()}
              className="flex h-8 w-8 items-center justify-center rounded-lg text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-700"
              title="Minimize"
            >
              <svg width="12" height="12" viewBox="0 0 12 12" fill="none" aria-hidden="true">
                <path d="M2 6.5H10" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
              </svg>
            </button>
            <button
              type="button"
              onClick={handleToggleMaximise}
              className="flex h-8 w-8 items-center justify-center rounded-lg text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-700"
              title={isMaximised ? 'Restore' : 'Maximize'}
            >
              {isMaximised ? (
                <svg width="12" height="12" viewBox="0 0 12 12" fill="none" aria-hidden="true">
                  <path d="M4 2.5H9.5V8" stroke="currentColor" strokeWidth="1.2" />
                  <path d="M2.5 4H8V9.5H2.5V4Z" stroke="currentColor" strokeWidth="1.2" />
                </svg>
              ) : (
                <svg width="12" height="12" viewBox="0 0 12 12" fill="none" aria-hidden="true">
                  <rect x="2.5" y="2.5" width="7" height="7" stroke="currentColor" strokeWidth="1.2" />
                </svg>
              )}
            </button>
            <button
              type="button"
              onClick={() => Quit()}
              className="flex h-8 w-8 items-center justify-center rounded-lg text-slate-400 transition-colors hover:bg-red-500 hover:text-white"
              title="Close"
            >
              <svg width="12" height="12" viewBox="0 0 12 12" fill="none" aria-hidden="true">
                <path d="M3 3L9 9" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
                <path d="M9 3L3 9" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
              </svg>
            </button>
          </div>
        )}
      </div>
    </div>
  );
}