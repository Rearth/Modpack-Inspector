import { useState, useEffect, useCallback } from 'react';
import { GetMods, SearchMods, GetUnusedLibraries } from '../../wailsjs/go/main/App';
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime';
import type { Mod, SearchResult } from '../lib/types';

export function useMods() {
  const [mods, setMods] = useState<Mod[]>([]);
  const [loading, setLoading] = useState(true);
  const [scanStatus, setScanStatus] = useState('');
  const [scanProgress, setScanProgress] = useState(0);
  const [unusedLibraries, setUnusedLibraries] = useState<string[]>([]);

  const refresh = useCallback(async () => {
    try {
      const data = await GetMods();
      setMods(data || []);
      const unused = await GetUnusedLibraries();
      setUnusedLibraries(unused || []);
    } catch (e) {
      console.error('Failed to load mods:', e);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();

    EventsOn('mods:updated', () => refresh());
    EventsOn('scan:progress', (msg: string) => {
      setScanStatus(msg);
      setScanProgress(prev => {
        const next = parseScanProgress(msg);
        if (msg.startsWith('Scanning mods')) {
          return next;
        }
        return Math.max(prev, next);
      });
    });
    EventsOn('scan:complete', () => {
      setScanStatus('');
      setScanProgress(0);
      refresh();
    });

    return () => {
      EventsOff('mods:updated');
      EventsOff('scan:progress');
      EventsOff('scan:complete');
    };
  }, [refresh]);

  return { mods, loading, scanStatus, scanProgress, unusedLibraries, refresh };
}

function parseScanProgress(msg: string): number {
  if (!msg) return 0;
  if (msg.startsWith('Scanning mods folder')) return 2;

  // "Scanning mod 15/247..."
  const scanMatch = msg.match(/Scanning mod\s+(\d+)\/(\d+)/i);
  if (scanMatch) {
    const current = Number(scanMatch[1]);
    const total = Number(scanMatch[2]);
    if (Number.isFinite(current) && Number.isFinite(total) && total > 0) {
      return Math.max(2, Math.min(35, 2 + Math.round((current / total) * 33)));
    }
    return 15;
  }

  if (msg.startsWith('Fetching mod info')) return 36;

  if (msg.startsWith('Matching config files')) {
    const match = msg.match(/Matching config files\s+(\d+)\/(\d+)/i);
    if (!match) return 94;
    const current = Number(match[1]);
    const total = Number(match[2]);
    if (!Number.isFinite(current) || !Number.isFinite(total) || total <= 0) return 94;
    return Math.max(94, Math.min(99, 94 + Math.round((current / total) * 5)));
  }

  // "Indexing mod 15/247: ModName"
  const indexMatch = msg.match(/Indexing mod\s+(\d+)\/(\d+)/i);
  if (indexMatch) {
    const current = Number(indexMatch[1]);
    const total = Number(indexMatch[2]);
    if (Number.isFinite(current) && Number.isFinite(total) && total > 0) {
      return Math.max(76, Math.min(92, 76 + Math.round((current / total) * 16)));
    }
    return 84;
  }

  // "Enriching mod 15/247: ModName"
  const enrichMatch = msg.match(/Enriching mod\s+(\d+)\/(\d+)/i);
  if (enrichMatch) {
    const current = Number(enrichMatch[1]);
    const total = Number(enrichMatch[2]);
    if (Number.isFinite(current) && Number.isFinite(total) && total > 0) {
      return Math.max(37, Math.min(75, 37 + Math.round((current / total) * 38)));
    }
    return 50;
  }

  return 36;
}

export function useSearch() {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResult[]>([]);
  const [searching, setSearching] = useState(false);

  useEffect(() => {
    if (!query.trim()) {
      setResults([]);
      return;
    }

    const timer = setTimeout(async () => {
      setSearching(true);
      try {
        const data = await SearchMods(query);
        setResults(data || []);
      } catch (e) {
        console.error('Search failed:', e);
      } finally {
        setSearching(false);
      }
    }, 200);

    return () => clearTimeout(timer);
  }, [query]);

  return { query, setQuery, results, searching };
}
