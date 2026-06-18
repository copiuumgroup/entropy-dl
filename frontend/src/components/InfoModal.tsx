import { useState, useEffect, useRef, useCallback } from 'react';
import { createPortal } from 'react-dom';
import { M3ViewTransition } from './m3';

// ═══════════════════════════════════════════════════════════════════════
//  InfoModal — About / How-it-works dialog
//  M3 Dialog with tabbed sections explaining the app.
// ═══════════════════════════════════════════════════════════════════════

interface InfoModalProps {
  open: boolean;
  onClose: () => void;
}

const SECTIONS = [
  { id: 'overview', icon: 'info', label: 'Overview' },
  { id: 'how', icon: 'build', label: 'How it works' },
  { id: 'stack', icon: 'code', label: 'Tech stack' },
  { id: 'platforms', icon: 'devices', label: 'Platforms' },
  { id: 'tools', icon: 'handyman', label: 'Tools' },
  { id: 'build', icon: 'deployed_code', label: 'Building' },
  { id: 'settings', icon: 'tune', label: 'Config' },
  { id: 'privacy', icon: 'lock', label: 'Privacy' },
] as const;

type SectionId = (typeof SECTIONS)[number]['id'];

export default function InfoModal({ open, onClose }: InfoModalProps) {
  const [activeSection, setActiveSection] = useState<SectionId>('overview');
  const dialogRef = useRef<HTMLDivElement>(null);
  const bodyRef = useRef<HTMLDivElement>(null);

  // Toggle scroll shadow on the body
  useEffect(() => {
    const el = bodyRef.current;
    if (!el) return;
    const onScroll = () => el.classList.toggle('scrolled', el.scrollTop > 0);
    el.addEventListener('scroll', onScroll, { passive: true });
    onScroll();
    return () => el.removeEventListener('scroll', onScroll);
  }, [open]);

  // Close on Escape
  const handleKeyDown = useCallback((e: KeyboardEvent) => {
    if (e.key === 'Escape') onClose();
  }, [onClose]);

  // Focus trap
  useEffect(() => {
    if (!open) return;
    document.addEventListener('keydown', handleKeyDown);
    // Move focus into dialog
    dialogRef.current?.focus();

    const previousFocus = document.activeElement as HTMLElement;

    const trap = (e: FocusEvent) => {
      if (!dialogRef.current?.contains(e.target as Node)) {
        const first = dialogRef.current?.querySelector<HTMLElement>('button, [tabindex]');
        first?.focus();
      }
    };
    document.addEventListener('focusin', trap);

    return () => {
      document.removeEventListener('keydown', handleKeyDown);
      document.removeEventListener('focusin', trap);
      previousFocus?.focus();
    };
  }, [open, handleKeyDown]);

  if (!open) return null;

  const content = (
    <div className="m3-dialog-portal">
      {/* Scrim */}
      <div className="m3-dialog-scrim" onClick={onClose} />

      {/* Dialog */}
      <div className="m3-dialog" role="dialog" aria-modal="true" aria-label="About Entropy" ref={dialogRef} tabIndex={-1}>
        {/* Header — close button pinned top-right so it's always visible */}
        <div className="m3-dialog-header">
          <div className="m3-dialog-header-text">
            <h2 className="m3-dialog-title">Entropy DL</h2>
            <span className="m3-dialog-subtitle">About &amp; how-to</span>
          </div>
          <button
            className="btn icon m3-dialog-close-btn"
            onClick={onClose}
            aria-label="Close dialog"
            title="Close"
          >
            <span className="search-icon" aria-hidden="true">close</span>
          </button>
        </div>

        {/* Section tabs (scrollable horizontally) */}
        <div className="info-tabs-wrapper">
          <div className="info-tabs">
            {SECTIONS.map((s) => (
              <button
                key={s.id}
                className={`info-tab${activeSection === s.id ? ' active' : ''}`}
                onClick={() => setActiveSection(s.id)}
              >
                <span className="info-tab-icon" aria-hidden="true">{s.icon}</span>
                <span>{s.label}</span>
              </button>
            ))}
          </div>
        </div>

        {/* Section content — scrollable, fills remaining height */}
        <div className="m3-dialog-body info-body" ref={bodyRef}>
          <M3ViewTransition keyProp={activeSection}>
            {activeSection === 'overview' && <SectionOverview />}
            {activeSection === 'how' && <SectionHow />}
            {activeSection === 'stack' && <SectionStack />}
            {activeSection === 'platforms' && <SectionPlatforms />}
            {activeSection === 'tools' && <SectionTools />}
            {activeSection === 'build' && <SectionBuild />}
            {activeSection === 'settings' && <SectionSettings />}
            {activeSection === 'privacy' && <SectionPrivacy />}
          </M3ViewTransition>
        </div>
      </div>
    </div>
  );

  return createPortal(content, document.body);
}

