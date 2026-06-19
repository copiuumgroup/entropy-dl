import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';
import LoginScreen from './components/LoginScreen';
import './styles.css';
import { generateScheme, applySchemeToCSS, DEFAULT_SEEDS, EXPRESSIVE_DEFAULT } from './lib/hct-palette';

// Apply a sensible default theme synchronously before first React render.
// This prevents the near-black fallback flash while ThemeSync awaits /api/theme.
// Any error here is caught so the app always mounts.
try {
  const isDark = typeof window !== 'undefined'
    ? window.matchMedia('(prefers-color-scheme: dark)').matches
    : true;

  let platform = 'linux';
  const ua = (typeof navigator !== 'undefined' ? navigator.userAgent : '').toLowerCase();
  if (ua.includes('macintosh') || ua.includes('mac os')) platform = 'macos';
  else if (ua.includes('windows')) platform = 'windows';

  const seed = DEFAULT_SEEDS[platform] ?? EXPRESSIVE_DEFAULT;
  const scheme = generateScheme(seed, isDark);
  applySchemeToCSS(scheme, isDark);
} catch (_e) {
  // Non-fatal: ThemeSync will apply colors after mount
  console.warn('[entropy] Initial theme failed, ThemeSync will recover:', _e);
}

const root = ReactDOM.createRoot(document.getElementById('root')!);
root.render(
  <React.StrictMode>
    <LoginScreen>
      <App />
    </LoginScreen>
  </React.StrictMode>
);
