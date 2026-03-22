import { Fragment, useCallback, useDeferredValue, useEffect, useMemo, useRef, useState } from 'react';
import { Activity, AlertTriangle, Bug, ChevronDown, ChevronUp, CircleAlert, Eraser, FileText, Info, RefreshCw, RotateCcw, Search, X } from 'lucide-react';
import { ClearLatestLogContent, GetLogsOverview, ReadInstanceTextFile, StartLiveLog, StopLiveLog } from '../../wailsjs/go/main/App';
import { EventsOff, EventsOn } from '../../wailsjs/runtime/runtime';
import type { LiveLogChunk, LogsOverview, TextFileContent } from '../lib/types';

interface LogsPanelProps {
  instanceName: string;
}

type ViewerKind = 'log' | 'crash';
type LevelBucket = 'debug' | 'info' | 'warn' | 'error';

type ParsedLogLine = {
  raw: string;
  timestamp?: string;
  channel?: string;
  source?: string;
  level?: string;
  bucket?: LevelBucket;
  message: string;
  kind: 'structured' | 'continuation' | 'comment' | 'plain';
};

type MatchRange = { start: number; end: number; globalIndex: number };

type RenderedLine = {
  lineIndex: number;
  parsed: ParsedLogLine;
  tone: ReturnType<typeof levelTone>;
  timestampRanges: MatchRange[];
  channelRanges: MatchRange[];
  sourceRanges: MatchRange[];
  messageRanges: MatchRange[];
  rawRanges: MatchRange[];
};

const defaultLevelVisibility: Record<LevelBucket, boolean> = {
  debug: true,
  info: true,
  warn: true,
  error: true,
};

function formatTimestamp(unix: number) {
  if (!unix) return 'Unknown time';
  return new Date(unix * 1000).toLocaleString();
}

function formatSize(size: number) {
  if (!Number.isFinite(size) || size <= 0) return '0 B';
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}

function isWailsRuntimeAvailable() {
  return typeof window !== 'undefined' && typeof (window as any).go?.main?.App !== 'undefined';
}

function escapeRegExp(value: string) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function normalizeLineEndings(content: string) {
  return content.replace(/\r\n/g, '\n');
}

function toLevelBucket(level?: string): LevelBucket | undefined {
  switch ((level || '').toUpperCase()) {
    case 'TRACE':
    case 'DEBUG':
      return 'debug';
    case 'WARN':
      return 'warn';
    case 'ERROR':
    case 'FATAL':
      return 'error';
    case 'INFO':
      return 'info';
    default:
      return undefined;
  }
}

function levelTone(bucket?: LevelBucket) {
  switch (bucket) {
    case 'error':
      return {
        badge: 'bg-rose-200/90 text-rose-950 ring-1 ring-rose-300/70',
        row: 'bg-rose-400/10 hover:bg-rose-300/12',
        text: 'text-slate-50',
      };
    case 'warn':
      return {
        badge: 'bg-amber-200/90 text-amber-950 ring-1 ring-amber-300/70',
        row: 'bg-amber-300/10 hover:bg-amber-300/14',
        text: 'text-slate-50',
      };
    case 'debug':
      return {
        badge: 'bg-sky-200/90 text-sky-950 ring-1 ring-sky-300/70',
        row: 'bg-sky-300/10 hover:bg-sky-300/14',
        text: 'text-slate-100',
      };
    default:
      return {
        badge: 'bg-emerald-200/90 text-emerald-950 ring-1 ring-emerald-300/70',
        row: 'bg-white/4 hover:bg-white/7',
        text: 'text-slate-50',
      };
  }
}

function levelFilterTone(bucket: LevelBucket) {
  switch (bucket) {
    case 'debug':
      return {
        dot: 'bg-sky-500',
        chip: 'bg-sky-50 text-sky-700 ring-sky-200',
      };
    case 'warn':
      return {
        dot: 'bg-amber-500',
        chip: 'bg-amber-50 text-amber-700 ring-amber-200',
      };
    case 'error':
      return {
        dot: 'bg-rose-500',
        chip: 'bg-rose-50 text-rose-700 ring-rose-200',
      };
    default:
      return {
        dot: 'bg-emerald-500',
        chip: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
      };
  }
}

function levelFilterIcon(bucket: LevelBucket) {
  switch (bucket) {
    case 'debug':
      return Bug;
    case 'warn':
      return AlertTriangle;
    case 'error':
      return CircleAlert;
    default:
      return Info;
  }
}

function classifyCrashLine(line: string) {
  if (!line.trim()) {
    return 'blank' as const;
  }
  if (/^--+/.test(line) || /^A detailed walkthrough/.test(line)) {
    return 'heading' as const;
  }
  if (/^(Caused by:|Suppressed:)/.test(line)) {
    return 'cause' as const;
  }
  if (/^\s+at\s/.test(line) || /^\s*\.\.\.\s+\d+ more$/.test(line)) {
    return 'trace' as const;
  }
  if (/^[A-Z][A-Za-z0-9 /()_-]+:$/.test(line)) {
    return 'section' as const;
  }
  return 'plain' as const;
}

