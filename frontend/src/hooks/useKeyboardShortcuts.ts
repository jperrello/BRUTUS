import { useEffect, useCallback, useState } from 'react';

export interface ShortcutConfig {
  key: string;
  modifiers?: ('ctrl' | 'alt' | 'shift' | 'meta')[];
  action: () => void;
  description: string;
  category: string;
  when?: () => boolean;
}

interface ShortcutMatch {
  key: string;
  ctrl: boolean;
  alt: boolean;
  shift: boolean;
  meta: boolean;
}

function normalizeKey(key: string): string {
  const keyMap: Record<string, string> = {
    'escape': 'Escape',
    'esc': 'Escape',
    'enter': 'Enter',
    'return': 'Enter',
    'space': ' ',
    'arrowup': 'ArrowUp',
    'arrowdown': 'ArrowDown',
    'arrowleft': 'ArrowLeft',
    'arrowright': 'ArrowRight',
    'backspace': 'Backspace',
    'delete': 'Delete',
    'tab': 'Tab',
  };
  const lower = key.toLowerCase();
  return keyMap[lower] || key;
}

function parseShortcut(config: ShortcutConfig): ShortcutMatch {
  const modifiers = config.modifiers || [];
  return {
    key: normalizeKey(config.key),
    ctrl: modifiers.includes('ctrl'),
    alt: modifiers.includes('alt'),
    shift: modifiers.includes('shift'),
    meta: modifiers.includes('meta'),
  };
}

function matchesShortcut(e: KeyboardEvent, match: ShortcutMatch): boolean {
  const keyMatches = e.key.toLowerCase() === match.key.toLowerCase();
  const ctrlMatches = e.ctrlKey === match.ctrl;
  const altMatches = e.altKey === match.alt;
  const shiftMatches = e.shiftKey === match.shift;
  const metaMatches = e.metaKey === match.meta;

  return keyMatches && ctrlMatches && altMatches && shiftMatches && metaMatches;
}

function formatShortcut(config: ShortcutConfig): string {
  const parts: string[] = [];
  const mods = config.modifiers || [];

  if (mods.includes('ctrl')) parts.push('Ctrl');
  if (mods.includes('alt')) parts.push('Alt');
  if (mods.includes('shift')) parts.push('Shift');
  if (mods.includes('meta')) parts.push(navigator.platform.includes('Mac') ? '⌘' : 'Win');

  let keyDisplay = config.key;
  const displayMap: Record<string, string> = {
    'Enter': '↵',
    'Escape': 'Esc',
    'ArrowUp': '↑',
    'ArrowDown': '↓',
    'ArrowLeft': '←',
    'ArrowRight': '→',
    ' ': 'Space',
  };
  if (displayMap[keyDisplay]) {
    keyDisplay = displayMap[keyDisplay];
  }
  parts.push(keyDisplay.toUpperCase());

  return parts.join('+');
}

export interface UseKeyboardShortcutsResult {
  registerShortcut: (config: ShortcutConfig) => () => void;
  unregisterShortcut: (key: string, modifiers?: string[]) => void;
  getShortcuts: () => ShortcutConfig[];
  formatShortcut: (config: ShortcutConfig) => string;
}

export function useKeyboardShortcuts(initialShortcuts?: ShortcutConfig[]): UseKeyboardShortcutsResult {
  const [shortcuts, setShortcuts] = useState<ShortcutConfig[]>(initialShortcuts || []);

  const registerShortcut = useCallback((config: ShortcutConfig) => {
    setShortcuts(prev => {
      const existing = prev.findIndex(s =>
        s.key === config.key &&
        JSON.stringify(s.modifiers) === JSON.stringify(config.modifiers)
      );
      if (existing >= 0) {
        const updated = [...prev];
        updated[existing] = config;
        return updated;
      }
      return [...prev, config];
    });

    return () => {
      setShortcuts(prev => prev.filter(s =>
        !(s.key === config.key && JSON.stringify(s.modifiers) === JSON.stringify(config.modifiers))
      ));
    };
  }, []);

  const unregisterShortcut = useCallback((key: string, modifiers?: string[]) => {
    setShortcuts(prev => prev.filter(s =>
      !(s.key === key && JSON.stringify(s.modifiers) === JSON.stringify(modifiers))
    ));
  }, []);

  const getShortcuts = useCallback(() => shortcuts, [shortcuts]);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement;
      const isInput = target.tagName === 'INPUT' ||
                      target.tagName === 'TEXTAREA' ||
                      target.isContentEditable;

      for (const config of shortcuts) {
        const match = parseShortcut(config);

        if (matchesShortcut(e, match)) {
          if (config.when && !config.when()) {
            continue;
          }

          const hasModifiers = match.ctrl || match.alt || match.meta;
          if (isInput && !hasModifiers) {
            continue;
          }

          e.preventDefault();
          e.stopPropagation();
          config.action();
          return;
        }
      }
    };

    window.addEventListener('keydown', handleKeyDown, true);
    return () => window.removeEventListener('keydown', handleKeyDown, true);
  }, [shortcuts]);

  return {
    registerShortcut,
    unregisterShortcut,
    getShortcuts,
    formatShortcut,
  };
}

export const DEFAULT_SHORTCUTS: Omit<ShortcutConfig, 'action'>[] = [
  { key: 'k', modifiers: ['ctrl'], description: 'Open command palette', category: 'General' },
  { key: 'Enter', modifiers: ['ctrl'], description: 'Send message', category: 'Chat' },
  { key: '.', modifiers: ['ctrl'], description: 'Stop generation', category: 'Chat' },
  { key: 'n', modifiers: ['ctrl'], description: 'New agent', category: 'Agents' },
  { key: ',', modifiers: ['ctrl'], description: 'Open settings', category: 'General' },
  { key: '/', modifiers: ['ctrl', 'shift'], description: 'Show shortcuts help', category: 'Help' },
  { key: 'Escape', description: 'Close modal/panel', category: 'Navigation' },
  { key: '1', modifiers: ['ctrl'], description: 'Focus first agent', category: 'Navigation' },
  { key: '2', modifiers: ['ctrl'], description: 'Focus second agent', category: 'Navigation' },
  { key: '3', modifiers: ['ctrl'], description: 'Focus third agent', category: 'Navigation' },
];
