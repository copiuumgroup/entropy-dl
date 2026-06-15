import { useState } from 'react';
import { M3Expand, M3ChipAnimated, M3SwitchAnimated, M3FadeIn } from './m3';
import type { SettingsPanelProps, Format, Resolution, Engine, CookiesBrowser } from '../types';

const FORMATS: Format[] = ['mp3', 'm4a', 'flac', 'opus', 'wav', 'mp4', 'webm', 'mkv', 'best'];
const RESOLUTIONS: { value: Resolution; dims: string }[] = [
  { value: 'BEST', dims: '' },
  { value: '4K', dims: '3840×2160' },
  { value: '1440p', dims: '2560×1440' },
  { value: '1080p', dims: '1920×1080' },
  { value: '720p', dims: '1280×720' },
  { value: '480p', dims: '854×480' },
];
const BITRATES = ['192', '320'];
const CONCURRENCIES = ['1', '2', '3', '5', '10'];
const ENGINES: { value: Engine; label: string }[] = [
  { value: 'ytdlp', label: 'yt-dlp' },
  { value: 'aria2c', label: 'aria2c' },
];
const COOKIES: { value: CookiesBrowser; label: string }[] = [
  { value: 'none', label: 'Off' },
  { value: 'chrome', label: 'Chrome' },
  { value: 'edge', label: 'Edge' },
  { value: 'firefox', label: 'Firefox' },
  { value: 'brave', label: 'Brave' },
  { value: 'chromium', label: 'Chromium' },
  { value: 'opera', label: 'Opera' },
  { value: 'vivaldi', label: 'Vivaldi' },
  { value: 'safari', label: 'Safari' },
];

const AUDIO_FORMATS = new Set<Format>(['mp3', 'm4a', 'flac', 'opus', 'wav', 'aac', 'vorbis']);