// ─── Section: Overview ──────────────────────────────────────────────

function SectionOverview() {
  return (
    <div className="info-section">
      <h3>What is Entropy?</h3>
      <p>
        Entropy is a self-contained desktop media downloader with a Material You 3
        interface. Search, queue, and download from YouTube, YouTube Music,
        SoundCloud, and direct URLs. The entire app is a single binary with an
        embedded web frontend — no installer, no runtime dependencies beyond the
        external tools. The UI runs at <code>127.0.0.1:8001</code> with real-time
        progress via Server-Sent Events.
      </p>
      <h3>Key features</h3>
      <ul>
        <li>Search YouTube, YT Music, and SoundCloud — or all three with "Everything"</li>
        <li>Download audio (MP3, M4A, FLAC, Opus, WAV) and video (MP4, MKV, WebM)</li>
        <li>Paste direct links, or bulk-import <code>.txt</code>/<code>.csv</code>/<code>.m3u</code> files</li>
        <li>Separate audio/video folders with per-job format, bitrate &amp; resolution</li>
        <li>Bulk queue with configurable concurrent downloads</li>
        <li>Automatic metadata embedding (tags, thumbnails) via yt-dlp</li>
        <li>Persists queue and settings across restarts (BoltDB)</li>
        <li>Material You 3 dynamic theming — matches your system accent color</li>
        <li>Bandwidth limiting, cookies-from-browser, speed control</li>
        <li>Opens in a chromeless browser window (native app feel)</li>
      </ul>
    </div>
  );
}

// ─── Section: How it works ──────────────────────────────────────────

function SectionHow() {
  return (
    <div className="info-section">
      <h3>Startup sequence</h3>
      <ol>
        <li>
          <strong>Load config</strong> — Reads <code>.env</code> beside the binary
          for overrides (port, output dir, workers). Everything has sensible defaults.
        </li>
        <li>
          <strong>Locate tools</strong> — Checks <code>$PATH</code> for
          yt-dlp, ffmpeg, and aria2c. Missing tools are reported in the UI but
          don't prevent startup.
        </li>
        <li>
          <strong>Open database</strong> — Creates or opens a BoltDB file
          (<code>entropy.db</code>) to store settings, job history, and onboarding
          state. Survives app restarts.
        </li>
        <li>
          <strong>Restore state</strong> — Loads saved settings (output dir,
          bandwidth limit, worker count) and any unfinished jobs from the
          previous session.
        </li>
        <li>
          <strong>Start HTTP server</strong> — Binds to <code>127.0.0.1:8001</code>
          (loopback only, not reachable from the network). Serves the embedded
          React SPA and 20 JSON API endpoints.
        </li>
        <li>
          <strong>Open browser</strong> — Launches a Chromium-based browser in
          app mode (no address bar, no tabs) pointing at the local server.
          Falls back to xdg-open or default browser on Linux, or system shell
          on Windows.
        </li>
      </ol>
      <h3>Download flow</h3>
      <ol>
        <li>You search or paste URLs and add items to the queue.</li>
        <li>The backend spawns yt-dlp (or aria2c for accelerated downloads).</li>
        <li>Progress is streamed to the frontend via Server-Sent Events in real time.</li>
        <li>Completed files land in your output directory with metadata embedded.</li>
      </ol>
      <h3>Graceful shutdown</h3>
      <p>
        Ctrl+C or the Quit button triggers a graceful shutdown: active downloads
        are allowed 5 seconds to finish, the database is flushed and closed,
        then the process exits. No orphan processes or corrupted state.
      </p>
    </div>
  );
}

// ─── Section: Tech stack ────────────────────────────────────────────

