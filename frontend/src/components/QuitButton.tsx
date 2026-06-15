import { useState } from 'react';
import type { QuitButtonProps } from '../types';

export default function QuitButton(_props: QuitButtonProps) {
  const [confirming, setConfirming] = useState(false);

  const handleShutdown = async (): Promise<void> => {
    try {
      const baseUrl = import.meta.env.VITE_BACKEND_URL || '';
      await fetch(`${baseUrl}/api/shutdown`, { method: 'POST' });
    } catch {
      // ignore
    }
    setTimeout(() => {
      document.body.innerHTML = `
        <div style="background:var(--md-background);color:var(--md-on-surface);font-family:var(--font-mono);display:grid;place-items:center;height:100vh;text-align:center">
          <div>
            <div style="font-size:var(--md-sys-typescale-size-2xl);letter-spacing:0.35em;margin-bottom:var(--sp-4)">SHUTDOWN COMPLETE</div>
            <div style="font-size:var(--text-xs);color:var(--md-outline);letter-spacing:0.2em">ENTROPY // MEDIA LIFT</div>
          </div>
        </div>`;
      window.close();
    }, 600);
  };

  if (confirming) {
    return (
      <span className="quit-confirm" data-testid="quit-confirm">
        Sure?
        <button className="btn ghost danger" onClick={handleShutdown} data-testid="quit-confirm-yes">
          Yes
        </button>
        <button className="btn ghost" onClick={() => setConfirming(false)} data-testid="quit-confirm-no">
          No
        </button>
      </span>
    );
  }

  return (
    <button
      className="btn ghost quit-btn"
      onClick={() => setConfirming(true)}
      data-testid="quit-btn"
      title="Shut down the backend and close the app"
      aria-label="Quit application"
    >
      <span className="search-icon">power_settings_new</span>
    </button>
  );
}