export default function SettingsPanel({ options, setOptions, themePref, setThemePref }: SettingsPanelProps) {
  const [expanded, setExpanded] = useState(true);

  const isAudio = AUDIO_FORMATS.has(options.format);

  const update = <K extends keyof typeof options>(key: K, value: typeof options[K]): void => {
    setOptions((prev) => ({ ...prev, [key]: value }));
  };

  const toggleBool = (key: 'embed_meta' | 'embed_thumb' | 'scrape_delay'): void => {
    setOptions((prev) => ({ ...prev, [key]: !prev[key] }));
  };

  return (
    <section className="settings" data-testid="settings-panel">
      <div
        className="settings-header"
        onClick={() => setExpanded(!expanded)}
        role="button"
        aria-expanded={expanded}
        tabIndex={0}
        onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); setExpanded(!expanded); } }}
      >
        <span>Settings</span>
        <span className="search-icon">{expanded ? 'expand_less' : 'expand_more'}</span>
      </div>

      <M3Expand expanded={expanded}>
        <div className="settings-grid">
          {/* FORMAT */}
          <div className="setting">
            <div className="setting-label">Format</div>
            <div className="chip-row">
              {FORMATS.map((f) => (
                <M3ChipAnimated
                  key={f}
                  selected={options.format === f}
                  onClick={() => update('format', f)}
                >
                  {f === 'best' ? 'Best' : f.toUpperCase()}
                </M3ChipAnimated>
              ))}
            </div>
            {options.format === 'best' && (
              <M3FadeIn delay={0.1}>
                <div className="setting-hint">
                  Downloads the highest quality video+audio available. No container preference — codec chosen
                  by source (AV1, VP9, H.264…)
                </div>
              </M3FadeIn>
            )}
          </div>

          {/* RESOLUTION */}
          <div className="setting">
            <div className="setting-label">Resolution</div>
            <div className="chip-row">
              {RESOLUTIONS.map((r) => (
                <M3ChipAnimated
                  key={r.value}
                  selected={options.resolution === r.value}
                  onClick={() => update('resolution', r.value)}
                  disabled={isAudio}
                  className="res-chip"
                >
                  <span className="res-label">{r.value}</span>
                  {r.dims && <span className="res-dims">{r.dims}</span>}
                </M3ChipAnimated>
              ))}
            </div>
            {isAudio && (
              <div className="setting-hint">Width is calculated automatically. Aspect ratio is always preserved.</div>
            )}
          </div>

          {/* BITRATE */}
          <div className="setting">
            <div className="setting-label">Bitrate (kbps)</div>
            <div className="chip-row">
              {BITRATES.map((b) => (
                <M3ChipAnimated
                  key={b}
                  selected={options.bitrate === b}
                  onClick={() => update('bitrate', b)}
                  disabled={isAudio}
                >
                  {b}
                </M3ChipAnimated>
              ))}
            </div>
          </div>

          {/* CONCURRENCY */}
          <div className="setting">
            <div className="setting-label">Concurrency</div>
            <div className="chip-row">
              {CONCURRENCIES.map((c) => (
                <M3ChipAnimated
                  key={c}
                  selected={String(options.concurrency) === c}
                  onClick={() => update('concurrency', Number(c))}
                >
                  {c}
                </M3ChipAnimated>
              ))}
            </div>
          </div>

          {/* ENGINE */}
          <div className="setting">
            <div className="setting-label">Engine</div>
            <div className="chip-row">
              {ENGINES.map((e) => (
                <M3ChipAnimated
                  key={e.value}
                  selected={options.engine === e.value}
                  onClick={() => update('engine', e.value)}
                >
                  {e.label}
                </M3ChipAnimated>
              ))}
            </div>
          </div>

          {/* THEME PREF */}
          <div className="setting">
            <div className="setting-label">Appearance</div>
            <div className="chip-row">
              {(['system', 'light', 'dark'] as const).map((t) => (
                <M3ChipAnimated
                  key={t}
                  selected={themePref === t}
                  onClick={() => setThemePref(t)}
                >
                  {t.charAt(0).toUpperCase() + t.slice(1)}
                </M3ChipAnimated>
              ))}
            </div>
          </div>

          {/* COOKIES + SWITCHES */}
          <div className="setting">
            <div className="setting-label">Cookies from</div>
            <div className="chip-row">
              {COOKIES.map((c) => (
                <M3ChipAnimated
                  key={c.value}
                  selected={options.cookies_browser === c.value}
                  onClick={() => update('cookies_browser', c.value)}
                >
                  {c.label}
                </M3ChipAnimated>
              ))}
            </div>
            <div className="m3-switch-row">
              <button
                className="m3-switch-item"
                onClick={() => toggleBool('embed_meta')}
                aria-pressed={options.embed_meta}
                type="button"
              >
                <M3SwitchAnimated on={options.embed_meta} />
                <span style={{ position: 'relative', zIndex: 2 }}>Embed metadata</span>
              </button>
              <button
                className="m3-switch-item"
                onClick={() => toggleBool('embed_thumb')}
                aria-pressed={options.embed_thumb}
                type="button"
              >
                <M3SwitchAnimated on={options.embed_thumb} />
                <span style={{ position: 'relative', zIndex: 2 }}>Embed thumbnail</span>
              </button>
              <button
                className="m3-switch-item"
                onClick={() => toggleBool('scrape_delay')}
                aria-pressed={options.scrape_delay}
                type="button"
              >
                <M3SwitchAnimated on={options.scrape_delay} />
                <span style={{ position: 'relative', zIndex: 2 }}>Random delay</span>
              </button>
            </div>
            {options.scrape_delay && (
              <M3FadeIn delay={0.1}>
                <div className="setting-hint">
                  Adds 1–5s delay between requests. Slower but avoids IP bans for large queues.
                </div>
              </M3FadeIn>
            )}
          </div>
        </div>
      </M3Expand>
    </section>
  );
}