function SectionStack() {
  return (
    <div className="info-section">
      <h3>Backend (Go)</h3>
      <ul>
        <li><strong>Language:</strong> Go 1.23+ — compiled to a static binary</li>
        <li><strong>HTTP:</strong> Net/http from the standard library (no frameworks)</li>
        <li><strong>Database:</strong> BoltDB — embedded key-value store, zero config</li>
        <li><strong>Real-time:</strong> Server-Sent Events (SSE) for live job updates</li>
        <li><strong>Frontend embed:</strong> Go <code>embed.FS</code> — React SPA baked into the binary</li>
        <li><strong>Security:</strong> CSRF tokens, rate limiting (30 req/s, burst 60), 1MB body cap, loopback-only binding, strict CORS</li>
        <li><strong>Platform:</strong> Windows, Linux, macOS — single codebase, conditional compilation for OS-specific features</li>
      </ul>
      <h3>Frontend (React + TypeScript)</h3>
      <ul>
        <li><strong>UI framework:</strong> React 18 with TypeScript — zero UI component libraries</li>
        <li><strong>Animation:</strong> Framer Motion — spring physics, Android 16+ M3 motion patterns</li>
        <li><strong>Design system:</strong> Hand-built Material You 3 with HCT color algorithm, dynamic token generation</li>
        <li><strong>Theming:</strong> Reads system accent color (Windows DWM / macOS / Linux D-Bus) and generates full M3 tonal palettes at runtime</li>
        <li><strong>Build:</strong> Vite 8 with <code>@vitejs/plugin-react</code> for fast HMR and optimized production builds</li>
        <li><strong>Size:</strong> A single bundled JS + CSS file, embedded directly in the Go binary — no runtime downloads</li>
      </ul>
      <h3>External tools</h3>
      <ul>
        <li><strong>yt-dlp:</strong> Media extraction, metadata, format conversion</li>
        <li><strong>ffmpeg:</strong> Audio/video encoding, thumbnail embedding, format transcoding</li>
        <li><strong>aria2c:</strong> Multi-connection accelerated downloads (optional)</li>
      </ul>
    </div>
  );
}

// ─── Section: Platforms ─────────────────────────────────────────────

function SectionPlatforms() {
  return (
    <div className="info-section">
      <h3>Portable across distros</h3>
      <p>
        The Go binary is fully static — no shared libraries, no glibc pinning,
        no runtime dependencies. It runs identically on any Linux distribution
        (Fedora, Ubuntu, Arch, NixOS, Alpine, etc.) and macOS. Separate builds
        are produced for x86_64 and ARM64; grab the one matching your CPU.
      </p>
      <h3>Supported operating systems</h3>
      <ul>
        <li>
          <strong>Linux</strong> — Any distro, x86_64 or ARM64. Only requires
          yt-dlp, ffmpeg, and aria2c on <code>$PATH</code>.
        </li>
        <li>
          <strong>Windows</strong> — Windows 10/11 (x64). Launches in Edge
          or Chrome app mode. Tools must be on <code>%PATH%</code>.
        </li>
        <li>
          <strong>macOS</strong> — Intel and Apple Silicon. Reads system accent
          color from Appearance settings for M3 theming.
        </li>
      </ul>
      <h3>Network</h3>
      <p>
        The server binds exclusively to <code>127.0.0.1</code> (loopback).
        Not accessible from your LAN or the internet. No external ports are opened.
      </p>
    </div>
  );
}

// ─── Section: External tools ────────────────────────────────────────

function SectionTools() {
  return (
    <div className="info-section">
      <h3>Required</h3>
      <ul>
        <li>
          <strong>yt-dlp</strong> — Core download engine. Handles URL resolution,
          format selection, metadata extraction, and download coordination.
        </li>
        <li>
          <strong>ffmpeg</strong> — Audio extraction, format conversion (MP3,
          FLAC, etc.), and thumbnail embedding.
        </li>
      </ul>
      <h3>Optional</h3>
      <ul>
        <li>
          <strong>aria2c</strong> — Multi-connection download accelerator. Speeds
          up downloads by splitting files into segments; without it, yt-dlp's
          built-in downloader is used.
        </li>
      </ul>
      <h3>Install commands</h3>
      <div className="info-code-block">
        <code>sudo dnf install yt-dlp ffmpeg aria2    # Fedora / RHEL</code>
        <code>sudo apt install yt-dlp ffmpeg aria2    # Ubuntu / Debian</code>
        <code>sudo pacman -S yt-dlp ffmpeg aria2      # Arch Linux</code>
        <code>brew install yt-dlp ffmpeg aria2         # macOS (Homebrew)</code>
      </div>
      <p>
        The app checks for these on startup and shows their status in the top bar.
        Missing required tools are shown as warnings — downloads will fail until
        they are installed.
      </p>
    </div>
  );
}

// ─── Section: Building ──────────────────────────────────────────────

