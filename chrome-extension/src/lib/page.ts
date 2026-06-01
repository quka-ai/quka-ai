import { truncateText } from './utils';
import type { PageSnapshot } from './types';

function readPageInTab(): PageSnapshot {
  const clone = document.body?.cloneNode(true) as HTMLElement | null;

  if (clone) {
    clone.querySelectorAll('script, style, noscript, svg, canvas, iframe, nav, footer, aside').forEach(node => node.remove());
  }

  const metaDescription = document.querySelector('meta[name="description"]')?.getAttribute('content') || '';
  const canonical = document.querySelector('link[rel="canonical"]')?.getAttribute('href') || window.location.href;
  const selection = window.getSelection()?.toString() || '';
  const text = clone?.innerText || document.body?.innerText || '';

  return {
    title: document.title || '',
    url: canonical,
    description: metaDescription,
    selection: truncateText(selection.trim(), 12000),
    text: truncateText(text.replace(/\n{3,}/g, '\n\n').trim(), 24000),
    lang: document.documentElement.lang || navigator.language || ''
  };
}

export async function readCurrentPage(): Promise<PageSnapshot> {
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });

  if (!tab?.id) {
    throw new Error('No active tab found.');
  }

  try {
    const [frame] = await chrome.scripting.executeScript({
      target: { tabId: tab.id },
      func: readPageInTab
    });

    if (frame?.result) {
      return {
        ...frame.result,
        title: frame.result.title || tab.title || '',
        url: frame.result.url || tab.url || ''
      };
    }
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    if (!tab.url?.startsWith('http')) {
      throw new Error('This page cannot be read by a Chrome extension.');
    }
    throw new Error(message);
  }

  return {
    title: tab.title || '',
    url: tab.url || '',
    description: '',
    selection: '',
    text: '',
    lang: ''
  };
}
