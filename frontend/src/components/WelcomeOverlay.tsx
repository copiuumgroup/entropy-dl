import { useState, useEffect, useRef, useCallback } from 'react';
import { createPortal } from 'react-dom';
import { fetchEnv, fetchSettings, saveOutputDirs, completeOnboarding, updateTools } from '../lib/api';
import {
  M3ViewTransition,
  M3FadeIn,
  M3Stagger,
  M3StaggerItem,
} from './m3';
import type { EnvData, Settings, ToolInfo } from '../types';

const TOOL_DISPLAY: Record<string, string> = { yt_dlp: 'yt-dlp', aria2c: 'aria2c', ffmpeg: 'ffmpeg' };
const DOWNLOAD_URLS: Record<string, string> = {
  yt_dlp: 'https://github.com/yt-dlp/yt-dlp/releases',
  aria2c: 'https://github.com/aria2/aria2/releases',
  ffmpeg: 'https://www.gyan.dev/ffmpeg/builds/',
};
const TOOLS = ['yt_dlp', 'aria2c', 'ffmpeg'] as const;

interface WelcomeOverlayProps {
  open: boolean;
  onClose: () => void;
  onOpenInfo: () => void;
}

function StepWelcome({ env }: { env: EnvData | null }) {
  const platformLabel = env
    ? env.platform === 'windows'
      ? 'Windows'
      : env.platform === 'darwin'
        ? 'macOS'
        : (env.distro || 'Linux')
    : null;

  const platformFull = platformLabel
    ? `Running on ${platformLabel}${env?.platform === 'windows' ? '' : ` (${env?.distro || ''})`}`
    : null;

  return (
    <M3Stagger staggerDelay={0.08}>
      <M3StaggerItem>
        <h2 className="welcome-hero-title">Entropy DL</h2>
      </M3StaggerItem>
      <M3StaggerItem>
        <p className="welcome-hero-sub">
          Beautiful, local, private media downloader.
        </p>
      </M3StaggerItem>
      <ul className="welcome-bullets">
        <M3StaggerItem>
          <li className="welcome-bullet">
            <span className="md-icon welcome-bullet-icon" aria-hidden="true">rocket_launch</span>
            <span>Single binary — everything embedded, no runtime dependencies</span>
          </li>
        </M3StaggerItem>
        <M3StaggerItem>
          <li className="welcome-bullet">
            <span className="md-icon welcome-bullet-icon" aria-hidden="true">lock</span>
            <span>Local &amp; private — runs on localhost, your files never leave this machine</span>
          </li>
        </M3StaggerItem>
        <M3StaggerItem>
          <li className="welcome-bullet">
            <span className="md-icon welcome-bullet-icon" aria-hidden="true">palette</span>
            <span>Material You theming — adapts to your system accent color</span>
          </li>
        </M3StaggerItem>
        <M3StaggerItem>
          <li className="welcome-bullet">
            <span className="md-icon welcome-bullet-icon" aria-hidden="true">speed</span>
            <span>Real-time progress — live speed, ETA, and status updates</span>
          </li>
        </M3StaggerItem>
      </ul>
      {platformFull && (
        <M3StaggerItem>
          <p className="welcome-platform">{platformFull}</p>
        </M3StaggerItem>
      )}
    </M3Stagger>
  );
}

