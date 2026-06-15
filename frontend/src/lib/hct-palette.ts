import {
  SchemeMonochrome,
  Hct,
  argbFromHex,
  hexFromArgb,
} from '@material/material-color-utilities';

// ─── Default Seed Colors (per OS) ───

export const DEFAULT_SEEDS: Record<string, string> = {
  windows: '#757575',
  darwin: '#757575',
  macos: '#757575',
  linux: '#757575',
};

export const EXPRESSIVE_DEFAULT = '#757575';

export interface M3Scheme {
  primary: string; onPrimary: string; primaryContainer: string; onPrimaryContainer: string;
  secondary: string; onSecondary: string; secondaryContainer: string; onSecondaryContainer: string;
  tertiary: string; onTertiary: string; tertiaryContainer: string; onTertiaryContainer: string;
  error: string; onError: string; errorContainer: string; onErrorContainer: string;
  background: string; onBackground: string;
  surface: string; onSurface: string;
  surfaceDim: string; surfaceBright: string;
  surfaceContainerLowest: string; surfaceContainerLow: string;
  surfaceContainer: string; surfaceContainerHigh: string; surfaceContainerHighest: string;
  surfaceVariant: string; onSurfaceVariant: string;
  outline: string; outlineVariant: string;
  inverseSurface: string; inverseOnSurface: string;
}

export function generateScheme(seedHex: string, isDark: boolean): M3Scheme {
  const seedArgb = argbFromHex(seedHex);
  const hct = Hct.fromInt(seedArgb);
  const scheme = new SchemeMonochrome(hct, isDark, 0.0);

  return {
    primary: hexFromArgb(scheme.primary),
    onPrimary: hexFromArgb(scheme.onPrimary),
    primaryContainer: hexFromArgb(scheme.primaryContainer),
    onPrimaryContainer: hexFromArgb(scheme.onPrimaryContainer),
    secondary: hexFromArgb(scheme.secondary),
    onSecondary: hexFromArgb(scheme.onSecondary),
    secondaryContainer: hexFromArgb(scheme.secondaryContainer),
    onSecondaryContainer: hexFromArgb(scheme.onSecondaryContainer),
    tertiary: hexFromArgb(scheme.tertiary),
    onTertiary: hexFromArgb(scheme.onTertiary),
    tertiaryContainer: hexFromArgb(scheme.tertiaryContainer),
    onTertiaryContainer: hexFromArgb(scheme.onTertiaryContainer),
    error: hexFromArgb(scheme.error),
    onError: hexFromArgb(scheme.onError),
    errorContainer: hexFromArgb(scheme.errorContainer),
    onErrorContainer: hexFromArgb(scheme.onErrorContainer),
    background: hexFromArgb(scheme.background),
    onBackground: hexFromArgb(scheme.onBackground),
    surface: hexFromArgb(scheme.surface),
    onSurface: hexFromArgb(scheme.onSurface),
    surfaceDim: hexFromArgb(scheme.surfaceDim),
    surfaceBright: hexFromArgb(scheme.surfaceBright),
    surfaceContainerLowest: hexFromArgb(scheme.surfaceContainerLowest),
    surfaceContainerLow: hexFromArgb(scheme.surfaceContainerLow),
    surfaceContainer: hexFromArgb(scheme.surfaceContainer),
    surfaceContainerHigh: hexFromArgb(scheme.surfaceContainerHigh),
    surfaceContainerHighest: hexFromArgb(scheme.surfaceContainerHighest),
    surfaceVariant: hexFromArgb(scheme.surfaceVariant),
    onSurfaceVariant: hexFromArgb(scheme.onSurfaceVariant),
    outline: hexFromArgb(scheme.outline),
    outlineVariant: hexFromArgb(scheme.outlineVariant),
    inverseSurface: hexFromArgb(scheme.inverseSurface),
    inverseOnSurface: hexFromArgb(scheme.inverseOnSurface),
  };
}

