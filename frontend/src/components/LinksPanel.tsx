import { useState, useMemo } from 'react';
import { cleanUrl } from '../lib/api';
import type { LinksPanelProps } from '../types';

// Platforms that yt-dlp cannot download from by URL keyword search
const UNSUPPORTED_PATTERNS: { pattern: RegExp; name: string }[] = [
  { pattern: /open\.spotify\.com/i, name: 'Spotify' },
  { pattern: /music\.apple\.com/i, name: 'Apple Music' },
  { pattern: /tidal\.com/i, name: 'Tidal' },
  { pattern: /deezer\.com/i, name: 'Deezer' },
  { pattern: /amazon\.com\/music|music\.amazon/i, name: 'Amazon Music' },
];

function detectUnsupported(text: string): string[] {
  const found = new Set<string>();
  for (const { pattern, name } of UNSUPPORTED_PATTERNS) {
    if (pattern.test(text)) found.add(name);
  }
  return Array.from(found);
}

export default function LinksPanel({ onQueue }: LinksPanelProps) {
  const [text, setText] = useState('');
  const [cleanedUrls, setCleanedUrls] = useState<string[]>([]);
  const [cleaning, setCleaning] = useState(false);

  const unsupportedPlatforms = useMemo(() => detectUnsupported(text), [text]);

  const handleClean = async (): Promise<void> => {
    if (!text.trim()) return;
    setCleaning(true);
    try {
      const urls = await cleanUrl(text);
      setCleanedUrls(urls);
    } catch {
      // Fallback: extract URLs manually
      const matches = text.match(/https?:\/\/[^\s]+/gi) || [];
      setCleanedUrls(matches);
    }
    setCleaning(false);
  };

  const handleQueue = (): void => {
    if (cleanedUrls.length > 0) {
      onQueue(cleanedUrls);
    } else {
      const matches = text.match(/https?:\/\/[^\s]+/gi) || [];
      if (matches.length > 0) {
        onQueue(matches);
      }
    }
  };

  return (
    <>
      <div className="search-row flex-col gap-3">
        <textarea
          className="search-input textarea"
          placeholder={
            'Paste links (one per line)\nyoutube.com/playlist?list=...\nsoundcloud.com/artist/sets/album\nhttps://youtu.be/abcd?si=tracking'
          }
          value={text}
          onChange={(e) => setText(e.target.value)}
          aria-label="Paste video links"
          data-testid="links-textarea"
        />

        {unsupportedPlatforms.length > 0 && (
          <div className="links-unsupported-banner" role="alert">
            <span className="md-icon links-banner-icon" aria-hidden="true">block</span>
            <span>
              <strong>{unsupportedPlatforms.join(', ')}</strong> {unsupportedPlatforms.length === 1 ? 'is' : 'are'} not supported by yt-dlp and cannot be downloaded.
              {unsupportedPlatforms.some(p => p === 'Spotify') && ' Try searching for the track name on YouTube or SoundCloud instead.'}
            </span>
          </div>
        )}

        <div className="flex gap-2">
          <button
            className="btn tonal"
            onClick={handleClean}
            disabled={!text.trim() || cleaning}
            data-testid="links-clean-btn"
          >
            Clean URLs
          </button>
          <button
            className="btn primary"
            onClick={handleQueue}
            disabled={text.trim().length === 0}
            data-testid="links-queue-btn"
          >
            Queue download →
          </button>
        </div>
      </div>

      {cleanedUrls.length > 0 && (
        <>
          <div className="results-meta">
            <span>{cleanedUrls.length} cleaned URLs</span>
            <span>Tracking removed</span>
          </div>
          {cleanedUrls.map((url, i) => (
            <div key={i} className="result-row">
              <div className="result-checkbox" aria-checked="true">
                <span className="search-icon search-icon-sm">check</span>
              </div>
              <div className="result-title">{url}</div>
              <div className="result-duration">Clean</div>
            </div>
          ))}
        </>
      )}

      {cleanedUrls.length === 0 && (
        <div className="empty">
          <span className="empty-icon" aria-hidden="true">link</span>
          <span className="empty-headline">Paste links above</span>
          <span className="empty-body">Strips tracking parameters, expands playlists and album sets</span>
        </div>
      )}
    </>
  );
}