function StepTools({
  env,
  updating,
  onUpdateYtdlp,
}: {
  env: EnvData | null;
  updating: boolean;
  onUpdateYtdlp: () => void;
}) {
  const allFound = env
    ? TOOLS.every((t) => (env.tools[t] as ToolInfo)?.found)
    : false;
  const missingCount = env
    ? TOOLS.filter((t) => !(env.tools[t] as ToolInfo)?.found).length
    : 0;

  const getInstallHint = (toolKey: string): string | null => {
    if (!env) return null;
    if (env.platform === 'windows') {
      const cmds: Record<string, string> = {
        yt_dlp: 'winget install yt-dlp.yt-dlp',
        aria2c: 'winget install aria2.aria2',
        ffmpeg: 'winget install Gyan.FFmpeg',
      };
      return cmds[toolKey] || null;
    }
    if (env.platform === 'darwin') return `brew install ${TOOL_DISPLAY[toolKey]}`;
    if (env.pkg_mgrs?.includes('pacman')) return `sudo pacman -S ${TOOL_DISPLAY[toolKey]}`;
    return `sudo apt install ${TOOL_DISPLAY[toolKey]}`;
  };

  return (
    <>
      {env && (
        <div className={`welcome-tools-banner ${allFound ? 'all-ok' : 'some-missing'}`}>
          {allFound
            ? '✓ All tools ready — you\'re good to go!'
            : `⚠ ${missingCount} tool${missingCount > 1 ? 's' : ''} missing — downloads may not work`}
        </div>
      )}
      {TOOLS.map((toolKey) => {
        const t = env?.tools[toolKey] as ToolInfo | undefined;
        const found = t?.found ?? false;
        const hint = found ? null : getInstallHint(toolKey);
        return (
          <div key={toolKey} className="welcome-tool-card">
            <span className={`md-icon welcome-tool-state ${found ? 'ok' : 'missing'}`}>
              {found ? 'check' : 'warning'}
            </span>
            <div className="welcome-tool-info">
              <div className="welcome-tool-name">{TOOL_DISPLAY[toolKey]}</div>
              {found && t?.version && (
                <div className="welcome-tool-ver">v{t.version}</div>
              )}
              {hint && <div className="welcome-tool-hint"><code>{hint}</code></div>}
            </div>
            <div className="welcome-tool-actions">
              {found && toolKey === 'yt_dlp' && (
                <button
                  className="btn ghost"
                  onClick={onUpdateYtdlp}
                  disabled={updating}
                  type="button"
                >
                  {updating ? 'Updating…' : 'Update'}
                </button>
              )}
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
          </div>
        );
      })}
    </>
  );
}

function StepDirs({
  audioDir,
  videoDir,
  setAudioDir,
  setVideoDir,
  saving,
  onSave,
}: {
  audioDir: string;
  videoDir: string;
  setAudioDir: (d: string) => void;
  setVideoDir: (d: string) => void;
  saving: boolean;
  onSave: () => void;
}) {
  return (
    <M3FadeIn delay={0.05}>
      <p style={{ font: 'var(--md-type-body-medium)', color: 'var(--md-on-surface-variant)', margin: '0 0 var(--sp-4)' }}>
        Choose where your downloads go. Audio formats land in the audio folder;
        video formats land in video. You can change these anytime in Settings.
      </p>
      <div className="welcome-dir-row">
        <span className="welcome-dir-label">Audio</span>
        <input
          className="welcome-dir-input"
          placeholder="e.g. ~/Music/Entropy"
          value={audioDir}
          onChange={(e) => setAudioDir(e.target.value)}
        />
      </div>
      <div className="welcome-dir-row">
        <span className="welcome-dir-label">Video</span>
        <input
          className="welcome-dir-input"
          placeholder="e.g. ~/Videos/Entropy"
          value={videoDir}
          onChange={(e) => setVideoDir(e.target.value)}
        />
      </div>
      <div style={{ display: 'flex', justifyContent: 'center', marginTop: 'var(--sp-3)' }}>
        <button
          className="btn tonal"
          onClick={onSave}
          disabled={saving}
          type="button"
        >
          {saving ? 'Saving…' : 'Save folders'}
        </button>
      </div>
    </M3FadeIn>
  );
}

