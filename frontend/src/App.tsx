import { useState, useEffect, useRef, useMemo } from 'react';
import { connectSSE } from './lib/utils';
import {

  fetchJobs,
  fetchConcurrency,
  updateConcurrency,
  createJobs,
  retryJob,
  deleteJob,
  clearJobs,
  completeOnboarding,
} from './lib/api';
import {
  M3ViewTransition,
  M3ToastAnimated,
  AnimatePresence,
} from './components/m3';
import ErrorBoundary from './components/ErrorBoundary';
import InfoModal from './components/InfoModal';
import QuitButton from './components/QuitButton';
import OutputDirBar from './components/OutputDirBar';
import ToolsStatus from './components/ToolsStatus';
import ThemeSync from './components/ThemeSync';
import MeshBackground from './components/MeshBackground';
import NavigationRail from './components/NavigationRail';
import SearchPanel from './components/SearchPanel';
import LinksPanel from './components/LinksPanel';
import JobCard from './components/JobCard';
import SettingsPanel from './components/SettingsPanel';
import LogDrawer from './components/LogDrawer';
type ViewId = 'search' | 'queue' | 'settings' | 'log';

import type {
  Job,
  SearchResult,
  SSEEvent,
  SSESnapshotEvent,
  SSEJobEvent,
  SSELogEvent,
  LogEntry,
  Toast,

  JobOptions,
  ThemePref,
  TabItem,
  QueueStats,
  Source,
} from './types';

const SOURCE_TABS: TabItem[] = [
  { id: 'everything', label: 'Search' },
  { id: 'links', label: 'Paste links' },
];

type SourceTab = Source | 'links';

const DEFAULT_OPTIONS: JobOptions = {
  format: 'mp3',
  bitrate: '320',
  embed_meta: true,
  embed_thumb: true,
  engine: 'ytdlp',
  cookies_browser: 'none',
  output_dir: '',
  scrape_delay: false,
  concurrency: 2,
  resolution: 'BEST',
};

