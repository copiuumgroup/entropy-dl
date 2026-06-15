import type { AsciiSpinnerProps } from '../types';

/**
 * M3 Circular Progress Indicator (indeterminate).
 * Replaces the legacy ASCII spinner with a proper Material You 3
 * circular loading animation rendered as inline SVG.
 */
export default function M3CircularProgress({ active = true, label = '', small = false }: AsciiSpinnerProps & { small?: boolean }) {
  if (!active) {
    return <span style={{ display: 'inline-block', width: small ? 'var(--md-sys-typescale-size-xl)' : 'var(--md-sys-size-icon-btn)', height: small ? 'var(--md-sys-typescale-size-xl)' : 'var(--md-sys-size-icon-btn)' }} aria-hidden="true" />;
  }

  // Standard M3 indeterminate: 24px viewport, r=12, C=75.398
  // Small variant: 20px viewport, r=10, C=62.832
  const r = small ? 10 : 12;
  const size = small ? 20 : 24;

  return (
    <span
      className={`m3-circular-progress${small ? ' sm' : ''}`}
      role="progressbar"
      aria-label={label || 'Loading'}
      data-testid="m3-circular-progress"
    >
      <svg viewBox={`0 0 ${size} ${size}`}>
        <circle className="track" cx={size / 2} cy={size / 2} r={r} />
        <circle className="indicator" cx={size / 2} cy={size / 2} r={r} />
      </svg>
      {label && <span className="sr-only">{label}</span>}
    </span>
  );
}