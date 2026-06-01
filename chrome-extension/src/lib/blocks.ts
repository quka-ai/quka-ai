import type { PartialBlock } from '@blocknote/core';

import type { PageSnapshot } from './types';
import { truncateText } from './utils';

export function emptyBlocks(): PartialBlock[] {
  return [{ type: 'paragraph', content: '' }];
}

export function summaryToBlocks(snapshot: PageSnapshot, summary: string): PartialBlock[] {
  return [
    { type: 'heading', props: { level: 2 }, content: snapshot.title || 'Quka page summary' },
    { type: 'paragraph', content: `来源: ${snapshot.url}` },
    { type: 'paragraph', content: summary.trim() }
  ];
}

export function pageToBlocks(snapshot: PageSnapshot, mode: 'selection' | 'page'): PartialBlock[] {
  const content = mode === 'selection' ? snapshot.selection : snapshot.text;

  return [
    { type: 'heading', props: { level: 2 }, content: snapshot.title || 'Untitled page' },
    { type: 'paragraph', content: `来源: ${snapshot.url}` },
    ...(snapshot.description ? [{ type: 'paragraph', content: snapshot.description } as PartialBlock] : []),
    { type: 'paragraph', content: truncateText(content || '', 12000) }
  ];
}

export function blocksText(blocks: PartialBlock[]): string {
  return blocks
    .map(block => blockText(block))
    .join('\n')
    .trim();
}

function blockText(block: PartialBlock): string {
  const content = block.content;
  const ownText =
    typeof content === 'string'
      ? content
      : Array.isArray(content)
        ? content
            .map(item => {
              if (typeof item === 'string') {
                return item;
              }
              if (item && typeof item === 'object' && 'text' in item) {
                return String(item.text || '');
              }
              return '';
            })
            .join('')
        : '';
  const childText = Array.isArray(block.children) ? block.children.map(child => blockText(child)).join('\n') : '';

  return [ownText, childText].filter(Boolean).join('\n');
}
