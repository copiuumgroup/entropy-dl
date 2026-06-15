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
  { id: 'tools', icon: 'handyman', label: 'External tools' },
  { id: 'build', icon: 'deployed_code', label: 'Building' },
  { id: 'settings', icon: 'tune', label: 'Configuration' },
  { id: 'privacy', icon: 'lock', label: 'Privacy' },
] as const;

type SectionId = (typeof SECTIONS)[number]['id'];

export default function InfoModal({ open, onClose }: InfoModalProps) {
  const [activeSection, setActiveSection] = useState<SectionId>('overview');
  const dialogRef = useRef<HTMLDivElement>(null);

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
            <h2 className="m3-dialog-title">Entropy // Media Lift</h2>
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

        {/* Section content — scrollable, fills remaining height */}
        <div className="m3-dialog-body info-body">
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
        Entropy is a self-contained desktop media downloader. It provides a clean
        Material You 3 interface for searching, queuing, and downloading media
        from YouTube, YouTube Music, SoundCloud, and direct URLs. The entire
        application is a single binary with an embedded web frontend — no
        installer, no runtime dependencies beyond the external download tools.
      </p>
      <p>
        The UI runs in your browser at <code>http://127.0.0.1:8001</code> and
        communicates with the Go backend over a local HTTP API. Server-Sent
        Events (SSE) provide real-time download progress updates without
        polling.
      </p>
      <h3>Key features</h3>
      <ul>
        <li>Search YouTube, YT Music, and SoundCloud directly</li>
        <li>Download audio (MP3, FLAC, OGG, M4A) and video (MP4, MKV, WEBM)</li>
        <li>Bulk queue with configurable concurrent downloads</li>
        <li>Automatic metadata embedding (tags, thumbnails) via yt-dlp</li>
        <li>Persists queue and settings across restarts (BoltDB)</li>
        <li>Material You 3 dynamic theming — matches your system colors</li>
        <li>Bandwidth limiting and download speed control</li>
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
        <li><strong>Language:</strong> Go 1.22+ — compiled to a static binary</li>
        <li><strong>HTTP:</strong> Net/http from the standard library (no frameworks)</li>
        <li><strong>Database:</strong> BoltDB — embedded key-value store, zero config</li>
        <li><strong>Real-time:</strong> Server-Sent Events (SSE) for live job updates</li>
        <li><strong>Frontend embed:</strong> Go 1.16+ <code>embed.FS</code> — React SPA baked into the binary</li>
        <li><strong>Security:</strong> CSRF tokens, rate limiting (30 req/s), 1MB body cap, loopback-only binding</li>
        <li><strong>Platform:</strong> Windows, Linux, macOS — single codebase, conditional compilation for OS-specific features</li>
      </ul>
      <h3>Frontend (React + TypeScript)</h3>
      <ul>
        <li><strong>UI framework:</strong> React 18 with TypeScript — zero UI component libraries</li>
        <li><strong>Animation:</strong> Framer Motion — spring physics, Android 16+ M3 motion patterns</li>
        <li><strong>Design system:</strong> Hand-built Material You 3 with HCT color algorithm, dynamic token generation</li>
        <li><strong>Theming:</strong> Reads system accent color (Windows DWM / macOS / Linux D-Bus) and generates full M3 tonal palettes at runtime</li>
        <li><strong>Build:</strong> Vite 5 with SWC for fast HMR and optimized production builds</li>
        <li><strong>Size:</strong> ~100KB gzipped JS + ~15KB CSS — embedded directly in the Go binary</li>
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
      <h3>Distro-agnostic &amp; portable</h3>
      <p>
        The Go binary is fully static — no shared libraries, no glibc version
        pinning, no runtime dependencies. It runs identically on any Linux
        distribution: Fedora, Ubuntu, Arch, Debian, NixOS, Alpine, Gentoo,
        openSUSE, or any other. The same binary works on x86_64 and ARM64
        (aarch64) without recompilation.
      </p>
      <p>
        There is nothing distro-specific in the application. It does not
        install system packages, modify system configuration, or depend on
        distribution-specific paths. It reads and writes only to its own
        directory (database, output files) and respects standard XDG
        conventions for configuration.
      </p>
      <h3>Supported operating systems</h3>
      <ul>
        <li>
          <strong>Linux</strong> — Any distribution with a kernel that supports
          the target architecture. Tested on Fedora, Ubuntu, Arch Linux, and
          their derivatives. Only requires the external tools (yt-dlp, ffmpeg,
          aria2c) to be on <code>$PATH</code>.
        </li>
        <li>
          <strong>Windows</strong> — Windows 10/11 (x64). Launches in
          Edge or Chrome app mode. External tools must be on <code>%PATH%</code>.
        </li>
        <li>
          <strong>macOS</strong> — Intel and Apple Silicon. Reads system accent
          color from Appearance settings for M3 theming.
        </li>
      </ul>
      <h3>Network</h3>
      <p>
        The server binds exclusively to <code>127.0.0.1</code> (loopback).
        It is not accessible from other devices on your LAN or from the
        internet. No ports are opened externally. No incoming connections
        are accepted.
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
          <strong>yt-dlp</strong> — The core download engine. Handles URL
          resolution, format selection, metadata extraction, and download
          coordination. Install via your package manager or from
          <code>github.com/yt-dlp/yt-dlp</code>.
        </li>
        <li>
          <strong>ffmpeg</strong> — Required by yt-dlp for audio extraction,
          format conversion (MP3, FLAC, etc.), and thumbnail embedding into
          audio files.
        </li>
      </ul>
      <h3>Optional</h3>
      <ul>
        <li>
          <strong>aria2c</strong> — Multi-connection download accelerator. When
          available, Entropy uses aria2c for faster downloads by splitting
          files into multiple segments. Without it, yt-dlp's built-in
          downloader is used instead.
        </li>
      </ul>
      <h3>Install commands</h3>
      <div className="info-code-block">
        <p>Fedora / RHEL:</p>
        <code>sudo dnf install yt-dlp ffmpeg aria2</code>
        <p>Ubuntu / Debian:</p>
        <code>sudo apt install yt-dlp ffmpeg aria2</code>
        <p>Arch Linux:</p>
        <code>sudo pacman -S yt-dlp ffmpeg aria2</code>
        <p>macOS (Homebrew):</p>
        <code>brew install yt-dlp ffmpeg aria2</code>
      </div>
      <p>
        The app checks for these tools on startup and reports their status
        in the top bar. Missing tools are shown as warnings — the app still
        starts, but downloads will fail until the required tools are installed.
      </p>
    </div>
  );
}

