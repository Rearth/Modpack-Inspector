import { AlertTriangle } from '@untitled-ui/icons-react';

export function AccuracyNotice() {
  return (
    <div className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900 overflow-hidden">
      <div className="flex items-start gap-3">
        <AlertTriangle width={16} height={16} className="mt-0.5 shrink-0 text-amber-700" />
        <div className="min-w-0 space-y-1">
          <div className="font-semibold">Best-effort analysis</div>
          <p className="text-amber-800">
            ModpackTool combines dependencies from each mod jar&apos;s own manifest files such as{' '}
            <span className="font-mono text-[12px] break-all">fabric.mod.json</span>,{' '}
            <span className="font-mono text-[12px] break-all">quilt.mod.json</span>,{' '}
            <span className="font-mono text-[12px] break-all">mods.toml</span>, and{' '}
            <span className="font-mono text-[12px] break-all">neoforge.mods.toml</span>, plus matched version or file dependency data from Modrinth and CurseForge when those lookups succeed. It then resolves them against installed mod IDs and provided or embedded module IDs.
          </p>
          <p className="text-amber-800">
            CurseForge and Modrinth are also used to enrich metadata like icons, categories, links, and long descriptions for search. Broken manifests, missing API matches, bundled mods, and custom setups can still make dependency results incomplete or wrong.
          </p>
        </div>
      </div>
    </div>
  );
}