import { useState, useEffect, useCallback, useMemo } from 'react';
import { motion } from 'framer-motion';
import { AnimatePresence, M3FadeIn, M3Stagger, M3StaggerItem, M3CircularProgress, M3ScaleIn } from './m3';
import { Ripple } from './Ripple';
import { fetchLibraryDir, libraryFileURL } from '../lib/api';
import type { LibraryEntry, LibraryRoot } from '../types';

// Audio/video file extensions for icon selection.
const AUDIO_EXTS = new Set(['mp3', 'm4a', 'flac', 'wav', 'opus', 'aac', 'ogg']);
const VIDEO_EXTS = new Set(['mp4', 'mkv', 'webm', 'avi', 'mov', 'flv']);
const IMAGE_EXTS = new Set(['jpg', 'jpeg', 'png', 'webp', 'gif']);

interface PlayerState {
  entry: LibraryEntry;
  root: LibraryRoot;
}

function formatSize(bytes: number): string {
  if (bytes === 0) return '—';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  const value = bytes / Math.pow(1024, i);
  return `${value.toFixed(value < 10 && i > 0 ? 1 : 0)} ${units[i]}`;
}

function formatDate(iso: string): string {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  // Relative-ish: just show month/day/year if older, else time.
  const now = new Date();
  const sameDay = d.toDateString() === now.toDateString();
  if (sameDay) {
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  }
  return d.toLocaleDateString([], { year: 'numeric', month: 'short', day: 'numeric' });
}

function iconForEntry(entry: LibraryEntry): string {
  if (entry.is_dir) return 'folder';
  const ext = entry.ext;
  if (AUDIO_EXTS.has(ext)) return 'music_note';
  if (VIDEO_EXTS.has(ext)) return 'movie';
  if (IMAGE_EXTS.has(ext)) return 'image';
  return 'draft';
}

