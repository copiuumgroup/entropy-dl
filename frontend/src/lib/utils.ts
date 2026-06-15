import type { SSEEvent } from '../types';

// SSE connection factory with auto-reconnect and exponential backoff.
// Returns a cleanup function.
export function connectSSE(callback: (event: SSEEvent) => void): () => void {
  const baseUrl = import.meta.env.VITE_BACKEND_URL || '';
  const url = `${baseUrl}/api/jobs/stream`;

  let es: EventSource | null = null;
  let retryDelay = 1000; // Start at 1s
  const maxDelay = 30000;  // Cap at 30s
  let timeoutId: ReturnType<typeof setTimeout> | null = null;
  let disposed = false;

  const handler = (event: MessageEvent): void => {
    try {
      callback(JSON.parse(event.data) as SSEEvent);
    } catch {
      // ignore parse errors
    }
  };

  const connect = (): void => {
    if (disposed) return;

    es = new EventSource(url);

    (['snapshot', 'job', 'log'] as const).forEach((type) => {
      es!.addEventListener(type, handler as EventListener);
    });

    es.onopen = () => {
      // Connection restored — reset backoff
      retryDelay = 1000;
    };

    es.onerror = () => {
      es?.close();
      es = null;

      if (disposed) return;

      // Exponential backoff with jitter
      const jitter = Math.random() * 500;
      const delay = retryDelay + jitter;

      timeoutId = setTimeout(() => {
        retryDelay = Math.min(retryDelay * 2, maxDelay);
        connect();
      }, delay);
    };
  };

  connect();

  return () => {
    disposed = true;
    if (timeoutId !== null) clearTimeout(timeoutId);
    es?.close();
  };
}

// Format duration in seconds to MM:SS or H:MM:SS
export function formatDuration(seconds: number | null | undefined): string {
  if (seconds == null || isNaN(seconds)) return '--:--';
  const s = Math.floor(seconds % 60);
  const m = Math.floor((seconds / 60) % 60);
  const h = Math.floor(seconds / 3600);
  const pad = (n: number): string => String(n).padStart(2, '0');
  return h > 0 ? `${h}:${pad(m)}:${pad(s)}` : `${m}:${pad(s)}`;
}

// Status color map — references vibrant semantic status CSS properties
export const STATUS_COLORS: Record<string, string> = {
  queued: 'var(--md-outline)',
  searching: 'var(--md-status-downloading)',
  downloading: 'var(--md-status-downloading)',
  processing: 'var(--md-status-processing)',
  done: 'var(--md-status-done)',
  failed: 'var(--md-status-failed)',
  canceled: 'var(--md-status-failed)',
};

// Status label map
export const STATUS_LABELS: Record<string, string> = {
  queued: 'Queued',
  searching: 'Searching',
  downloading: 'Downloading',
  processing: 'Post-processing',
  done: 'Complete',
  failed: 'Failed',
  canceled: 'Canceled',
};