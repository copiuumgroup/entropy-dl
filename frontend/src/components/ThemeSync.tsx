import { useEffect, useRef, useCallback } from 'react';
import {
  generateScheme,
  applySchemeToCSS,
  DEFAULT_SEEDS,
  EXPRESSIVE_DEFAULT,
  type M3Scheme,
} from '../lib/hct-palette';

interface ThemeResponse {
  seed: string;
  platform: string;
  source: string;
}

/**
 * ThemeSync fetches the OS accent color from /api/theme,
 * generates an M3 tonal palette via HCT, and applies it as CSS variables.
 * Also listens for prefers-color-scheme changes to swap light/dark, and
 * polls the OS accent periodically so changes apply without an app restart.
 */
import type { ThemePref } from '../types';

interface ThemeSyncProps {
  themePref: ThemePref;
}

// How often to re-check the OS accent color. Windows/macOS/Linux don't
// broadcast accent changes to the browser, so we poll. 5s is responsive
// enough to feel live without any meaningful cost.
const ACCENT_POLL_INTERVAL = 5000;

export default function ThemeSync({ themePref }: ThemeSyncProps) {
  const schemeRef = useRef<M3Scheme | null>(null);
  const seedRef = useRef<string>('');

  const applyForMode = useCallback((seed: string, isDark: boolean) => {
    let platform = 'windows';
    if (typeof navigator !== 'undefined') {
      const ua = navigator.userAgent || '';
      if (/mac/i.test(ua) || /darwin/i.test(navigator.platform)) {
        platform = 'macos';
      } else if (/linux/i.test(navigator.platform) || /linux/i.test(ua)) {
        platform = 'linux';
      }
    }
    const effectiveSeed = seed || DEFAULT_SEEDS[platform] || EXPRESSIVE_DEFAULT;
    const scheme = generateScheme(effectiveSeed, isDark);
    schemeRef.current = scheme;
    applySchemeToCSS(scheme, isDark);
  }, []);

  useEffect(() => {
    let cancelled = false;

    const determineIsDark = () => {
      if (themePref === 'dark') return true;
      if (themePref === 'light') return false;
      return window.matchMedia('(prefers-color-scheme: dark)').matches;
    };

    // Fetch (or re-fetch) the OS accent color. Re-applies the scheme only
    // when the seed actually changed, so polling is a no-op most of the time.
    const refreshSeed = () => {
      fetch('/api/theme')
        .then((r) => r.json())
        .then((data: ThemeResponse) => {
          if (cancelled) return;
          const seed = data.seed || DEFAULT_SEEDS[data.platform] || EXPRESSIVE_DEFAULT;
          if (seed !== seedRef.current) {
            seedRef.current = seed;
            applyForMode(seed, determineIsDark());
          }
        })
        .catch(() => { /* non-fatal; keep previous theme */ });
    };

    // Initial fetch — apply immediately so the first paint is correct.
    refreshSeed();
    // Poll for OS accent changes (user changes Windows color, etc.)
    const pollId = window.setInterval(refreshSeed, ACCENT_POLL_INTERVAL);

    // Listen for system dark/light mode changes (only matters if system)
    const mql = window.matchMedia('(prefers-color-scheme: dark)');
    const handler = () => {
      if (!cancelled && themePref === 'system') {
        applyForMode(seedRef.current, mql.matches);
      }
    };
    mql.addEventListener('change', handler);

    return () => {
      cancelled = true;
      window.clearInterval(pollId);
      mql.removeEventListener('change', handler);
    };
  }, [applyForMode, themePref]);

  return null; // Renders nothing — purely side-effect component
}