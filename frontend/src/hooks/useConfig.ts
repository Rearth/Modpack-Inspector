import { useState, useCallback } from 'react';
import { GetAbsoluteConfigPath, GetConfigsForMod, ReadConfigFile, SaveConfigFile, SetConfigOverride, RemoveConfigOverride } from '../../wailsjs/go/main/App';
import type { ConfigMapping } from '../lib/types';

export function useConfig() {
  const [configs, setConfigs] = useState<ConfigMapping[]>([]);
  const [content, setContent] = useState('');
  const [activePath, setActivePath] = useState('');
  const [activeFullPath, setActiveFullPath] = useState('');
  const [loading, setLoading] = useState(false);
  const [dirty, setDirty] = useState(false);

  const loadConfigsForMod = useCallback(async (modID: string) => {
    try {
      const data = await GetConfigsForMod(modID);
      setConfigs(data || []);
    } catch (e) {
      console.error('Failed to load configs:', e);
      setConfigs([]);
    }
  }, []);

  const openConfig = useCallback(async (configPath: string) => {
    if (!configPath) {
      setActivePath('');
      setActiveFullPath('');
      setContent('');
      setDirty(false);
      return;
    }
    setLoading(true);
    try {
      const [data, fullPath] = await Promise.all([
        ReadConfigFile(configPath),
        GetAbsoluteConfigPath(configPath),
      ]);
      setContent(data);
      setActivePath(configPath);
      setActiveFullPath(fullPath);
      setDirty(false);
    } catch (e) {
      setContent('');
      setActivePath(configPath);
      setActiveFullPath('');
      setDirty(false);
    } finally {
      setLoading(false);
    }
  }, []);

  const saveConfig = useCallback(async () => {
    if (!activePath) return;
    try {
      await SaveConfigFile(activePath, content);
      setDirty(false);
    } catch (e) {
      console.error('Failed to save config:', e);
    }
  }, [activePath, content]);

  const setOverride = useCallback(async (modID: string, configPath: string) => {
    await SetConfigOverride(modID, configPath);
  }, []);

  const removeOverride = useCallback(async (modID: string, configPath: string) => {
    await RemoveConfigOverride(modID, configPath);
  }, []);

  return {
    configs, content, setContent, activePath, activeFullPath, loading, dirty, setDirty,
    loadConfigsForMod, openConfig, saveConfig, setOverride, removeOverride
  };
}
