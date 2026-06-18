/* ═══════════════════════════════════════════════════════════════
   Entropy DL — TypeScript Type Definitions
   ═══════════════════════════════════════════════════════════════ */

// ─── Job Status ───

export type JobStatus =
  | 'queued'
  | 'searching'
  | 'downloading'
  | 'processing'
  | 'done'
  | 'failed'
  | 'canceled';

// ─── Job ───

export interface Job {
  id: string;
  title: string;
  url: string;
  uploader: string;
  thumbnail: string;
  status: JobStatus;
  progress: number;
  speed: string;
  eta: string;
  output_file: string;
  error: string;
  options?: JobOptions;
}

// ─── Search Result ───

export interface SearchResult {
  id: string;
  title: string;
  url: string;
  uploader: string;
  thumbnail: string;
  duration: number;
  source: string;
}

// ─── Environment ───

export interface ToolInfo {
  found: boolean;
  version: string;
  path: string;
}

export interface EnvData {
  platform: string;
  distro: string;
  pkg_mgrs: string[];
  tools: Record<string, ToolInfo>;
  onboarding_done: boolean;
}

// ─── Settings ───

export interface Settings {
  audio_dir: string;
  video_dir: string;
  bandwidth_limit?: string;
  smart_routing?: boolean;
}

// ─── Job Options ───

export type Format = 'mp3' | 'm4a' | 'flac' | 'opus' | 'wav' | 'aac' | 'vorbis' | 'mp4' | 'webm' | 'mkv' | 'best';
export type Resolution = 'BEST' | '4K' | '1440p' | '1080p' | '720p' | '480p';
export type Engine = 'ytdlp' | 'aria2c';
export type CookiesBrowser = 'none' | 'chrome' | 'edge' | 'firefox' | 'brave' | 'chromium' | 'opera' | 'vivaldi' | 'safari';
export type Source = 'youtube' | 'ytmusic' | 'soundcloud' | 'everything';

export interface JobOptions {
  format: Format;
  bitrate: string;
  embed_meta: boolean;
  embed_thumb: boolean;
  engine: Engine;
  cookies_browser: CookiesBrowser;
  audio_dir: string;
  video_dir: string;
  scrape_delay: boolean;
  concurrency: number;
  resolution: Resolution;
  media_type?: string; // 'music' | 'audio' | 'video' when smart routing detected a type
}

// ─── SSE Events ───

export interface LogEntry {
  time: string;
  line: string;
  job_id: string;
}

export type SSEEventType = 'snapshot' | 'job' | 'log';

export interface SSESnapshotEvent {
  type: 'snapshot';
  jobs: Job[];
}

export interface SSEJobEvent {
  type: 'job';
  job: Job;
}

export interface SSELogEvent {
  type: 'log';
  log: LogEntry;
}

export type SSEEvent = SSESnapshotEvent | SSEJobEvent | SSELogEvent;

// ─── Toast ───

export interface Toast {
  msg: string;
  isErr: boolean;
}

// ─── Clear Jobs Response ───

export interface ClearJobsResponse {
  removed: number;
}

// ─── Component Props ───

export interface AsciiSpinnerProps {
  active?: boolean;
  label?: string;
}

export interface JobCardProps {
  job: Job;
  onRetry: (id: string) => Promise<void>;
  onRemove: (id: string) => Promise<void>;
  onOpenFolder: (id: string) => Promise<void>;
}

export interface LinksPanelProps {
  onQueue: (urls: string[]) => void;
}

export interface LogDrawerProps {
  logs: LogEntry[];
}

export interface SearchPanelProps {
  source: Source;
  selected: Record<string, boolean>;
  onToggle: (item: SearchResult) => void;
  onAddAll: (items: SearchResult[]) => void;
  onRemoveAll: (items: SearchResult[]) => void;
  onQueueSelected: (items: SearchResult[]) => Promise<void>;
}

export type ThemePref = 'system' | 'light' | 'dark';

export interface SettingsPanelProps {
  options: JobOptions;
  setOptions: React.Dispatch<React.SetStateAction<JobOptions>>;
  themePref: ThemePref;
  setThemePref: (t: ThemePref) => void;
  smartRouting: boolean;
  setSmartRouting: (enabled: boolean) => void;
}

export interface WelcomeOverlayProps {
  env: EnvData;
  onComplete: () => void;
}

export interface ToolsStatusProps {
  // No external props — fetches its own data
}

export type QuitButtonProps = Record<string, never>;

// ─── App-level state types ───

export interface TabItem {
  id: Source | 'links';
  label: string;
}

export interface QueueStats {
  total: number;
  active: number;
  done: number;
  failed: number;
}

export interface ToolsData {
  platform: string;
  distro: string;
  pkg_mgrs: string[];
  [key: string]: ToolInfo | string | string[] | boolean;
}