function parseLogLine(line: string): ParsedLogLine {
  const structured = line.match(/^\[(\d{2}:\d{2}:\d{2})\]\s+\[([^\]]+)\](?:\s+\[([^\]]+?)\])?(?:[:\s]+)(.*)$/);
  if (structured) {
    const channel = structured[2] || '';
    const sourcePart = structured[3] || '';
    const channelParts = channel.split('/');
    const sourceParts = sourcePart.split('/');
    const levelCandidate = [...channelParts, ...sourceParts].find(part => /^(TRACE|DEBUG|INFO|WARN|ERROR|FATAL)$/i.test(part));

    return {
      raw: line,
      timestamp: structured[1],
      channel,
      source: sourcePart || undefined,
      level: levelCandidate?.toUpperCase() || undefined,
      bucket: toLevelBucket(levelCandidate?.toUpperCase()),
      message: structured[4] || '',
      kind: 'structured',
    };
  }

  if (/^#/.test(line)) {
    return { raw: line, message: line, kind: 'comment' };
  }
  if (/^\s*[-|\\]/.test(line) || /^\t/.test(line)) {
    return { raw: line, message: line, kind: 'continuation' };
  }
  return { raw: line, message: line, kind: 'plain' };
}

function buildMatchRanges(text: string, query: string, startIndex: number) {
  if (!query) {
    return { ranges: [] as MatchRange[], nextIndex: startIndex };
  }
  const regex = new RegExp(escapeRegExp(query), 'ig');
  const ranges: MatchRange[] = [];
  let match: RegExpExecArray | null;
  let currentIndex = startIndex;
  while ((match = regex.exec(text)) !== null) {
    ranges.push({
      start: match.index,
      end: match.index + match[0].length,
      globalIndex: currentIndex,
    });
    currentIndex += 1;
    if (match.index === regex.lastIndex) {
      regex.lastIndex += 1;
    }
  }
  return { ranges, nextIndex: currentIndex };
}

function highlightFragments(
  text: string,
  ranges: MatchRange[],
  activeMatchIndex: number,
  getMatchRef?: (index: number) => (node: HTMLElement | null) => void,
) {
  if (ranges.length === 0) {
    return [<Fragment key="plain">{text || ' '}</Fragment>];
  }
  const fragments: JSX.Element[] = [];
  let cursor = 0;
  ranges.forEach(range => {
    if (cursor < range.start) {
      fragments.push(<Fragment key={`text-${cursor}`}>{text.slice(cursor, range.start)}</Fragment>);
    }
    fragments.push(
      <mark
        key={`match-${range.globalIndex}-${range.start}`}
        ref={getMatchRef?.(range.globalIndex)}
        data-match-index={range.globalIndex}
        className={range.globalIndex === activeMatchIndex
          ? 'rounded bg-cyan-300/95 px-0.5 text-slate-950 shadow-[0_0_0_1px_rgba(34,211,238,0.45)]'
          : 'rounded bg-amber-200/85 px-0.5 text-slate-950'}
      >
        {text.slice(range.start, range.end)}
      </mark>,
    );
    cursor = range.end;
  });
  if (cursor < text.length) {
    fragments.push(<Fragment key={`text-tail-${cursor}`}>{text.slice(cursor)}</Fragment>);
  }
  return fragments;
}

