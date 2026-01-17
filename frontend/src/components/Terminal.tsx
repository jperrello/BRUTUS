import { useEffect, useRef, useState } from 'react';
import { Terminal as XTerm } from 'xterm';
import { FitAddon } from 'xterm-addon-fit';
import { WebLinksAddon } from 'xterm-addon-web-links';
import 'xterm/css/xterm.css';

declare global {
  interface Window {
    go: {
      main: {
        App: {
          PTYSpawn: (shell: string) => Promise<string>;
          PTYWrite: (id: string, data: string) => Promise<void>;
          PTYKill: (id: string) => Promise<void>;
          PTYList: () => Promise<string[]>;
        };
      };
    };
    runtime: {
      EventsOn: (eventName: string, callback: (...args: any[]) => void) => () => void;
      EventsOff: (eventName: string) => void;
    };
  }
}

interface TerminalProps {
  onReady?: (id: string) => void;
  className?: string;
}

export function Terminal({ onReady, className }: TerminalProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<XTerm | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!containerRef.current) return;

    const term = new XTerm({
      cursorBlink: true,
      fontSize: 14,
      fontFamily: 'Consolas, "Courier New", monospace',
      theme: {
        background: '#1a1a1a',
        foreground: '#e0e0e0',
        cursor: '#ff9800',
        cursorAccent: '#1a1a1a',
        selectionBackground: 'rgba(255, 152, 0, 0.3)',
        black: '#1a1a1a',
        red: '#ff5252',
        green: '#69f0ae',
        yellow: '#ffd740',
        blue: '#448aff',
        magenta: '#e040fb',
        cyan: '#18ffff',
        white: '#e0e0e0',
        brightBlack: '#616161',
        brightRed: '#ff8a80',
        brightGreen: '#b9f6ca',
        brightYellow: '#ffe57f',
        brightBlue: '#82b1ff',
        brightMagenta: '#ea80fc',
        brightCyan: '#84ffff',
        brightWhite: '#ffffff',
      },
    });

    const fitAddon = new FitAddon();
    const webLinksAddon = new WebLinksAddon();

    term.loadAddon(fitAddon);
    term.loadAddon(webLinksAddon);

    term.open(containerRef.current);
    fitAddon.fit();

    termRef.current = term;
    fitAddonRef.current = fitAddon;

    const resizeObserver = new ResizeObserver(() => {
      if (fitAddonRef.current) {
        fitAddonRef.current.fit();
      }
    });
    resizeObserver.observe(containerRef.current);

    window.go.main.App.PTYSpawn('')
      .then((id) => {
        setSessionId(id);
        if (onReady) {
          onReady(id);
        }

        term.onData((data) => {
          window.go.main.App.PTYWrite(id, data).catch(console.error);
        });
      })
      .catch((err) => {
        setError(`Failed to spawn terminal: ${err}`);
        term.writeln(`\x1b[31mFailed to spawn terminal: ${err}\x1b[0m`);
      });

    return () => {
      resizeObserver.disconnect();
      term.dispose();
    };
  }, [onReady]);

  useEffect(() => {
    if (!sessionId || !termRef.current) return;

    const unsubscribeData = window.runtime.EventsOn('pty:data', (event: { id: string; data: string }) => {
      if (event.id === sessionId && termRef.current) {
        termRef.current.write(event.data);
      }
    });

    const unsubscribeExit = window.runtime.EventsOn('pty:exit', (event: { id: string; exitCode: number }) => {
      if (event.id === sessionId && termRef.current) {
        termRef.current.writeln(`\r\n\x1b[33mProcess exited with code ${event.exitCode}\x1b[0m`);
      }
    });

    return () => {
      unsubscribeData();
      unsubscribeExit();
      if (sessionId) {
        window.go.main.App.PTYKill(sessionId).catch(console.error);
      }
    };
  }, [sessionId]);

  return (
    <div
      ref={containerRef}
      className={className}
      style={{
        width: '100%',
        height: '100%',
        minHeight: '200px',
        backgroundColor: '#1a1a1a',
      }}
    />
  );
}

export default Terminal;
