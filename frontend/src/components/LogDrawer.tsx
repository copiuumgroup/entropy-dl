import { useState, useEffect, useRef } from 'react';
import { M3Expand, M3FadeIn } from './m3';
import type { LogDrawerProps } from '../types';

function isError(line: string): boolean {
  return line.toLowerCase().includes('error') || line.startsWith('ERROR');
}

function formatTime(ts: string): string {
  if (!ts) return '--:--:--';
  try {
    const d = new Date(ts);
    const pad = (n: number): string => String(n).padStart(2, '0');
    return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
  } catch {
    return '--:--:--';
  }
}

export default function LogDrawer({ logs }: LogDrawerProps) {
  const [open, setOpen] = useState(true);
  const logRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (open && logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight;
    }
  }, [open, logs]);

  return (
    <aside className="log-drawer" data-testid="log-drawer" aria-label="Activity log">
      <div className="log-head" onClick={() => setOpen(!open)} role="button" aria-expanded={open} tabIndex={0}>
        <span>Activity log · {logs.length} lines</span>
        <span data-testid="log-toggle">
          {open ? <span className="search-icon">expand_less</span> : <span className="search-icon">expand_more</span>}
        </span>
      </div>
      <M3Expand expanded={open}>
        <div className="log-body" ref={logRef} role="log" aria-live="polite" aria-label="Log messages">
          {logs.length === 0 ? (
            <M3FadeIn>
              <div className="log-line">
                <span className="ts">--:--:--</span> Waiting for activity…
              </div>
            </M3FadeIn>
          ) : (
            logs.map((entry, idx) => (
              <div key={idx} className={`log-line${isError(entry.line) ? ' err' : ''}`}>
                <span className="ts">{formatTime(entry.time)}</span>{' '}
                <span className="jid">[{(entry.job_id || '').slice(0, 6)}]</span>{' '}
                {entry.line}
              </div>
            ))
          )}
        </div>
      </M3Expand>
    </aside>
  );
}