import { useState, useEffect, useRef } from 'react';
import { fetchEnv, updateTools } from '../lib/api';
import { M3ScaleIn } from './m3';
import { CircularProgress } from './CircularProgress';
import type { ToolsStatusProps, ToolInfo } from '../types';

const TOOL_DISPLAY: Record<string, string> = { yt_dlp: 'yt-dlp', aria2c: 'aria2c', ffmpeg: 'ffmpeg' };
const DOWNLOAD_URLS: Record<string, string> = {
  yt_dlp: 'https://github.com/yt-dlp/yt-dlp/releases',
  aria2c: 'https://github.com/aria2/aria2/releases',
  ffmpeg: 'https://www.gyan.dev/ffmpeg/builds/',
};
const TOOLS = ['yt_dlp', 'aria2c', 'ffmpeg'] as const;

type ToolsData = {
  platform: string;
  distro: string;
  pkg_mgrs: string[];
} & Record<string, ToolInfo | string | string[]>;

export default function ToolsStatus(_props: ToolsStatusProps) {
  const [tools, setTools] = useState<ToolsData | null>(null);
  const [showPop, setShowPop] = useState(false);
  const [updating, setUpdating] = useState(false);
  const popRef = useRef<HTMLDivElement>(null);

  const loadTools = async (): Promise<void> => {
    try {
      const data = await fetchEnv();
      setTools({ ...data.tools, platform: data.platform, distro: data.distro, pkg_mgrs: data.pkg_mgrs } as unknown as ToolsData);
    } catch {
      // ignore
    }
  };

  // Close popover on Escape and trap focus inside
  const handleKeyDown = (e: React.KeyboardEvent): void => {
    if (e.key === 'Escape' && showPop) {
      e.stopPropagation();
      setShowPop(false);
      return;
    }
    // Trap Tab within the popover
    if (e.key === 'Tab' && popRef.current && showPop) {
      const focusable = popRef.current.querySelectorAll<HTMLElement>('button, a, [tabindex]:not([tabindex="-1"])');
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    }
  };

  useEffect(() => {
    loadTools();
    const interval = setInterval(loadTools, 30000);
    return () => clearInterval(interval);
  }, []);

  const handleUpdateYtdlp = async (): Promise<void> => {
    setUpdating(true);
    try {
      await updateTools();
      await loadTools();
    } catch {
      // ignore
    }
    setUpdating(false);
  };

  if (!tools) return null;

  const allFound = TOOLS.every((t) => (tools[t] as ToolInfo)?.found);
  const missingCount = TOOLS.filter((t) => !(tools[t] as ToolInfo)?.found).length;
  const isWindows = tools.platform === 'windows';
  const isDarwin = tools.platform === 'darwin';

  return (
    <div className="tools-status" data-testid="tools-status">
      <button
        className={`tools-pill${!allFound ? ' warn' : ''}`}
        onClick={() => setShowPop(!showPop)}
        aria-label={allFound ? 'Tools available' : `${missingCount} tools missing`}
        aria-expanded={showPop}
        data-testid="tools-pill"
      >
        <span className="tools-dot" />
        {!allFound ? `${missingCount} missing` : 'Tools ready'}
      </button>

      {/* Animated popover */}
      <M3ScaleIn show={showPop} origin="top right" className="tools-pop-anchor">
        <div
          className="tools-pop"
          data-testid="tools-popover"
          role="dialog"
          aria-label="System tools details"
          ref={popRef}
          onKeyDown={handleKeyDown}
        >
          <div className="tools-pop-head">
            <span>System tools</span>
            <button className="btn ghost" onClick={loadTools} data-testid="tools-refresh">
              Refresh
            </button>
          </div>

          {TOOLS.map((toolKey) => {
            const t = (tools[toolKey] as ToolInfo) || {};
            const found = t.found;
            return (
              <div key={toolKey} className="tools-row">
                <div className="tools-row-1">
                  <span className={`tools-state ${found ? 'ok' : 'warn'}`}>
                    {found ? 'OK' : 'Miss'}
                  </span>
                  <span className="tools-name">{TOOL_DISPLAY[toolKey]}</span>
                  {found && t.version && <span className="tools-ver">{t.version}</span>}
                  {toolKey === 'yt_dlp' && found && (
                    <button className="btn ghost" onClick={handleUpdateYtdlp} disabled={updating}>
                      {updating ? <CircularProgress size={16} thickness={2} /> : 'Update'}
                    </button>
                  )}
                </div>
                <div className="tools-row-2">{t.path || ''}</div>
                {!found && (
                  <a
                    className="btn ghost"
                    href={DOWNLOAD_URLS[toolKey]}
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    Download ↗
                  </a>
                )}
              </div>
            );
          })}

          {isWindows && (
            <div className="tools-os-notice windows">
              <div className="tools-os-notice-title">⚠ Windows setup guide</div>
              <div className="tools-os-notice-body">
                <div className="tools-guide-section">
                  <div className="tools-guide-label">yt-dlp (portable)</div>
                  <div className="tools-guide-cmd">
                    <code>winget install yt-dlp.yt-dlp</code>
                    <span className="cmd-comment"># or download .exe from GitHub releases</span>
                  </div>
                </div>
                <div className="tools-guide-section">
                  <div className="tools-guide-label">FFmpeg</div>
                  <div className="tools-guide-cmd">
                    <code>winget install Gyan.FFmpeg</code>
                    <span className="cmd-comment"># or from gyan.dev/ffmpeg/builds/</span>
                  </div>
                </div>
                <div className="tools-guide-section">
                  <div className="tools-guide-label">aria2c</div>
                  <div className="tools-guide-cmd">
                    <code>winget install aria2.aria2</code>
                  </div>
                </div>
                <div className="tools-guide-tip">
                  Place .exe files in ./tools/ beside entropy-gui.exe for portable installs
                </div>
              </div>
            </div>
          )}

          {!isWindows && (
            <div className="tools-os-notice linux">
              <div className="tools-os-notice-title">
                {isDarwin ? 'macOS' : (tools.distro as string)?.toUpperCase() || 'Linux'} · Keep tools updated
              </div>
              <div className="tools-os-notice-body">
                {isDarwin ? (
                  <p>
                    <code>brew upgrade yt-dlp ffmpeg aria2</code>
                  </p>
                ) : (tools.pkg_mgrs as string[])?.includes('pacman') ? (
                  <p>
                    <code>sudo pacman -Syu yt-dlp ffmpeg aria2</code>
                  </p>
                ) : (
                  <p>
                    <code>sudo apt upgrade yt-dlp ffmpeg aria2</code>
                    {' '}or{' '}
                    <code>pipx upgrade yt-dlp</code>
                  </p>
                )}
              </div>
            </div>
          )}

          <div className="tools-pop-foot">
            {isWindows
              ? 'Place .exe files in ./tools/ beside entropy-gui.exe'
              : 'Tools are resolved from PATH · set YTDLP_BIN / FFMPEG_BIN / ARIA2C_BIN to override'}
          </div>
        </div>
      </M3ScaleIn>
    </div>
  );
}