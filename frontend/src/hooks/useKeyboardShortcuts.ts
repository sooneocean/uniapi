import { useEffect } from 'react';

type Shortcut = {
  key: string;
  ctrl?: boolean;
  meta?: boolean; // Cmd on Mac
  shift?: boolean;
  action: () => void;
  description: string;
};

export const shortcuts: Omit<Shortcut, 'action'>[] = [
  { key: 'k', ctrl: true, description: 'Search conversations' },
  { key: 'k', meta: true, description: 'Search conversations' },
  { key: 'n', ctrl: true, description: 'New conversation' },
  { key: 'n', meta: true, description: 'New conversation' },
  { key: '/', ctrl: true, description: 'Focus message input' },
  { key: ',', ctrl: true, description: 'Open settings' },
  { key: ',', meta: true, description: 'Open settings' },
  { key: '?', ctrl: true, shift: true, description: 'Show keyboard shortcuts' },
];

export function useKeyboardShortcuts(shortcutList: Shortcut[]) {
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      for (const s of shortcutList) {
        const ctrlMatch = s.ctrl ? (e.ctrlKey || e.metaKey) : true;
        const metaMatch = s.meta ? e.metaKey : true;
        const shiftMatch = s.shift ? e.shiftKey : !e.shiftKey;

        if (e.key.toLowerCase() === s.key.toLowerCase() && ctrlMatch && metaMatch && shiftMatch) {
          e.preventDefault();
          s.action();
          return;
        }
      }
    };

    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [shortcutList]);
}
