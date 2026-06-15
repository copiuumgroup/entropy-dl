import { useState, useRef, type FormEvent } from 'react';
import { searchItems } from '../lib/api';
import { formatDuration } from '../lib/utils';
import {
  M3Stagger,
  M3StaggerItem,
  M3FadeIn,
  M3Skeleton,
  M3SelectionBar,
} from './m3';
import { CircularProgress } from './CircularProgress';
import { LinearProgress } from './LinearProgress';
import type { SearchPanelProps, SearchResult } from '../types';

const PLACEHOLDERS: Record<string, string> = {
  youtube: 'Search YouTube…',
  ytmusic: 'Search YouTube Music…',
  soundcloud: 'Search SoundCloud…',
};

export default function SearchPanel({ source, selected, onToggle, onAddAll, onRemoveAll, onQueueSelected }: SearchPanelProps) {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [limit, setLimit] = useState(20);
  const [loadingMore, setLoadingMore] = useState(false);

  const handleSearch = async (e: FormEvent | null, newLimit?: number, append = false): Promise<void> => {
    if (e) e.preventDefault();
    const q = query.trim();
    if (!q) return;
    setLoading(append ? false : true);
    setError('');
    try {
      const res = await searchItems(source, q, newLimit || limit);
      if (append) {
        // Backend returns first N results from start — skip the ones we already have
        const newItems = res.slice(results.length);
        setResults((prev) => [...prev, ...newItems]);
      } else {
        setResults(res);
      }
      setLimit(newLimit || 20);
    } catch (err: unknown) {
      const errorObj = err as { response?: { data?: { error?: string } }; message?: string };
      setError(errorObj?.response?.data?.error || errorObj?.message || 'Search failed');
      if (!append) setResults([]);
    }
    setLoading(false);
    setLoadingMore(false);
  };

  const loadMoreGuard = useRef(false);
  const loadMore = (): void => {
    if (loadMoreGuard.current) return;
    loadMoreGuard.current = true;
    setLoadingMore(true);
    const nextLimit = limit + 20;
    handleSearch(null, nextLimit, true).finally(() => {
      loadMoreGuard.current = false;
    });
  };

  const selectedCount = results.filter((r) => selected[r.id || r.url]).length;

  return (
    <>
      {/* Indeterminate linear progress at top while loading */}
      {loading && <LinearProgress determinate={false} color="var(--md-status-downloading)" />}

      <form
        className="search-row"
        onSubmit={handleSearch}
        data-testid={`search-form-${source}`}
      >
        <span className="search-icon" aria-hidden="true">search</span>
        <input
          className="search-input"
          placeholder={PLACEHOLDERS[source] || 'Search…'}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          aria-label={`Search ${source}`}
          data-testid={`search-input-${source}`}
        />
        <button
          type="submit"
          className="btn primary"
          disabled={loading || !query.trim()}
          aria-label="Submit search"
          data-testid={`search-submit-${source}`}
        >
          {loading ? <CircularProgress size={20} thickness={3} /> : 'Search'}
        </button>
      </form>

      <div className="results-meta">
        <span style={{ display: 'flex', alignItems: 'center', gap: 'var(--sp-2)' }}>
          {!loading && `${results.length} results · ${selectedCount} selected`}
        </span>
        {results.length > 0 && (
          <div style={{ display: 'flex', gap: 'var(--sp-1)' }}>
            <button className="btn ghost" onClick={() => onAddAll(results)}>
              Select all
            </button>
            {selectedCount > 0 && (
              <button className="btn ghost" onClick={() => onRemoveAll(results)}>
                Deselect all
              </button>
            )}
          </div>
        )}
      </div>

      {error && (
        <M3FadeIn>
          <div className="empty st-fail" data-testid="search-error">
            <span className="empty-icon" aria-hidden="true">error_outline</span>
            <span className="empty-headline">Search failed</span>
            <span className="empty-body">{error}</span>
          </div>
        </M3FadeIn>
      )}

      {!loading && !error && results.length === 0 && (
        <M3FadeIn delay={0.1}>
          <div className="empty">
            <span className="empty-icon" aria-hidden="true">search</span>
            <span className="empty-headline">Search for something</span>
            <span className="empty-body">Type a query above and press Search to find music or videos</span>
          </div>
        </M3FadeIn>
      )}

      {/* Skeleton loading state */}
      {loading && results.length === 0 && (
        <div style={{ padding: 'var(--sp-3) var(--sp-4)', display: 'flex', flexDirection: 'column', gap: 'var(--sp-2)' }}>
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} style={{ display: 'flex', gap: 'var(--sp-3)', alignItems: 'center', padding: 'var(--sp-3) var(--sp-4)' }}>
              <M3Skeleton variant="rect" width={20} height={20} />
              <M3Skeleton variant="rect" width={60} height={40} />
              <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 6 }}>
                <M3Skeleton variant="text" width="70%" />
                <M3Skeleton variant="text" width="40%" />
              </div>
              <M3Skeleton variant="text" width={40} />
            </div>
          ))}
        </div>
      )}

      <div data-testid={`search-results-${source}`} role="listbox" aria-label="Search results">
        <M3Stagger className="result-grid" staggerDelay={0.02}>
          {results.map((item) => {
            const key = item.id || item.url;
            const isSelected = !!selected[key];
            return (
              <M3StaggerItem key={key}>
                <div
                  className={`result-card${isSelected ? ' selected' : ''}`}
                  onClick={() => onToggle(item)}
                  role="option"
                  aria-selected={isSelected}
                >
                  <div className="result-card-checkbox" aria-checked={isSelected}>
                    <div className="result-checkbox" aria-checked={isSelected}>
                      {isSelected ? <span className="search-icon" style={{ fontSize: 'var(--text-sm)' }}>check</span> : ''}
                    </div>
                  </div>
                  <div className="result-card-source" data-source={item.source}>{item.source}</div>
                  
                  <div className="result-card-thumb-container">
                    {item.thumbnail ? (
                      <img className="result-card-thumb" src={item.thumbnail} alt="" loading="lazy" />
                    ) : (
                      <div className="result-card-thumb" style={{ background: 'var(--md-surface-variant)' }}></div>
                    )}
                    <div className="result-card-gradient"></div>
                    <div className="result-card-duration">{formatDuration(item.duration)}</div>
                  </div>
                  
                  <div className="result-card-content">
                    <div className="result-card-title" title={item.title || item.url}>{item.title || item.url}</div>
                    <div className="result-card-uploader" title={item.uploader}>{item.uploader || '—'}</div>
                  </div>
                </div>
              </M3StaggerItem>
            );
          })}
        </M3Stagger>

        {results.length > 0 && !loading && !error && (
          <div className="pad-2-x" style={{ textAlign: 'center' }}>
            <button className="btn ghost" onClick={loadMore} disabled={loadingMore}>
              {loadingMore ? 'Loading…' : 'Load more'}
            </button>
          </div>
        )}
      </div>

      {/* Animated selection bar */}
      <M3SelectionBar show={selectedCount > 0}>
        <span data-testid="selection-count">{selectedCount} selected</span>
        <button
          className="btn"
          onClick={() => onQueueSelected(results.filter((r) => selected[r.id || r.url]))}
          data-testid="queue-selected-btn"
        >
          Queue download →
        </button>
      </M3SelectionBar>
    </>
  );
}