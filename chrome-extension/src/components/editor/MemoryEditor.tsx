import type { PartialBlock } from '@blocknote/core';
import { zh } from '@blocknote/core/locales';
import '@blocknote/core/fonts/inter.css';
import { BlockNoteView } from '@blocknote/mantine';
import '@blocknote/mantine/style.css';
import { useCreateBlockNote } from '@blocknote/react';
import { useEffect, useMemo, useRef } from 'react';

import { emptyBlocks } from '@/lib/blocks';

interface MemoryEditorProps {
  value: PartialBlock[];
  onChange: (value: PartialBlock[]) => void;
}

export function MemoryEditor({ value, onChange }: MemoryEditorProps) {
  const isReplacingRef = useRef(false);
  const lastEmittedRef = useRef('');
  const initialContent = useMemo(() => (value.length > 0 ? value : emptyBlocks()), []);
  const editor = useCreateBlockNote({
    dictionary: zh,
    initialContent,
    placeholders: {
      default: '输入新的记忆'
    }
  });

  useEffect(() => {
    const serialized = JSON.stringify(value);

    if (serialized === lastEmittedRef.current) {
      return;
    }

    isReplacingRef.current = true;
    editor.replaceBlocks(editor.document, value.length > 0 ? value : emptyBlocks());
    window.setTimeout(() => {
      isReplacingRef.current = false;
    });
  }, [editor, value]);

  return (
    <div className="memory-editor rounded-md border border-input bg-background">
      <BlockNoteView
        editor={editor}
        editable
        theme="light"
        sideMenu={false}
        onChange={currentEditor => {
          if (isReplacingRef.current) {
            return;
          }

          const nextValue = currentEditor.document as PartialBlock[];
          lastEmittedRef.current = JSON.stringify(nextValue);
          onChange(nextValue);
        }}
      />
    </div>
  );
}