// ─── Section: Building ──────────────────────────────────────────────

function SectionBuild() {
  return (
    <div className="info-section">
      <h3>Build from source</h3>
      <p>
        The build is fully automated by a single script. It installs frontend
        dependencies, compiles the React SPA with Vite, copies the output into
        the Go embed directory, and compiles the Go binary with the frontend
        baked in.
      </p>
      <div className="info-code-block">
        <p>Full production build:</p>
        <code>./build.sh</code>
        <p>The binary lands in <code>releases/entropy</code> — run it directly:</p>
        <code>./releases/entropy</code>
      </div>
      <h3>Build outputs</h3>
      <ul>
        <li>
          <code>releases/entropy</code> — The standalone binary (Linux/macOS).
          <code>entropy.exe</code> on Windows.
        </li>
        <li>
          <code>{'releases/entropy-v{version}-{platform}.tar.gz'}</code> — Versioned
          archive with the binary and build metadata.
        </li>
        <li>
          <code>{'releases/entropy-v{version}-{platform}.tar.gz.sha256'}</code> —
          SHA-256 checksum for verification.
        </li>
      </ul>
      <h3>Build requirements</h3>
      <ul>
        <li>Go 1.22+</li>
        <li>Node.js 18+ and npm</li>
        <li>That's it — no other build-time dependencies</li>
      </ul>
      <h3>Dev mode</h3>
      <div className="info-code-block">
        <p>Frontend only (hot reload, backend separate):</p>
        <code>./build.sh dev</code>
        <p>Clean all build artifacts:</p>
        <code>./build.sh clean</code>
      </div>
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
        All settings can also be changed from the Settings panel in the UI,
        which persists them to the database.
      </p>
      <ul>
        <li>
          <code>PORT</code> — HTTP port (default: 8001)
        </li>
        <li>
          <code>DOWNLOAD_DIR</code> — Output directory for downloads
          (default: ~/Downloads/Entropy)
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
          <code>STATE_FILE</code> — Path to BoltDB database
          (default: entropy.db beside binary)
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
        Entropy does not collect, transmit, or report any data. There is no
        analytics, no tracking, no error reporting service, and no phone-home
        mechanism of any kind. The application has no network activity beyond
        your explicit downloads and the local HTTP server.
      </p>
      <h3>Local only</h3>
      <p>
        The HTTP server binds to <code>127.0.0.1</code> exclusively. It is not
        reachable from other devices on your local network or from the internet.
        No incoming connections are accepted from external addresses.
      </p>
      <h3>No accounts</h3>
      <p>
        There are no user accounts, no cloud sync, no authentication. The app
        is a single-user local tool. Session cookies are HttpOnly and
        SameSite=Lax, used only for CSRF protection on the shutdown endpoint.
      </p>
      <h3>No file scanning</h3>
      <p>
        The application does not scan your file system, monitor your activity,
        or access files outside of its output directory and database. It does
        not install background services, launch daemons, or modify system
        configuration.
      </p>
    </div>
  );
}