export default function App() {
  const [activeView, setActiveView] = useState<ViewId>('search');
  const [sourceTab, setSourceTab] = useState<SourceTab>('everything');
  const [selected, setSelected] = useState<Record<string, boolean>>({});
  const [jobs, setJobs] = useState<Job[]>([]);
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [toast, setToast] = useState<Toast | null>(null);
  const [options, setOptions] = useState<JobOptions>(DEFAULT_OPTIONS);
  const [themePref, setThemePrefState] = useState<ThemePref>(() => {
    return (localStorage.getItem('theme_pref') as ThemePref) || 'system';
  });
  // showWelcome removed — overlay eliminated
  const [showInfo, setShowInfo] = useState(false);
  const [, setAppReady] = useState(false);

  const setThemePref = (pref: ThemePref) => {
    localStorage.setItem('theme_pref', pref);
    setThemePrefState(pref);
  };

  const concurrencyReady = useRef(false);
  const lastConcurrency = useRef(options.concurrency);
  const sseCleanup = useRef<(() => void) | null>(null);
  const toastTimeout = useRef<ReturnType<typeof setTimeout> | null>(null);

  // ─── Toast ───

  const showToast = (msg: string, isErr = false): void => {
    if (toastTimeout.current) clearTimeout(toastTimeout.current);
    setToast({ msg, isErr });
    toastTimeout.current = setTimeout(() => setToast(null), 3500);
  };

  // ─── Init ───

  useEffect(() => {
    fetchJobs().then((j) => {
      setJobs(j);
      setAppReady(true);
    }).catch(() => {
      setAppReady(true);
    });

    fetchConcurrency().then((workers) => {
      setOptions((prev) => ({ ...prev, concurrency: workers }));
      concurrencyReady.current = true;
      lastConcurrency.current = workers;
    }).catch(() => {
      // Use default concurrency
      concurrencyReady.current = true;
    });

    sseCleanup.current = connectSSE((event: SSEEvent) => {
      if (event.type === 'snapshot') {
        setJobs((event as SSESnapshotEvent).jobs || []);
      } else if (event.type === 'job') {
        const job = (event as SSEJobEvent).job;
        if (!job) return;
        setJobs((prev) => {
          const idx = prev.findIndex((j) => j.id === job.id);
          if (idx >= 0) {
            const next = [...prev];
            next[idx] = job;
            return next;
          }
          return [...prev, job];
        });
      } else if (event.type === 'log') {
        const log = (event as SSELogEvent).log;
        if (!log) return;
        setLogs((prev) => {
          const next = [...prev, log];
          if (next.length > 500) return next.slice(next.length - 500);
          return next;
        });
      }
    });

    return () => {
      if (sseCleanup.current) sseCleanup.current();
    };
  }, []);

  // ─── Sync concurrency ───

  useEffect(() => {
    if (concurrencyReady.current && options.concurrency !== lastConcurrency.current) {
      lastConcurrency.current = options.concurrency;
      updateConcurrency(options.concurrency).catch(() => {
        showToast('Failed to update concurrency limit', true);
      });
    }
  }, [options.concurrency]);

  // ─── Stats ───

  const stats: QueueStats = useMemo(() => {
    const s: QueueStats = { total: jobs.length, active: 0, done: 0, failed: 0 };
    jobs.forEach((j) => {
      if (j.status === 'done') s.done++;
      else if (j.status === 'failed' || j.status === 'canceled') s.failed++;
      else if (['downloading', 'processing', 'queued'].includes(j.status)) s.active++;
    });
    return s;
  }, [jobs]);

  // ─── Selection ───

  const toggleSelection = (item: SearchResult): void => {
    const key = item.id || item.url;
    setSelected((prev) => {
      const next = { ...prev };
      if (next[key]) delete next[key];
      else next[key] = true;
      return next;
    });
  };

  const selectAllItems = (items: SearchResult[]): void => {
    setSelected((prev) => {
      const next = { ...prev };
      items.forEach((item) => {
        next[item.id || item.url] = true;
      });
      return next;
    });
  };

  const deselectAllItems = (items: SearchResult[]): void => {
    setSelected((prev) => {
      const next = { ...prev };
      items.forEach((item) => {
        delete next[item.id || item.url];
      });
      return next;
    });
  };

  // ─── Queue actions ───

  const queueSelected = async (items: SearchResult[]): Promise<void> => {
    try {
      const directItems = items.map((item) => ({
        url: item.url,
        title: item.title,
        uploader: item.uploader,
        thumbnail: item.thumbnail,
        duration: item.duration,
      }));
      const created = await createJobs({ items: directItems, options });
      showToast(`Queued ${created.length} item(s)`);
      setSelected({});
    } catch {
      showToast('Queue failed', true);
    }
  };

  const queueUrls = async (urls: string[]): Promise<void> => {
    try {
      const created = await createJobs({ urls, options });
      showToast(`Queued ${created.length} item(s)`);
    } catch {
      showToast('Queue failed', true);
    }
  };

  const handleRetryJob = async (id: string): Promise<void> => {
    try { await retryJob(id); } catch { showToast('Retry failed', true); }
  };

  const handleRemoveJob = async (id: string): Promise<void> => {
    try { await deleteJob(id); } catch { showToast('Remove failed', true); }
  };

  const handleOpenFolder = async (id: string): Promise<void> => {
    try {
      await fetch(`/api/jobs/${id}/open-folder`, { method: 'POST' });
    } catch { showToast('Could not open folder', true); }
  };

  const handleClearJobs = async (what: string): Promise<void> => {
    try {
      const data = await clearJobs(what);
      showToast(`Cleared ${data.removed} ${what}`);
    } catch { showToast('Clear failed', true); }
  };

  const hasActiveJobs = jobs.some((j) =>
    ['downloading', 'processing', 'queued', 'searching'].includes(j.status)
  );

  const handleViewChange = (view: ViewId): void => {
    setActiveView(view);
  };

  return (
    <ErrorBoundary>
    <>
      <ThemeSync themePref={themePref} />
      <MeshBackground />

      {/* WelcomeOverlay removed — tool status shown in header via ToolsStatus */}

      {/* Animated Toast */}
      <M3ToastAnimated show={!!toast} isError={toast?.isErr}>
        {toast?.msg}
      </M3ToastAnimated>

      <div className="app app-rail-layout">
        {/* ─── Navigation Rail ─── */}
        <NavigationRail
          active={activeView}
          onChange={handleViewChange}
          queueCount={stats.active + stats.failed}
        />

        {/* ─── Content Area ─── */}
        <div className="rail-content">
          {/* ─── Small Top App Bar ─── */}
          <header className="rail-header">
            <div className="rail-header-left">
              <span className="rail-header-title">Entropy</span>
              <OutputDirBar onToast={showToast} />
            </div>
            <div className="rail-header-right">
              <button
                className="btn ghost"
                onClick={() => setShowInfo(true)}
                aria-label="About"
                type="button"
              >
                <span className="md-icon" aria-hidden="true">info</span>
              </button>
              <ToolsStatus />
              <QuitButton />
            </div>
          </header>

          {/* ─── View Content with animated transitions ─── */}
          <main className="rail-body">
            {activeView === 'search' && (
              <M3ViewTransition keyProp="search">
                <>
                  <div className="tabs">
                    {SOURCE_TABS.map((tab) => (
                      <button
                        key={tab.id}
                        className={`tab${sourceTab === tab.id ? ' active' : ''}`}
                        onClick={() => setSourceTab(tab.id)}
                      >
                        {tab.label}
                      </button>
                    ))}
                  </div>
                  {sourceTab !== 'links' ? (
                    <SearchPanel
                      source={sourceTab as Source}
                      selected={selected}
                      onToggle={toggleSelection}
                      onAddAll={selectAllItems}
                      onRemoveAll={deselectAllItems}
                      onQueueSelected={queueSelected}
                    />
                  ) : (
                    <LinksPanel onQueue={queueUrls} />
                  )}
                </>
              </M3ViewTransition>
            )}

            {activeView === 'queue' && (
              <M3ViewTransition keyProp="queue">
                <>
                  <div className="panel-header">
                    <span>Download queue</span>
                    <div className="queue-controls">
                      {stats.done > 0 && (
                        <button className="btn ghost" onClick={() => handleClearJobs('completed')}>
                          Clear done
                        </button>
                      )}
                      {stats.failed > 0 && (
                        <button className="btn ghost" onClick={() => handleClearJobs('failed')}>
                          Clear failed
                        </button>
                      )}
                    </div>
                  </div>
                  {jobs.length === 0 ? (
                    <div className="queue-empty">
                      <span className="empty-icon" aria-hidden="true">cloud_download</span>
                      <span>No downloads yet</span>
                      <span className="empty-body">Search for music or videos and queue them for download</span>
                    </div>
                  ) : (
                    <div style={{ padding: '0 var(--sp-2)' }}>
                      <AnimatePresence mode="popLayout">
                        {jobs.map((job) => (
                          <JobCard
                            key={job.id}
                            job={job}
                            onRetry={handleRetryJob}
                            onRemove={handleRemoveJob}
                            onOpenFolder={handleOpenFolder}
                          />
                        ))}
                      </AnimatePresence>
                    </div>
                  )}
                </>
              </M3ViewTransition>
            )}

            {activeView === 'settings' && (
              <M3ViewTransition keyProp="settings">
                <SettingsPanel 
                  options={options} 
                  setOptions={setOptions} 
                  themePref={themePref} 
                  setThemePref={setThemePref} 
                />
              </M3ViewTransition>
            )}

            {activeView === 'log' && (
              <M3ViewTransition keyProp="log">
                <LogDrawer logs={logs} />
              </M3ViewTransition>
            )}
          </main>

          {/* ─── Bottom Status Bar ─── */}
          <div className="rail-statusbar">
            <span className={`stat-dot ${hasActiveJobs ? 'active' : ''}`} />
            <span className="rail-status-text">
              {hasActiveJobs ? `${stats.active} active` : 'Idle'}
              {stats.done > 0 && ` · ${stats.done} done`}
              {stats.failed > 0 && ` · ${stats.failed} failed`}
            </span>
          </div>
        </div>
      </div>

      <InfoModal open={showInfo} onClose={() => setShowInfo(false)} />
    </>
    </ErrorBoundary>
  );
}