export default function LibraryPanel() {
  const [root, setRoot] = useState<LibraryRoot>('audio');
  const [entries, setEntries] = useState<LibraryEntry[]>([]);
  const [path, setPath] = useState(''); // current subdirectory relative to root
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [player, setPlayer] = useState<PlayerState | null>(null);

  // Breadcrumb segments derived from the current path.
  const breadcrumbs = useMemo(() => {
    const segs = path ? path.split('/').filter(Boolean) : [];
    return [{ name: 'Library', path: '' }, ...segs.map((s, i) => ({
      name: s,
      path: segs.slice(0, i + 1).join('/'),
    }))];
  }, [path]);

  const loadDir = useCallback(async (targetRoot: LibraryRoot, targetPath: string) => {
    setLoading(true);
    setError(null);
    try {
      const result = targetPath === ''
        // Root listing comes from the overview endpoint (avoids a second call
        // when we already have both roots' contents). But for subdirectories
        // we must use /api/library/dir.
        ? await fetchLibraryDir(targetRoot, '')
        : await fetchLibraryDir(targetRoot, targetPath);
      setEntries(result);
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : 'Failed to load directory';
      setError(msg);
      setEntries([]);
    } finally {
      setLoading(false);
    }
  }, []);

  // Reload whenever root or path changes.
  useEffect(() => {
    loadDir(root, path);
  }, [root, path, loadDir]);

  // When switching the audio/video tab, reset to the root of that tab.
  const switchRoot = (next: LibraryRoot) => {
    setRoot(next);
    setPath('');
  };

  const navigateTo = (targetPath: string) => {
    setPath(targetPath);
  };

  const openEntry = (entry: LibraryEntry) => {
    if (entry.is_dir) {
      navigateTo(entry.path);
      return;
    }
    // Only open playable media in the inline player.
    if (AUDIO_EXTS.has(entry.ext) || VIDEO_EXTS.has(entry.ext) || IMAGE_EXTS.has(entry.ext)) {
      setPlayer({ entry, root });
    }
  };

  const isPlayable = (entry: LibraryEntry): boolean => {
    if (entry.is_dir) return false;
    return AUDIO_EXTS.has(entry.ext) || VIDEO_EXTS.has(entry.ext) || IMAGE_EXTS.has(entry.ext);
  };

  return (
    <section className="library" data-testid="library-panel">
      <div className="library-header">
        <span className="library-title">Your library</span>
      </div>

      {/* Audio / Video tab switch */}
      <div className="library-tabs" role="tablist">
        {(['audio', 'video'] as const).map((r) => (
          <button
            key={r}
            role="tab"
            aria-selected={root === r}
            className={`tab${root === r ? ' active' : ''}`}
            onClick={() => switchRoot(r)}
            type="button"
          >
            {r === 'audio' ? 'Audio' : 'Video'}
          </button>
        ))}
      </div>

      {/* Breadcrumb navigation */}
      {path !== '' && (
        <nav className="library-breadcrumb" aria-label="Directory path">
          {breadcrumbs.map((seg, i) => {
            const isLast = i === breadcrumbs.length - 1;
            return (
              <span key={seg.path || 'root'} className="library-crumb-wrap">
                <button
                  className={`library-crumb${isLast ? ' current' : ''}`}
                  onClick={() => navigateTo(seg.path)}
                  disabled={isLast}
                  type="button"
                >
                  {seg.name}
                </button>
                {!isLast && <span className="library-crumb-sep" aria-hidden="true">/</span>}
              </span>
            );
          })}
        </nav>
      )}

      {/* Content */}
      {loading ? (
        <div className="library-loading">
          <M3CircularProgress />
          <span>Loading…</span>
        </div>
      ) : error ? (
        <div className="library-empty">
          <span className="empty-icon" aria-hidden="true">error_outline</span>
          <span>{error}</span>
        </div>
      ) : entries.length === 0 ? (
        <div className="library-empty">
          <span className="empty-icon" aria-hidden="true">{root === 'audio' ? 'library_music' : 'movie'}</span>
          <span>No {root} files yet</span>
          <span className="empty-body">Downloaded {root} files will appear here</span>
        </div>
      ) : (
        <M3Stagger className="library-list" staggerDelay={0.03}>
          <AnimatePresence mode="popLayout">
            {entries.map((entry) => (
              <M3StaggerItem key={entry.path}>
                <motion.button
                  className={`library-entry${entry.is_dir ? ' dir' : ''}${isPlayable(entry) ? ' playable' : ''}`}
                  onClick={() => openEntry(entry)}
                  type="button"
                  title={entry.is_dir ? `Open ${entry.name}` : `Play ${entry.name}`}
                  whileTap={{ scale: 0.98 }}
                  transition={{ type: 'spring', stiffness: 500, damping: 30 }}
                >
                  <span className="library-entry-icon" aria-hidden="true">
                    {iconForEntry(entry)}
                  </span>
                  <span className="library-entry-info">
                    <span className="library-entry-name">{entry.name}</span>
                    <span className="library-entry-meta">
                      {entry.is_dir ? 'Folder' : formatSize(entry.size)}
                      {entry.mod_time && ` · ${formatDate(entry.mod_time)}`}
                    </span>
                  </span>
                  {!entry.is_dir && isPlayable(entry) && (
                    <span className="library-entry-action" aria-hidden="true">
                      play_arrow
                    </span>
                  )}
                  <Ripple />
                </motion.button>
              </M3StaggerItem>
            ))}
          </AnimatePresence>
        </M3Stagger>
      )}

      {/* Inline media player */}
      <AnimatePresence>
        {player && (
          <M3ScaleIn>
            <div
              className="library-player-overlay"
              onClick={() => setPlayer(null)}
              role="dialog"
              aria-modal="true"
              aria-label={`Playing ${player.entry.name}`}
            >
              <div className="library-player" onClick={(e) => e.stopPropagation()}>
                <div className="library-player-header">
                  <span className="library-player-title" title={player.entry.name}>
                    {player.entry.name}
                  </span>
                  <button
                    className="btn icon"
                    onClick={() => setPlayer(null)}
                    aria-label="Close player"
                    type="button"
                  >
                    <span className="md-icon" aria-hidden="true">close</span>
                    <Ripple />
                  </button>
                </div>
                <M3FadeIn className="library-player-body">
                  {VIDEO_EXTS.has(player.entry.ext) ? (
                    <video
                      className="library-player-video"
                      src={libraryFileURL(player.root, player.entry.path)}
                      controls
                      autoPlay
                    />
                  ) : IMAGE_EXTS.has(player.entry.ext) ? (
                    <img
                      className="library-player-image"
                      src={libraryFileURL(player.root, player.entry.path)}
                      alt={player.entry.name}
                    />
                  ) : (
                    <audio
                      className="library-player-audio"
                      src={libraryFileURL(player.root, player.entry.path)}
                      controls
                      autoPlay
                    />
                  )}
                </M3FadeIn>
              </div>
            </div>
          </M3ScaleIn>
        )}
      </AnimatePresence>
    </section>
  );
}