function SectionBuild() {
  return (
    <div className="info-section">
      <h3>Quick start</h3>
      <div className="info-code-block">
        <code>./build.sh            # Linux / macOS</code>
        <code>build.bat             # Windows</code>
      </div>
      <p>
        The build script installs frontend dependencies, compiles the React SPA
        with Vite, copies the output into the Go embed directory, and compiles
        the single binary with the frontend baked in. The output lands in
        <code> releases/entropy</code> — just run it.
      </p>
      <h3>Build outputs</h3>
      <ul>
        <li>
          <code>releases/entropy</code> (or <code>entropy.exe</code>) —
          the standalone binary, produced by both scripts.
        </li>
        <li>
          <code>{'releases/entropy-dev-{platform}'}</code> —
          archive directory. <strong>Linux/macOS only.</strong>
        </li>
        <li>
          <code>{'releases/entropy-dev-{platform}.sha256'}</code> —
          SHA-256 checksum. <strong>Linux/macOS only.</strong>
        </li>
      </ul>
      <h3>Requirements</h3>
      <ul>
        <li>Go 1.23+</li>
        <li>Node.js 20+ and npm</li>
      </ul>
      <h3>Other commands</h3>
      <div className="info-code-block">
        <code>./build.sh dev         # Frontend dev server with HMR</code>
        <code>./build.sh clean      # Wipe all build artifacts</code>
      </div>
      <p>
        On Windows, <code>build.bat</code> only supports <code>release</code>{' '}
        and <code>clean</code> — for dev mode, run{' '}
        <code>cd frontend &amp;&amp; npm run dev</code> manually.
      </p>
    </div>
  );
}

// ─── Section: Configuration ─────────────────────────────────────────

function SectionSettings() {
  return (
    <div className="info-section">
      <h3>Environment variables</h3>
      <p>
        Create a <code>.env</code> file beside the binary to override defaults.
        Values changed in the Settings panel take precedence and are persisted
        to the database — so the UI is the source of truth after first run.
      </p>
      <ul>
        <li>
          <code>PORT</code> — HTTP port (default: 8001)
        </li>
        <li>
          <code>DOWNLOAD_DIR</code> — Base directory used to derive the
          audio/video output folders before they are customized
          (defaults: <code>~/Music/Entropy</code> for audio,
          <code>~/Videos/Entropy</code> for video)
        </li>
        <li>
          <code>MAX_WORKERS</code> — Max concurrent downloads (default: 2)
        </li>
        <li>
          <code>YTDLP_BIN</code> — Path to yt-dlp binary (default: yt-dlp)
        </li>
        <li>
          <code>FFMPEG_BIN</code> — Path to ffmpeg binary (default: ffmpeg)
        </li>
        <li>
          <code>ARIA2C_BIN</code> — Path to aria2c binary (default: aria2c)
        </li>
        <li>
          <code>STATE_FILE</code> — Path to the BoltDB database. Falls back
          through: the given path → beside the binary (<code>entropy.db</code>)
          → <code>~/.config/entropy-gui/entropy.db</code> → current directory
        </li>
        <li>
          <code>ENTROPY_NO_LAUNCH</code> — Set to 1 to skip auto-opening
          the browser on startup
        </li>
        <li>
          <code>USE_HTTPS</code> — Set to 1 for HTTPS (requires certificate
          setup, currently aspirational)
        </li>
      </ul>
      <h3>Settings you can change in the UI</h3>
      <p>
        These live in the Settings panel and persist across restarts:
      </p>
      <ul>
        <li><strong>Output folders</strong> — separate paths for audio and video</li>
        <li><strong>Concurrent workers</strong> — how many jobs run at once (1–32)</li>
        <li><strong>Bandwidth limit</strong> — global rate cap for yt-dlp (e.g. <code>5M</code>, <code>0</code> = unlimited)</li>
        <li><strong>Cookies</strong> — pull session cookies from a browser (Chrome, Edge, Firefox, …) for age-gated or members content</li>
        <li><strong>Format, bitrate &amp; resolution</strong> — per-job defaults for audio and video</li>
        <li><strong>Metadata &amp; thumbnails</strong> — embed tags and cover art</li>
      </ul>
      <h3>Database</h3>
      <p>
        Settings and job history are stored in a BoltDB file. This is a
        single-file embedded database — no separate server process. You can
        safely delete <code>entropy.db</code> to reset all settings and job
        history to defaults.
      </p>
    </div>
  );
}

// ─── Section: Privacy ───────────────────────────────────────────────

function SectionPrivacy() {
  return (
    <div className="info-section">
      <h3>No telemetry</h3>
      <p>
        Entropy does not collect, transmit, or report any data. No analytics,
        no tracking, no error reporting, no phone-home. The only network activity
        is your explicit downloads and the local HTTP server.
      </p>
      <h3>Local only</h3>
      <p>
        The HTTP server binds exclusively to <code>127.0.0.1</code> — not
        reachable from your LAN or the internet. CORS is restricted to the
        localhost origin, so other websites can't talk to the API. The shutdown
        endpoint requires an HttpOnly SameSite=Lax cookie for CSRF protection.
      </p>
      <h3>No accounts</h3>
      <p>
        Single-user local tool. No accounts, no cloud sync, no authentication.
        Session cookies are only used for CSRF protection.
      </p>
      <h3>No file scanning</h3>
      <p>
        The app accesses only its own output directory and database. It does not
        monitor your activity, install background services, or modify system
        configuration.
      </p>
    </div>
  );
}