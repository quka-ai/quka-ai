import type { ExtensionSettings } from './types';

const STORAGE_KEY = 'quka-extension-settings';

export const defaultSettings: ExtensionSettings = {
  host: 'http://localhost:33033/api/v1',
  authType: 'access',
  token: '',
  selectedSpaceId: '',
  selectedResourceId: 'knowledge'
};

export async function loadSettings(): Promise<ExtensionSettings> {
  const value = await chrome.storage.local.get(STORAGE_KEY);
  return {
    ...defaultSettings,
    ...(value[STORAGE_KEY] || {})
  };
}

export async function saveSettings(settings: ExtensionSettings) {
  await chrome.storage.local.set({ [STORAGE_KEY]: settings });
}

export async function clearSettings() {
  await chrome.storage.local.remove(STORAGE_KEY);
}
