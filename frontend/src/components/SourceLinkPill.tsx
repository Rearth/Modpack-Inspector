import curseforgeIcon from '../assets/curseforge_icon.png';
import modrinthIcon from '../assets/modrinth_icon.png';
import { LinkExternal01 } from '@untitled-ui/icons-react';

type SourceTone = 'curseforge' | 'modrinth';

export function SourceLinkPill({
  href,
  label,
  shortLabel,
  tone,
  compact = false,
  showShortLabel = true,
  onClick,
}: {
  href: string;
  label: string;
  shortLabel: string;
  tone: SourceTone;
  compact?: boolean;
  showShortLabel?: boolean;
  onClick?: React.MouseEventHandler<HTMLAnchorElement>;
}) {
  const toneClasses = tone === 'curseforge'
    ? 'border-orange-200 bg-orange-50 text-orange-700 hover:bg-orange-100'
    : 'border-emerald-200 bg-emerald-50 text-emerald-700 hover:bg-emerald-100';
  const sizeClasses = compact
    ? 'rounded-full px-1.5 py-0.5 text-[10px] font-semibold gap-1'
    : 'rounded-lg px-2.5 py-1.5 text-xs font-medium gap-1.5';

  return (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      title={label}
      onClick={onClick}
      className={`inline-flex items-center border transition-colors ${toneClasses} ${sizeClasses}`}
    >
      <ServiceIcon tone={tone} compact={compact} />
      {showShortLabel && <span>{shortLabel}</span>}
      {!compact && <span>{label}</span>}
      <LinkExternal01 width={compact ? 10 : 12} height={compact ? 10 : 12} />
    </a>
  );
}

function ServiceIcon({ tone, compact }: { tone: SourceTone; compact: boolean }) {
  const size = compact ? 'h-3 w-3' : 'h-3.5 w-3.5';
  const src = tone === 'curseforge' ? curseforgeIcon : modrinthIcon;
  const alt = tone === 'curseforge' ? 'CurseForge' : 'Modrinth';

  return (
    <img src={src} alt={alt} className={`${size} shrink-0 rounded-sm`} />
  );
}