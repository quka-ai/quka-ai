import type { ReactNode } from 'react';

import { cn } from '@/lib/utils';

interface TabsProps<T extends string> {
  value: T;
  tabs: Array<{ value: T; label: string; icon?: ReactNode }>;
  onValueChange: (value: T) => void;
}

export function Tabs<T extends string>({ value, tabs, onValueChange }: TabsProps<T>) {
  return (
    <div className="grid gap-1 rounded-lg bg-muted p-1" style={{ gridTemplateColumns: `repeat(${tabs.length}, minmax(0, 1fr))` }}>
      {tabs.map(tab => (
        <button
          key={tab.value}
          type="button"
          className={cn(
            'inline-flex h-8 items-center justify-center gap-1.5 rounded-md px-2 text-sm font-medium transition-colors',
            value === tab.value ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
          )}
          onClick={() => onValueChange(tab.value)}
        >
          {tab.icon}
          {tab.label}
        </button>
      ))}
    </div>
  );
}