export function applySchemeToCSS(scheme: M3Scheme, isDark: boolean): void {
  const root = document.documentElement;
  root.style.setProperty('color-scheme', isDark ? 'dark' : 'light');
  
  const mapping: Record<string, keyof M3Scheme> = {
    '--md-sys-color-primary': 'primary',
    '--md-sys-color-on-primary': 'onPrimary',
    '--md-sys-color-primary-container': 'primaryContainer',
    '--md-sys-color-on-primary-container': 'onPrimaryContainer',
    '--md-sys-color-secondary': 'secondary',
    '--md-sys-color-on-secondary': 'onSecondary',
    '--md-sys-color-secondary-container': 'secondaryContainer',
    '--md-sys-color-on-secondary-container': 'onSecondaryContainer',
    '--md-sys-color-tertiary': 'tertiary',
    '--md-sys-color-on-tertiary': 'onTertiary',
    '--md-sys-color-tertiary-container': 'tertiaryContainer',
    '--md-sys-color-on-tertiary-container': 'onTertiaryContainer',
    '--md-sys-color-error': 'error',
    '--md-sys-color-on-error': 'onError',
    '--md-sys-color-error-container': 'errorContainer',
    '--md-sys-color-on-error-container': 'onErrorContainer',
    '--md-sys-color-background': 'background',
    '--md-sys-color-on-background': 'onBackground',
    '--md-sys-color-surface': 'surface',
    '--md-sys-color-on-surface': 'onSurface',
    '--md-sys-color-surface-dim': 'surfaceDim',
    '--md-sys-color-surface-bright': 'surfaceBright',
    '--md-sys-color-surface-container-lowest': 'surfaceContainerLowest',
    '--md-sys-color-surface-container-low': 'surfaceContainerLow',
    '--md-sys-color-surface-container': 'surfaceContainer',
    '--md-sys-color-surface-container-high': 'surfaceContainerHigh',
    '--md-sys-color-surface-container-highest': 'surfaceContainerHighest',
    '--md-sys-color-surface-variant': 'surfaceVariant',
    '--md-sys-color-on-surface-variant': 'onSurfaceVariant',
    '--md-sys-color-outline': 'outline',
    '--md-sys-color-outline-variant': 'outlineVariant',
    '--md-sys-color-inverse-surface': 'inverseSurface',
    '--md-sys-color-inverse-on-surface': 'inverseOnSurface',
  };

  for (const [cssVar, schemeKey] of Object.entries(mapping)) {
    root.style.setProperty(cssVar, scheme[schemeKey]);
  }

  root.style.setProperty('--md-sys-color-success', isDark ? '#8BD48C' : '#1B6E20');
  root.style.setProperty('--md-sys-color-on-success', isDark ? '#003910' : '#FFFFFF');
  root.style.setProperty('--md-sys-color-success-container', isDark ? '#005321' : '#A5F1A7');
  root.style.setProperty('--md-sys-color-on-success-container', isDark ? '#A8F1A7' : '#002106');
  
  root.style.setProperty('--md-sys-color-warning', isDark ? '#FFB74D' : '#F57C00');
  root.style.setProperty('--md-sys-color-on-warning', isDark ? '#4E342E' : '#FFFFFF');
  root.style.setProperty('--md-sys-color-warning-container', isDark ? '#5C4033' : '#FFE0B2');
  root.style.setProperty('--md-sys-color-on-warning-container', isDark ? '#FFE0B2' : '#3E2723');

  // Explicit semantic colors for statuses (vibrant across themes)
  root.style.setProperty('--md-status-downloading', isDark ? '#4FC3F7' : '#0288D1'); // Blue
  root.style.setProperty('--md-status-processing', isDark ? '#FFCA28' : '#F57F17'); // Amber
  root.style.setProperty('--md-status-done', isDark ? '#81C784' : '#388E3C'); // Green
  root.style.setProperty('--md-status-failed', isDark ? '#E57373' : '#D32F2F'); // Red

  root.style.setProperty('--md-primary', scheme.primary);
  root.style.setProperty('--md-on-primary', scheme.onPrimary);
}