export default function WelcomeOverlay({ open, onClose, onOpenInfo }: WelcomeOverlayProps) {
  const [step, setStep] = useState(0);
  const [env, setEnv] = useState<EnvData | null>(null);
  const [audioDir, setAudioDir] = useState('');
  const [videoDir, setVideoDir] = useState('');
  const [updating, setUpdating] = useState(false);
  const [saving, setSaving] = useState(false);
  const dialogRef = useRef<HTMLDivElement>(null);
  const prevFocusRef = useRef<HTMLElement | null>(null);

  // Fetch env and settings on mount
  useEffect(() => {
    if (!open) return;
    fetchEnv().then(setEnv).catch(() => {});
    fetchSettings().then((s) => {
      setAudioDir(s.audio_dir || '');
      setVideoDir(s.video_dir || '');
    }).catch(() => {});
    setStep(0);
  }, [open]);

  // Focus trap
  useEffect(() => {
    if (!open) return;
    prevFocusRef.current = document.activeElement as HTMLElement;
    const timer = setTimeout(() => dialogRef.current?.focus(), 50);

    const handleFocus = (e: FocusEvent) => {
      if (!dialogRef.current?.contains(e.target as Node)) {
        dialogRef.current?.focus();
      }
    };
    document.addEventListener('focusin', handleFocus);
    return () => {
      clearTimeout(timer);
      document.removeEventListener('focusin', handleFocus);
      prevFocusRef.current?.focus();
    };
  }, [open]);

  // Escape to close
  useEffect(() => {
    if (!open) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        onClose();
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [open, onClose]);

  const handleUpdateYtdlp = useCallback(async () => {
    setUpdating(true);
    try {
      await updateTools();
      const data = await fetchEnv();
      setEnv(data);
    } catch {
      // ignore
    }
    setUpdating(false);
  }, []);

  const handleSaveDirs = useCallback(async () => {
    setSaving(true);
    try {
      await saveOutputDirs({ audio_dir: audioDir, video_dir: videoDir });
    } catch {
      // ignore
    }
    setSaving(false);
  }, [audioDir, videoDir]);

  const handleGetStarted = useCallback(async () => {
    try {
      // Save dirs first if non-empty
      if (audioDir || videoDir) {
        await saveOutputDirs({ audio_dir: audioDir, video_dir: videoDir });
      }
      await completeOnboarding();
    } catch {
      // ignore — user can still close
    }
    onClose();
  }, [audioDir, videoDir, onClose]);

  const handleNext = () => {
    if (step < 2) setStep(step + 1);
    else handleGetStarted();
  };

  if (!open) return null;

  const content = (
    <div className="m3-dialog-portal">
      <div className="m3-dialog-scrim" onClick={onClose} />
      <div
        className="m3-dialog"
        role="dialog"
        aria-modal="true"
        aria-label="Welcome to Entropy DL"
        tabIndex={-1}
        ref={dialogRef}
        style={{ width: 'min(92vw, 36rem)', padding: 'var(--sp-6) var(--sp-6) var(--sp-5)' }}
      >
        {/* Step dots */}
        <div className="welcome-dots">
          {[0, 1, 2].map((i) => (
            <div key={i} className={`welcome-dot ${i === step ? 'active' : ''}`} />
          ))}
        </div>

        {/* Step content with crossfade transition */}
        <div style={{ minHeight: '240px' }}>
        <M3ViewTransition keyProp={`welcome-step-${step}`}>
          {step === 0 && <StepWelcome env={env} />}
          {step === 1 && (
            <StepTools
              env={env}
              updating={updating}
              onUpdateYtdlp={handleUpdateYtdlp}
            />
          )}
          {step === 2 && (
            <StepDirs
              audioDir={audioDir}
              videoDir={videoDir}
              setAudioDir={setAudioDir}
              setVideoDir={setVideoDir}
              saving={saving}
              onSave={handleSaveDirs}
            />
          )}
        </M3ViewTransition>
        </div>

        {/* Navigation buttons */}
        <div className="m3-dialog-actions" style={{ justifyContent: 'space-between', marginTop: 'var(--sp-3)' }}>
          <div style={{ display: 'flex', gap: 'var(--sp-2)' }}>
            {step > 0 && (
              <button className="btn text" onClick={() => setStep(step - 1)} type="button">
                Back
              </button>
            )}
            {step < 2 && (
              <button className="btn text" onClick={onClose} type="button">
                Skip
              </button>
            )}
          </div>
          <div style={{ display: 'flex', gap: 'var(--sp-2)' }}>
            {step < 2 && (
              <button className="btn tonal" onClick={handleNext} type="button">
                Next
              </button>
            )}
            {step === 2 && (
              <button className="btn primary" onClick={handleGetStarted} type="button">
                Get Started
              </button>
            )}
          </div>
        </div>

        {/* Learn more link */}
        <div style={{ textAlign: 'center', marginTop: 'var(--sp-2)' }}>
          <button
            className="btn ghost"
            onClick={onOpenInfo}
            style={{ font: 'var(--md-type-label-small)', opacity: 0.7 }}
            type="button"
          >
            Learn more about Entropy DL
          </button>
        </div>
      </div>
    </div>
  );

  return createPortal(content, document.body);
}