export function LogsPanel({ instanceName }: LogsPanelProps) {
  const [overview, setOverview] = useState<LogsOverview | null>(null);
  const [viewer, setViewer] = useState<TextFileContent | null>(null);
  const [selectedLogPath, setSelectedLogPath] = useState('');
  const [selectedCrashPath, setSelectedCrashPath] = useState('');
  const [viewerMode, setViewerMode] = useState<ViewerKind>('log');
  const [live, setLive] = useState(false);
  const [autoScroll, setAutoScroll] = useState(true);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState('');
  const [searchQuery, setSearchQuery] = useState('');
  const [activeMatchIndex, setActiveMatchIndex] = useState(0);
  const [clearingLog, setClearingLog] = useState(false);
  const [levelVisibility, setLevelVisibility] = useState(defaultLevelVisibility);
  const [levelMenuOpen, setLevelMenuOpen] = useState(false);
  const scrollerRef = useRef<HTMLDivElement | null>(null);
  const levelMenuRef = useRef<HTMLDivElement | null>(null);
  const requestSequenceRef = useRef(0);
  const matchElementsRef = useRef(new Map<number, HTMLElement>());
  const deferredSearchQuery = useDeferredValue(searchQuery);

  const registerMatchElement = useCallback(
    (matchIndex: number) => (node: HTMLElement | null) => {
      if (node) {
        matchElementsRef.current.set(matchIndex, node);
        return;
      }
      matchElementsRef.current.delete(matchIndex);
    },
    [],
  );

  const applyViewer = useCallback((next: TextFileContent, mode: ViewerKind) => {
    matchElementsRef.current.clear();
    setViewer(next);
    setViewerMode(mode);
    setError('');
  }, []);

  const stopLiveStream = useCallback(async () => {
    if (!isWailsRuntimeAvailable()) {
      setLive(false);
      return;
    }
    try {
      await StopLiveLog();
    } catch (err) {
      console.error('Failed to stop live log stream:', err);
    } finally {
      setLive(false);
    }
  }, []);

  const openCrash = useCallback(async (relativePath: string) => {
    if (!relativePath || !isWailsRuntimeAvailable()) return;
    const requestId = ++requestSequenceRef.current;
    setSelectedCrashPath(relativePath);
    setError('');
    if (live) {
      await stopLiveStream();
    }
    try {
      const next = await ReadInstanceTextFile(relativePath);
      if (requestId !== requestSequenceRef.current) {
        return;
      }
      applyViewer(next, 'crash');
      setLive(false);
    } catch (err) {
      if (requestId !== requestSequenceRef.current) {
        return;
      }
      console.error(`Failed to read ${relativePath}:`, err);
      setError(`Failed to read ${relativePath}.`);
    }
  }, [applyViewer, live, stopLiveStream]);

  const openLog = useCallback(async (relativePath: string, nextLive: boolean) => {
    if (!relativePath || !isWailsRuntimeAvailable()) return;
    const requestId = ++requestSequenceRef.current;
    setSelectedLogPath(relativePath);
    setError('');

    try {
      if (live) {
        await stopLiveStream();
      }
      if (requestId !== requestSequenceRef.current) {
        return;
      }

      const next = nextLive
        ? await StartLiveLog(relativePath)
        : await ReadInstanceTextFile(relativePath);
      if (requestId !== requestSequenceRef.current) {
        if (nextLive) {
          await StopLiveLog();
        }
        return;
      }

      applyViewer(next, 'log');
      setLive(nextLive);
    } catch (err) {
      if (requestId !== requestSequenceRef.current) {
        return;
      }
      console.error(`Failed to open ${relativePath}:`, err);
      setError(nextLive ? `Failed to start live stream for ${relativePath}.` : `Failed to read ${relativePath}.`);
      setLive(false);
    }
  }, [applyViewer, live, stopLiveStream]);

  const refreshOverview = useCallback(async (preserveSelection = true) => {
    if (!isWailsRuntimeAvailable()) {
      setOverview({
        available: false,
        defaultLiveLog: 'logs/latest.log',
        latestCrash: undefined,
        crashReports: [],
        logFiles: [],
      });
      setLoading(false);
      return;
    }

    setRefreshing(true);
    try {
      const data = await GetLogsOverview();
      setOverview(data);

      const nextLogPath = preserveSelection && data.logFiles.some(file => file.relativePath === selectedLogPath)
        ? selectedLogPath
        : data.logFiles.find(file => file.relativePath === data.defaultLiveLog)?.relativePath || data.logFiles[0]?.relativePath || data.defaultLiveLog;
      const nextCrashPath = preserveSelection && data.crashReports.some(file => file.relativePath === selectedCrashPath)
        ? selectedCrashPath
        : data.latestCrash?.relativePath || data.crashReports[0]?.relativePath || '';

      setSelectedLogPath(nextLogPath);
      setSelectedCrashPath(nextCrashPath);

      const currentPath = viewer?.relativePath || '';
      if (!currentPath) {
        if (nextLogPath) {
          await openLog(nextLogPath, false);
        }
      } else if (viewerMode === 'crash' && currentPath !== nextCrashPath && !data.crashReports.some(file => file.relativePath === currentPath)) {
        if (nextCrashPath) {
          await openCrash(nextCrashPath);
        } else {
          setViewer(null);
        }
      } else if (viewerMode === 'log' && currentPath !== nextLogPath && currentPath !== data.defaultLiveLog && !data.logFiles.some(file => file.relativePath === currentPath)) {
        if (nextLogPath) {
          await openLog(nextLogPath, live);
        } else {
          setViewer(null);
        }
      }

      setError('');
    } catch (err) {
      console.error('Failed to load logs overview:', err);
      setError('Failed to load instance logs.');
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, [live, openCrash, openLog, selectedCrashPath, selectedLogPath, viewer?.relativePath, viewerMode]);

  const openLatestCrash = useCallback(async () => {
    if (!overview?.latestCrash?.relativePath) return;
    await openCrash(overview.latestCrash.relativePath);
  }, [openCrash, overview?.latestCrash?.relativePath]);

  const toggleLiveMode = useCallback(async (nextLive: boolean) => {
    const logPath = selectedLogPath || overview?.defaultLiveLog;
    if (!logPath) {
      return;
    }
    await openLog(logPath, nextLive);
  }, [openLog, overview?.defaultLiveLog, selectedLogPath]);

  const clearLatestLog = useCallback(async () => {
    if (!selectedLogPath || selectedLogPath !== overview?.defaultLiveLog || !isWailsRuntimeAvailable()) {
      return;
    }
    setClearingLog(true);
    try {
      const wasLive = live;
      if (wasLive) {
        await stopLiveStream();
      }
      const next = await ClearLatestLogContent(selectedLogPath);
      applyViewer(next, 'log');
      if (wasLive) {
        await openLog(selectedLogPath, true);
      } else {
        await refreshOverview();
      }
    } catch (err) {
      console.error(`Failed to clear ${selectedLogPath}:`, err);
      setError(`Failed to clear ${selectedLogPath}.`);
    } finally {
      setClearingLog(false);
    }
  }, [applyViewer, live, openLog, overview?.defaultLiveLog, refreshOverview, selectedLogPath, stopLiveStream]);

  useEffect(() => {
    void refreshOverview(false);
  }, [instanceName]);

  useEffect(() => {
    if (!isWailsRuntimeAvailable()) {
      return;
    }

    const handleAppend = (chunk: LiveLogChunk) => {
      setViewer(current => {
        if (!current || current.relativePath !== chunk.relativePath) {
          return current;
        }
        return {
          ...current,
          content: current.content + chunk.content,
          totalSize: chunk.totalSize,
          modifiedUnix: chunk.modifiedUnix,
          missing: false,
        };
      });
    };

    const handleReset = (snapshot: TextFileContent) => {
      setViewer(current => {
        if (current && current.relativePath !== snapshot.relativePath) {
          return current;
        }
        return snapshot;
      });
    };

    const handleStopped = () => {
      setLive(false);
    };

    EventsOn('logs:append', handleAppend);
    EventsOn('logs:reset', handleReset);
    EventsOn('logs:stopped', handleStopped);

    return () => {
      EventsOff('logs:append');
      EventsOff('logs:reset');
      EventsOff('logs:stopped');
      void stopLiveStream();
    };
  }, [stopLiveStream]);

  useEffect(() => {
    if (!autoScroll || !live || !scrollerRef.current) return;
    scrollerRef.current.scrollTop = scrollerRef.current.scrollHeight;
  }, [autoScroll, live, viewer?.content]);

  const parsedLines = useMemo(() => {
    let currentBucket: LevelBucket | undefined;
    return normalizeLineEndings(viewer?.content || '').split('\n').map(line => {
      const parsed = parseLogLine(line);
      if (parsed.kind === 'structured') {
        currentBucket = parsed.bucket || currentBucket || 'info';
        return parsed;
      }
      if (parsed.kind === 'continuation') {
        return { ...parsed, bucket: currentBucket };
      }
      return parsed;
    });
  }, [viewer?.content]);

  const visibleLines = useMemo(() => {
    if (viewerMode !== 'log') {
      return parsedLines;
    }
    return parsedLines.filter(line => {
      if (!line.bucket) {
        return true;
      }
      return levelVisibility[line.bucket];
    });
  }, [levelVisibility, parsedLines, viewerMode]);

  const renderedLines = useMemo(() => {
    const query = deferredSearchQuery.trim();
    let nextMatchIndex = 0;

    const lines = visibleLines.map((parsed, lineIndex) => {
      const tone = levelTone(parsed.bucket);
      let timestampRanges: MatchRange[] = [];
      let channelRanges: MatchRange[] = [];
      let sourceRanges: MatchRange[] = [];
      let messageRanges: MatchRange[] = [];
      let rawRanges: MatchRange[] = [];

      if (viewerMode === 'crash') {
        const rawMatch = buildMatchRanges(parsed.raw || '', query, nextMatchIndex);
        rawRanges = rawMatch.ranges;
        nextMatchIndex = rawMatch.nextIndex;
      } else if (parsed.kind === 'structured') {
        const timestampMatch = buildMatchRanges(parsed.timestamp || '', query, nextMatchIndex);
        timestampRanges = timestampMatch.ranges;
        nextMatchIndex = timestampMatch.nextIndex;

        const channelMatch = buildMatchRanges(parsed.channel || '', query, nextMatchIndex);
        channelRanges = channelMatch.ranges;
        nextMatchIndex = channelMatch.nextIndex;

        const sourceMatch = buildMatchRanges(parsed.source || '', query, nextMatchIndex);
        sourceRanges = sourceMatch.ranges;
        nextMatchIndex = sourceMatch.nextIndex;

        const messageMatch = buildMatchRanges(parsed.message || '', query, nextMatchIndex);
        messageRanges = messageMatch.ranges;
        nextMatchIndex = messageMatch.nextIndex;
      } else {
        const rawMatch = buildMatchRanges(parsed.raw || '', query, nextMatchIndex);
        rawRanges = rawMatch.ranges;
        nextMatchIndex = rawMatch.nextIndex;
      }

      return {
        lineIndex,
        parsed,
        tone,
        timestampRanges,
        channelRanges,
        sourceRanges,
        messageRanges,
        rawRanges,
      } satisfies RenderedLine;
    });

    return { lines, totalMatches: nextMatchIndex };
  }, [deferredSearchQuery, visibleLines]);

  useEffect(() => {
    setActiveMatchIndex(0);
  }, [deferredSearchQuery, viewer?.relativePath, viewerMode, levelVisibility]);

  useEffect(() => {
    if (!deferredSearchQuery.trim() || renderedLines.totalMatches === 0) {
      return;
    }
    const frame = requestAnimationFrame(() => {
      const scroller = scrollerRef.current;
      const target = matchElementsRef.current.get(activeMatchIndex);
      if (!scroller || !target) {
        return;
      }
      const scrollerRect = scroller.getBoundingClientRect();
      const targetRect = target.getBoundingClientRect();
      const targetTopInside = targetRect.top - scrollerRect.top + scroller.scrollTop;
      const visiblePadding = 88;
      const top = targetTopInside - visiblePadding;
      scroller.scrollTop = Math.max(top, 0);
    });
    return () => cancelAnimationFrame(frame);
  }, [activeMatchIndex, deferredSearchQuery, renderedLines.totalMatches]);

  useEffect(() => {
    if (!levelMenuOpen) {
      return;
    }
    const handlePointerDown = (event: MouseEvent) => {
      if (levelMenuRef.current?.contains(event.target as Node)) {
        return;
      }
      setLevelMenuOpen(false);
    };
    window.addEventListener('mousedown', handlePointerDown);
    return () => window.removeEventListener('mousedown', handlePointerDown);
  }, [levelMenuOpen]);

  const latestCrashLabel = useMemo(() => {
    if (!overview?.latestCrash) return 'No crash reports found';
    return `${overview.latestCrash.name} • ${formatTimestamp(overview.latestCrash.modifiedUnix)}`;
  }, [overview]);

  const jumpMatch = (direction: 1 | -1) => {
    if (renderedLines.totalMatches === 0) {
      return;
    }
    setActiveMatchIndex(current => {
      const next = current + direction;
      if (next < 0) {
        return renderedLines.totalMatches - 1;
      }
      if (next >= renderedLines.totalMatches) {
        return 0;
      }
      return next;
    });
  };

  const toggleLevel = (bucket: LevelBucket) => {
    setLevelVisibility(current => ({
      ...current,
      [bucket]: !current[bucket],
    }));
  };

  const enabledLevelCount = useMemo(
    () => (Object.values(levelVisibility).filter(Boolean).length),
    [levelVisibility],
  );

  if (loading && !overview) {
    return <div className="flex h-full items-center justify-center text-sm text-slate-500">Loading logs...</div>;
  }

  if (!overview?.available) {
    return (
      <div className="flex h-full items-center justify-center p-6">
        <div className="max-w-lg rounded-2xl border border-amber-200/80 bg-white/90 p-6 shadow-[0_20px_60px_-30px_rgba(15,23,42,0.18)]">
          <div className="flex items-start gap-3">
            <div className="rounded-xl bg-amber-50 p-2 text-amber-700 ring-1 ring-amber-200">
              <AlertTriangle width={18} height={18} />
            </div>
            <div>
              <h2 className="text-base font-semibold text-slate-900">No active instance</h2>
              <p className="mt-1 text-sm text-slate-600">
                Pick a Minecraft instance in Settings before opening the logs view. Runtime logs and crash reports are read from the selected pack folder.
              </p>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-full min-h-0 flex-col bg-[radial-gradient(circle_at_top_right,_rgba(20,184,166,0.08),_transparent_26%),linear-gradient(180deg,_rgba(255,255,255,0.92)_0%,_rgba(248,250,252,0.96)_100%)]">
      <div className="border-b border-emerald-100/80 bg-white/80 px-5 py-4 backdrop-blur-sm">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <div className="flex items-center gap-2">
              <span className="inline-flex items-center rounded-full bg-teal-50 px-2 py-1 text-[10px] font-semibold uppercase tracking-[0.12em] text-teal-700 ring-1 ring-teal-200">
                Logs
              </span>
              {instanceName && <span className="text-sm font-medium text-slate-700">{instanceName}</span>}
            </div>
            <h1 className="mt-2 text-xl font-semibold text-slate-900">Runtime logs and crash reports</h1>
          </div>

          <div className="flex items-center gap-2 text-xs text-slate-500">
            <span className="rounded-full bg-white px-3 py-1.5 ring-1 ring-slate-200">{overview.logFiles.length} log file{overview.logFiles.length !== 1 ? 's' : ''}</span>
            <span className="rounded-full bg-white px-3 py-1.5 ring-1 ring-slate-200">{overview.crashReports.length} crash report{overview.crashReports.length !== 1 ? 's' : ''}</span>
            <button
              onClick={() => void refreshOverview()}
              className="inline-flex items-center gap-2 rounded-full bg-slate-900 px-3 py-1.5 font-medium text-white transition-colors hover:bg-slate-800"
            >
              <RefreshCw width={14} height={14} className={refreshing ? 'animate-spin' : ''} /> Refresh
            </button>
          </div>
        </div>
      </div>

      <div className="flex min-h-0 flex-1 flex-col gap-4 p-4 lg:flex-row">
        <div className="flex w-full shrink-0 flex-col gap-4 lg:w-[348px]">
          <section className="rounded-2xl border border-emerald-100/80 bg-white/92 p-4 shadow-[0_18px_40px_-28px_rgba(16,185,129,0.22)]">
            <div className="flex items-start justify-between gap-3">
              <div>
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-400">Latest crash</div>
                <div className="mt-1 text-sm font-medium text-slate-900">{latestCrashLabel}</div>
              </div>
              <div className="rounded-xl bg-rose-50 p-2 text-rose-700 ring-1 ring-rose-200">
                <AlertTriangle width={16} height={16} />
              </div>
            </div>

            <button
              onClick={() => void openLatestCrash()}
              disabled={!overview.latestCrash}
              className="mt-4 inline-flex w-full items-center justify-center gap-2 rounded-xl border border-rose-200 bg-rose-50 px-3 py-2 text-sm font-medium text-rose-700 transition-colors hover:bg-rose-100 disabled:cursor-not-allowed disabled:opacity-50"
            >
              <FileText width={16} height={16} /> Open latest crash report
            </button>

            <div className="mt-4 space-y-2">
              <label className="text-xs font-medium text-slate-500">Crash report history</label>
              <select
                value={selectedCrashPath}
                onChange={event => void openCrash(event.target.value)}
                className="w-full rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 focus:border-slate-400 focus:outline-none focus:ring-1 focus:ring-slate-300"
              >
                {overview.crashReports.length === 0 && <option value="">No crash reports found</option>}
                {overview.crashReports.map(file => (
                  <option key={file.relativePath} value={file.relativePath}>
                    {file.name}
                  </option>
                ))}
              </select>
            </div>
          </section>

          <section className="rounded-2xl border border-emerald-100/80 bg-white/92 p-4 shadow-[0_18px_40px_-28px_rgba(14,165,233,0.20)]">
            <div className="flex items-start justify-between gap-3">
              <div>
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-400">Runtime logs</div>
                <div className="mt-1 text-sm text-slate-600">Selecting a log opens it immediately. Use the mode switch to follow it live or inspect it as a static snapshot.</div>
              </div>
              <div className="rounded-xl bg-teal-50 p-2 text-teal-700 ring-1 ring-teal-200">
                <Activity width={16} height={16} />
              </div>
            </div>

            <div className="mt-4 space-y-2">
              <label className="text-xs font-medium text-slate-500">Runtime log</label>
              <select
                value={selectedLogPath}
                onChange={event => void openLog(event.target.value, live)}
                className="w-full rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 focus:border-slate-400 focus:outline-none focus:ring-1 focus:ring-slate-300"
              >
                {overview.logFiles.length === 0 && <option value={overview.defaultLiveLog}>{overview.defaultLiveLog}</option>}
                {overview.logFiles.map(file => (
                  <option key={file.relativePath} value={file.relativePath}>
                    {file.name}
                  </option>
                ))}
                {!overview.logFiles.some(file => file.relativePath === overview.defaultLiveLog) && (
                  <option value={overview.defaultLiveLog}>{overview.defaultLiveLog}</option>
                )}
              </select>
            </div>

            <div className="mt-4 flex flex-col gap-2 sm:flex-row">
              <button
                onClick={() => void openLog(selectedLogPath || overview.defaultLiveLog, live)}
                className="inline-flex flex-1 items-center justify-center gap-2 rounded-xl bg-slate-900 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-slate-800"
              >
                <FileText width={16} height={16} /> Open selected log
              </button>
              <div className="inline-flex rounded-xl bg-slate-100 p-1 ring-1 ring-slate-200">
                <button
                  onClick={() => void toggleLiveMode(false)}
                  className={`rounded-lg px-3 py-1.5 text-sm font-medium transition-colors ${!live ? 'bg-white text-slate-900 shadow-sm ring-1 ring-slate-200' : 'text-slate-600 hover:text-slate-900'}`}
                >
                  Static
                </button>
                <button
                  onClick={() => void toggleLiveMode(true)}
                  className={`rounded-lg px-3 py-1.5 text-sm font-medium transition-colors ${live ? 'bg-slate-900 text-white shadow-sm' : 'text-slate-600 hover:text-slate-900'}`}
                >
                  Live
                </button>
              </div>
            </div>

            <div className="mt-4 rounded-xl border border-slate-200 bg-slate-50/70 px-3 py-3 text-xs text-slate-600">
              <div className="flex items-center justify-between gap-2">
                <span className="font-medium text-slate-700">Live status</span>
                <span className={`rounded-full px-2 py-0.5 font-semibold ${live ? 'bg-emerald-100 text-emerald-700' : 'bg-slate-200 text-slate-600'}`}>
                  {live ? 'Following selected log' : 'Static snapshot'}
                </span>
              </div>
              <p className="mt-2 leading-5">
                Default live target: <span className="font-medium text-slate-800">{overview.defaultLiveLog}</span>
              </p>
              <label className="mt-3 inline-flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={autoScroll}
                  onChange={event => setAutoScroll(event.target.checked)}
                  className="h-4 w-4 rounded border-slate-300 text-slate-900 focus:ring-slate-400"
                />
                Auto-scroll while new lines arrive
              </label>
            </div>
          </section>
        </div>

        <section className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-2xl border border-emerald-100/80 bg-white/92 shadow-[0_18px_40px_-28px_rgba(15,23,42,0.16)]">
          <div className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-200/80 bg-white/85 px-4 py-3">
            <div>
              <div className="flex items-center gap-2">
                <span className="text-sm font-semibold text-slate-900">{viewer?.name || 'No file selected'}</span>
                <span className={`rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.12em] ring-1 ${viewerMode === 'crash' ? 'bg-rose-100 text-rose-700 ring-rose-200' : 'bg-sky-100 text-sky-700 ring-sky-200'}`}>
                  {viewerMode === 'crash' ? 'Crash' : live ? 'Live log' : 'Static log'}
                </span>
                {viewer?.truncated && (
                  <span className="rounded-full bg-amber-100 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.12em] text-amber-700 ring-1 ring-amber-200">
                    Tail view
                  </span>
                )}
                {viewer?.missing && (
                  <span className="rounded-full bg-rose-100 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.12em] text-rose-700 ring-1 ring-rose-200">
                    Waiting for file
                  </span>
                )}
              </div>
              <div className="mt-1 text-xs text-slate-500">
                {viewer?.relativePath || 'Choose a log or crash report from the left panel.'}
              </div>
            </div>

            <div className="flex flex-wrap items-center gap-2 text-xs text-slate-600">
              {viewer && <span className="rounded-full bg-slate-100 px-3 py-1.5 ring-1 ring-slate-200">{formatSize(viewer.totalSize)}</span>}
              {viewer?.modifiedUnix ? <span className="rounded-full bg-slate-100 px-3 py-1.5 ring-1 ring-slate-200">{formatTimestamp(viewer.modifiedUnix)}</span> : null}
              {viewerMode === 'log' && selectedLogPath === overview?.defaultLiveLog && (
                <button
                  onClick={() => void clearLatestLog()}
                  disabled={clearingLog}
                  className="inline-flex items-center gap-2 rounded-full bg-rose-50 px-3 py-1.5 font-medium text-rose-700 ring-1 ring-rose-200 transition-colors hover:bg-rose-100 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <Eraser width={14} height={14} /> {clearingLog ? 'Clearing...' : 'Clear latest.log'}
                </button>
              )}
              <button
                onClick={() => {
                  if (viewerMode === 'crash' && selectedCrashPath) {
                    void openCrash(selectedCrashPath);
                    return;
                  }
                  if (selectedLogPath) {
                    void openLog(selectedLogPath, live);
                  }
                }}
                className="inline-flex items-center gap-2 rounded-full bg-slate-100 px-3 py-1.5 font-medium text-slate-700 ring-1 ring-slate-200 transition-colors hover:bg-slate-200"
              >
                <RotateCcw width={14} height={14} /> Reload
              </button>
            </div>
          </div>

          <div className="border-b border-slate-200/80 bg-white/80 px-4 py-3">
            <div className="flex min-w-0 flex-1 items-center gap-2">
                {viewerMode === 'log' && (
                  <div ref={levelMenuRef} className="relative shrink-0">
                    <button
                      onClick={() => setLevelMenuOpen(open => !open)}
                      className="inline-flex items-center gap-1.5 rounded-full bg-slate-100 px-2.5 py-1.5 text-xs font-medium text-slate-700 ring-1 ring-slate-200 transition-colors hover:bg-slate-200"
                    >
                      <span>Levels</span>
                      <span className="rounded-full bg-white px-1.5 py-0.5 text-[10px] text-slate-500 ring-1 ring-slate-200">{enabledLevelCount}/4</span>
                      <ChevronDown width={12} height={12} className={`transition-transform ${levelMenuOpen ? 'rotate-180' : ''}`} />
                    </button>

                    {levelMenuOpen && (
                      <div className="absolute left-0 top-[calc(100%+0.45rem)] z-20 min-w-[244px] rounded-2xl border border-slate-200 bg-white p-2 shadow-[0_18px_50px_-28px_rgba(15,23,42,0.28)]">
                        <div className="px-2 pb-2 pt-1 text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-400">Visible levels</div>
                        <div className="grid grid-cols-2 gap-2 px-2 pb-2">
                          {(['debug', 'info', 'warn', 'error'] as LevelBucket[]).map(bucket => {
                            const filterTone = levelFilterTone(bucket);
                            const Icon = levelFilterIcon(bucket);
                            const enabled = levelVisibility[bucket];
                            return (
                              <button
                                key={bucket}
                                onClick={() => toggleLevel(bucket)}
                                className={`inline-flex items-center gap-2 rounded-2xl px-2.5 py-2 text-[11px] font-semibold capitalize ring-1 transition-colors ${enabled ? `${filterTone.chip}` : 'bg-slate-100 text-slate-500 ring-slate-200'}`}
                              >
                                <span className={`inline-flex h-6 w-6 items-center justify-center rounded-full ${enabled ? filterTone.dot : 'bg-slate-300'} text-white`}>
                                  <Icon width={12} height={12} />
                                </span>
                                <span>{bucket}</span>
                              </button>
                            );
                          })}
                        </div>
                      </div>
                    )}
                  </div>
                )}

                <div className="relative min-w-0 flex-1">
                  <Search width={14} height={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
                  <input
                    type="text"
                    value={searchQuery}
                    onChange={event => setSearchQuery(event.target.value)}
                    placeholder="Search inside the current log or crash report..."
                    className="w-full rounded-xl border border-slate-200 bg-white py-2 pl-9 pr-10 text-sm text-slate-900 placeholder:text-slate-400 focus:border-slate-400 focus:outline-none focus:ring-1 focus:ring-slate-300"
                  />
                  {searchQuery && (
                    <button
                      onClick={() => setSearchQuery('')}
                      className="absolute right-2 top-1/2 inline-flex h-6 w-6 -translate-y-1/2 items-center justify-center rounded-full text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-700"
                      aria-label="Clear search"
                    >
                      <X width={14} height={14} />
                    </button>
                  )}
                </div>
              </div>
          </div>

          {error && (
            <div className="border-b border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
              {error}
            </div>
          )}

          <div ref={scrollerRef} className="relative min-h-0 flex-1 overflow-auto px-4 py-4">
            {deferredSearchQuery.trim() && renderedLines.totalMatches > 0 && (
              <div className="pointer-events-none sticky right-0 top-0 z-10 flex justify-end pr-1 pt-1">
                <div className="pointer-events-auto inline-flex items-center gap-2 rounded-full border border-slate-700/80 bg-slate-950/88 px-2 py-1 text-xs text-slate-100 shadow-[0_12px_28px_-16px_rgba(2,6,23,0.9)] backdrop-blur-sm">
                  <span className="rounded-full bg-white/8 px-2 py-0.5 text-[11px] text-slate-200 ring-1 ring-white/10">
                    {activeMatchIndex + 1}/{renderedLines.totalMatches}
                  </span>
                  <button
                    onClick={() => jumpMatch(-1)}
                    disabled={renderedLines.totalMatches === 0}
                    className="inline-flex items-center gap-1 rounded-full bg-white/8 px-2 py-1 font-medium text-slate-100 ring-1 ring-white/10 transition-colors hover:bg-white/14 disabled:cursor-not-allowed disabled:opacity-40"
                  >
                    <ChevronUp width={13} height={13} />
                  </button>
                  <button
                    onClick={() => jumpMatch(1)}
                    disabled={renderedLines.totalMatches === 0}
                    className="inline-flex items-center gap-1 rounded-full bg-white/8 px-2 py-1 font-medium text-slate-100 ring-1 ring-white/10 transition-colors hover:bg-white/14 disabled:cursor-not-allowed disabled:opacity-40"
                  >
                    <ChevronDown width={13} height={13} />
                  </button>
                </div>
              </div>
            )}

            <div className={`rounded-2xl border border-slate-900/60 bg-[linear-gradient(180deg,rgba(2,6,23,0.985)_0%,rgba(15,23,42,0.97)_100%)] p-2.5 font-mono text-[11.5px] leading-[1.16rem] text-slate-50 shadow-inner shadow-black/30 [font-variant-numeric:tabular-nums] ${deferredSearchQuery.trim() && renderedLines.totalMatches > 0 ? 'mt-2' : ''}`}>
              {!viewer?.content ? (
                <div className="rounded-lg px-2 py-1 text-slate-400">No file loaded yet.</div>
              ) : renderedLines.lines.length === 0 ? (
                <div className="rounded-lg px-2 py-1 text-slate-400">No lines match the current level filter.</div>
              ) : renderedLines.lines.map(({ lineIndex, parsed, tone, timestampRanges, channelRanges, sourceRanges, messageRanges, rawRanges }) => {
                if (viewerMode === 'crash') {
                  const crashKind = classifyCrashLine(parsed.raw);
                  if (crashKind === 'blank') {
                    return <div key={lineIndex} className="h-2" />;
                  }

                  const crashClassName = crashKind === 'heading'
                    ? 'mb-px rounded-md bg-amber-300/10 px-2 py-[3px] font-semibold text-amber-100'
                    : crashKind === 'section'
                      ? 'mb-px rounded-md bg-sky-300/8 px-2 py-[3px] font-semibold text-sky-100'
                      : crashKind === 'cause'
                        ? 'mb-px rounded-md bg-rose-300/10 px-2 py-[3px] text-rose-100'
                        : crashKind === 'trace'
                          ? 'mb-px border-l border-white/10 pl-4 pr-2 py-[2px] text-slate-300'
                          : 'mb-px rounded-md px-2 py-[3px] text-slate-100';

                  return (
                    <div key={lineIndex} className={`${crashClassName} whitespace-pre-wrap break-words`}>
                      {highlightFragments(parsed.raw, rawRanges, activeMatchIndex, registerMatchElement)}
                    </div>
                  );
                }

                if (parsed.kind === 'structured') {
                  return (
                    <div
                      key={lineIndex}
                      className={`mb-px grid grid-cols-[5.4rem_auto_minmax(0,1fr)] items-start gap-x-1.5 rounded-md px-2 py-[3px] ${tone.row}`}
                    >
                      <span className="pt-px text-slate-400">
                        {highlightFragments(parsed.timestamp || '', timestampRanges, activeMatchIndex, registerMatchElement)}
                      </span>
                      <div className="flex min-w-0 flex-wrap items-center gap-1.5 pt-px">
                        <span className={`rounded-full px-1.5 py-[1px] text-[9px] font-semibold uppercase tracking-[0.12em] ${tone.badge}`}>
                          {parsed.level || 'LOG'}
                        </span>
                        {parsed.channel && (
                          <span className="text-[10px] text-slate-400">
                            {highlightFragments(parsed.channel, channelRanges, activeMatchIndex, registerMatchElement)}
                          </span>
                        )}
                        {parsed.source && (
                          <span className="rounded-full bg-white/6 px-1.5 py-[1px] text-[9px] text-slate-300 ring-1 ring-white/10">
                            {highlightFragments(parsed.source, sourceRanges, activeMatchIndex, registerMatchElement)}
                          </span>
                        )}
                      </div>
                      <span className={`min-w-0 whitespace-pre-wrap break-words ${tone.text}`}>
                        {highlightFragments(parsed.message, messageRanges, activeMatchIndex, registerMatchElement)}
                      </span>
                    </div>
                  );
                }

                if (parsed.kind === 'comment') {
                  return (
                    <div key={lineIndex} className="mb-px rounded-md bg-sky-300/8 px-2 py-[3px] whitespace-pre-wrap break-words text-sky-100/90">
                      {highlightFragments(parsed.raw, rawRanges, activeMatchIndex, registerMatchElement)}
                    </div>
                  );
                }

                if (parsed.kind === 'continuation') {
                  return (
                    <div key={lineIndex} className="mb-px rounded-md bg-white/5 px-2 py-[3px] whitespace-pre-wrap break-words text-slate-200">
                      {highlightFragments(parsed.raw, rawRanges, activeMatchIndex, registerMatchElement)}
                    </div>
                  );
                }

                return (
                  <div key={lineIndex} className="mb-px rounded-md px-2 py-[3px] whitespace-pre-wrap break-words text-slate-100">
                    {highlightFragments(parsed.raw, rawRanges, activeMatchIndex, registerMatchElement)}
                  </div>
                );
              })}
            </div>
          </div>
        </section>
      </div>
    </div>
  );
}