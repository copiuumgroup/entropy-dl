// ═══════════════════════════════════════════════════════════════════════
// M3 Animation Primitives — Framer Motion Spring implementation
// ═══════════════════════════════════════════════════════════════════════

import React, { type ReactNode, useState, useEffect, useRef } from 'react';
import { motion, AnimatePresence as FramerAnimatePresence, useReducedMotion as useFramerReducedMotion } from 'framer-motion';

// ─── Reduced motion hook ───
function useReducedMotion(): boolean {
  const reduced = useFramerReducedMotion();
  if (typeof window === 'undefined') return false;
  return reduced || window.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

const springConfig = { type: 'spring', stiffness: 400, damping: 30 } as const;

// ═══════════════════════════════════════════════════════════════════════
//  M3FadeIn — Fade in with optional directional slide
// ═══════════════════════════════════════════════════════════════════════

export interface M3FadeInProps {
  children: ReactNode;
  delay?: number;
  direction?: 'up' | 'down' | 'left' | 'right' | 'none';
  distance?: number;
  className?: string;
}

export const M3FadeIn: React.FC<M3FadeInProps> = ({
  children,
  delay = 0,
  direction = 'up',
  distance = 16,
  className,
}) => {
  const reduced = useReducedMotion();
  const dir = reduced ? 'none' : direction;

  let initialTransform = {};
  if (dir === 'up') initialTransform = { y: distance };
  if (dir === 'down') initialTransform = { y: -distance };
  if (dir === 'left') initialTransform = { x: distance };
  if (dir === 'right') initialTransform = { x: -distance };

  return (
    <motion.div
      className={className}
      initial={{ opacity: 0, ...initialTransform }}
      animate={{ opacity: 1, x: 0, y: 0 }}
      transition={{ ...springConfig, delay }}
    >
      {children}
    </motion.div>
  );
};

// ═══════════════════════════════════════════════════════════════════════
//  M3ViewTransition — Crossfade + subtle scale for view switches
// ═══════════════════════════════════════════════════════════════════════

export interface M3ViewTransitionProps {
  children: ReactNode;
  keyProp: string;
  className?: string;
}

export const M3ViewTransition: React.FC<M3ViewTransitionProps> = ({
  children,
  keyProp,
  className,
}) => {
  return (
    <FramerAnimatePresence mode="wait">
      <motion.div
        key={keyProp}
        className={className}
        initial={{ opacity: 0, scale: 0.98 }}
        animate={{ opacity: 1, scale: 1 }}
        exit={{ opacity: 0, scale: 0.98 }}
        transition={springConfig}
      >
        {children}
      </motion.div>
    </FramerAnimatePresence>
  );
};

// ═══════════════════════════════════════════════════════════════════════
//  M3CircularProgress — Indeterminate circular spinner (CSS-only)
// ═══════════════════════════════════════════════════════════════════════

export interface M3CircularProgressProps {
  active?: boolean;
  size?: number;
  strokeWidth?: number;
  className?: string;
}

export const M3CircularProgress: React.FC<M3CircularProgressProps> = ({
  active = true,
  size = 40,
  strokeWidth = 4,
  className,
}) => {
  const reduced = useReducedMotion();
  const center = size / 2;
  const radius = (size - strokeWidth) / 2;
  const circumference = 2 * Math.PI * radius;

  if (!active) {
    return (
      <span
        style={{ display: 'inline-block', width: size, height: size }}
        aria-hidden="true"
      />
    );
  }

  return (
    <span
      className={`m3-circular-progress${className ? ` ${className}` : ''}`}
      role="progressbar"
      aria-label="Loading"
      data-testid="m3-circular-progress"
      style={{ width: size, height: size, display: 'inline-flex', alignItems: 'center', justifyContent: 'center' }}
    >
      <svg viewBox={`0 0 ${size} ${size}`} style={{ width: '100%', height: '100%' }}>
        <circle
          cx={center}
          cy={center}
          r={radius}
          fill="none"
          stroke="var(--md-surface-container-highest, #2b2930)"
          strokeWidth={strokeWidth}
        />
        <circle
          cx={center}
          cy={center}
          r={radius}
          fill="none"
          stroke="var(--md-primary, #D0BCFF)"
          strokeWidth={strokeWidth}
          strokeLinecap="round"
          strokeDasharray={circumference}
          strokeDashoffset={circumference * 0.25}
          className="m3-spinner-arc"
          style={
            reduced
              ? undefined
              : {
                  animation: 'm3-spin 2s linear infinite',
                  transformOrigin: 'center',
                }
          }
        />
      </svg>
    </span>
  );
};

// ═══════════════════════════════════════════════════════════════════════
//  M3LinearProgress — Indeterminate linear progress bar (CSS-only)
// ═══════════════════════════════════════════════════════════════════════

export interface M3LinearProgressProps {
  active?: boolean;
  className?: string;
  color?: string;
}

export const M3LinearProgress: React.FC<M3LinearProgressProps> = ({
  active = true,
  className,
  color,
}) => {
  const reduced = useReducedMotion();
  const barColor = color || 'var(--md-primary, #D0BCFF)';

  if (!active) return null;

  return (
    <div
      className={`m3-linear-progress${className ? ` ${className}` : ''}`}
      role="progressbar"
      aria-label="Loading"
      style={{
        position: 'relative',
        height: 4,
        borderRadius: 'var(--md-shape-full, 9999px)',
        background: 'var(--md-surface-container-highest, #2b2930)',
        overflow: 'hidden',
      }}
    >
      <div
        className="m3-linear-bar"
        style={{
          position: 'absolute',
          top: 0,
          bottom: 0,
          width: '40%',
          borderRadius: 'var(--md-shape-full, 9999px)',
          background: barColor,
          ...(reduced ? undefined : { animation: 'm3-linear-slide 2s cubic-bezier(0.3, 0, 0.8, 0.15) infinite' }),
        }}
      />
      <div
        className="m3-linear-bar m3-linear-bar-secondary"
        style={{
          position: 'absolute',
          top: 0,
          bottom: 0,
          width: '30%',
          borderRadius: 'var(--md-shape-full, 9999px)',
          background: barColor,
          opacity: 0.6,
          ...(reduced ? undefined : { animation: 'm3-linear-slide 2.4s cubic-bezier(0.3, 0, 0.8, 0.15) 0.5s infinite' }),
        }}
      />
    </div>
  );
};

// ═══════════════════════════════════════════════════════════════════════
//  M3LinearProgressDeterminate — Determinate progress bar (CSS transition)
// ═══════════════════════════════════════════════════════════════════════

export interface M3LinearProgressDeterminateProps {
  value: number;
  className?: string;
  color?: string;
}

export const M3LinearProgressDeterminate: React.FC<M3LinearProgressDeterminateProps> = ({
  value,
  className,
  color,
}) => {
  const barColor = color || 'var(--md-primary, #D0BCFF)';
  const clamped = Math.max(0, Math.min(100, value));

  return (
    <div
      className={`m3-linear-progress${className ? ` ${className}` : ''}`}
      role="progressbar"
      aria-valuenow={Math.round(clamped)}
      aria-valuemin={0}
      aria-valuemax={100}
      style={{
        position: 'relative',
        height: 4,
        borderRadius: 'var(--md-shape-full, 9999px)',
        background: 'var(--md-surface-container-highest, #2b2930)',
        overflow: 'hidden',
      }}
    >
      <div
        style={{
          position: 'absolute',
          top: 0,
          bottom: 0,
          left: 0,
          borderRadius: 'var(--md-shape-full, 9999px)',
          background: barColor,
          width: `${clamped}%`,
          transition: 'width 0.3s cubic-bezier(0, 0, 0, 1)',
        }}
      />
    </div>
  );
};

// ═══════════════════════════════════════════════════════════════════════
//  M3Skeleton — Shimmer skeleton loading state (CSS-only)
// ═══════════════════════════════════════════════════════════════════════

export interface M3SkeletonProps {
  width?: string | number;
  height?: string | number;
  variant?: 'rect' | 'circle' | 'text';
  lines?: number;
  className?: string;
}

export const M3Skeleton: React.FC<M3SkeletonProps> = ({
  width,
  height,
  variant = 'rect',
  lines = 1,
  className,
}) => {
  const reduced = useReducedMotion();

  const baseStyle: React.CSSProperties = {
    background: 'var(--md-surface-container-highest, #2b2930)',
    overflow: 'hidden',
    position: 'relative',
  };

  if (variant === 'circle') {
    const size = width || 40;
    Object.assign(baseStyle, { width: size, height: size, borderRadius: '50%' });
  } else if (variant === 'text') {
    Object.assign(baseStyle, {
      width: width || '100%',
      height: height || 14,
      borderRadius: 'var(--md-shape-xs, 4px)',
    });
  } else {
    Object.assign(baseStyle, {
      width: width || '100%',
      height: height || 48,
      borderRadius: 'var(--md-shape-sm, 8px)',
    });
  }

  return (
    <div className={className} style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
      {Array.from({ length: lines }).map((_, i) => (
        <div
          key={i}
          style={{
            ...baseStyle,
            width: variant === 'text' && i === lines - 1 && lines > 1
              ? '60%'
              : baseStyle.width,
          }}
        >
          <div
            className="m3-shimmer"
            style={{
              position: 'absolute',
              inset: 0,
              background: 'linear-gradient(90deg, transparent, var(--md-surface-container-high, #1e1b2e), transparent)',
              backgroundSize: '200% 100%',
              ...(reduced ? undefined : { animation: 'm3-shimmer 1.8s linear infinite' }),
            }}
          />
        </div>
      ))}
    </div>
  );
};

// ═══════════════════════════════════════════════════════════════════════
//  M3Expand — Expand/collapse with AnimatePresence
// ═══════════════════════════════════════════════════════════════════════

export interface M3ExpandProps {
  expanded: boolean;
  children: ReactNode;
  className?: string;
}

export const M3Expand: React.FC<M3ExpandProps> = ({
  expanded,
  children,
  className,
}) => {
  return (
    <FramerAnimatePresence initial={false}>
      {expanded && (
        <motion.div
          className={className}
          initial={{ height: 0, opacity: 0 }}
          animate={{ height: 'auto', opacity: 1 }}
          exit={{ height: 0, opacity: 0 }}
          transition={{ opacity: { duration: 0.2 }, height: springConfig }}
          style={{ overflow: 'hidden' }}
        >
          {children}
        </motion.div>
      )}
    </FramerAnimatePresence>
  );
};

// ═══════════════════════════════════════════════════════════════════════
//  M3ScaleIn — Scale + fade for popovers, dialogs, tooltips
// ═══════════════════════════════════════════════════════════════════════

export interface M3ScaleInProps {
  children: ReactNode;
  show?: boolean;
  className?: string;
  origin?: string;
}

export const M3ScaleIn: React.FC<M3ScaleInProps> = ({
  children,
  show = true,
  className,
  origin = 'top right',
}) => {
  return (
    <FramerAnimatePresence>
      {show && (
        <motion.div
          className={className}
          initial={{ opacity: 0, scale: 0.9 }}
          animate={{ opacity: 1, scale: 1 }}
          exit={{ opacity: 0, scale: 0.9 }}
          transition={springConfig}
          style={{ transformOrigin: origin }}
        >
          {children}
        </motion.div>
      )}
    </FramerAnimatePresence>
  );
};

// ═══════════════════════════════════════════════════════════════════════
//  M3Stagger — Stagger children's entrance animations via Framer Motion
// ═══════════════════════════════════════════════════════════════════════

export interface M3StaggerProps {
  children: ReactNode;
  staggerDelay?: number;
  className?: string;
}

export const M3Stagger: React.FC<M3StaggerProps> = ({
  children,
  staggerDelay = 0.05,
  className,
}) => {
  return (
    <motion.div
      className={className}
      initial="hidden"
      animate="visible"
      variants={{
        visible: {
          transition: { staggerChildren: staggerDelay },
        },
      }}
    >
      {children}
    </motion.div>
  );
};

export const M3StaggerItem: React.FC<{ children: ReactNode; className?: string }> = ({
  children,
  className,
}) => {
  return (
    <motion.div
      className={className}
      variants={{
        hidden: { opacity: 0, y: 16 },
        visible: { opacity: 1, y: 0, transition: springConfig },
      }}
    >
      {children}
    </motion.div>
  );
};

// ═══════════════════════════════════════════════════════════════════════
//  M3SwitchAnimated — Toggle switch with CSS transitions
// ═══════════════════════════════════════════════════════════════════════

export interface M3SwitchAnimatedProps {
  on: boolean;
}

export const M3SwitchAnimated: React.FC<M3SwitchAnimatedProps> = ({ on }) => {
  const reduced = useReducedMotion();
  const trackWidth = 52;
  const thumbSize = on ? 24 : 16;
  const thumbLeft = on ? trackWidth - 8 - thumbSize : 6;
  const dur = reduced ? '0ms' : '0.3s';

  return (
    <span className={`m3-switch${on ? ' on' : ''}`} style={{ position: 'relative', display: 'inline-flex', width: trackWidth, height: 32, alignItems: 'center' }}>
      <span
        className="switch-track"
        style={{
          position: 'absolute',
          inset: 0,
          borderRadius: 'var(--md-shape-full, 9999px)',
          border: '2px solid',
          background: on ? 'var(--md-primary, #D0BCFF)' : 'var(--md-surface-container-highest, #2b2930)',
          borderColor: on ? 'var(--md-primary, #D0BCFF)' : 'var(--md-outline, #79747E)',
          transition: `background ${dur}, border-color ${dur}`,
        }}
      />
      <motion.span
        className="switch-thumb"
        initial={false}
        animate={{
          left: thumbLeft,
          width: thumbSize,
          height: thumbSize,
          background: on ? 'var(--md-on-primary, #381E72)' : 'var(--md-outline, #79747E)'
        }}
        transition={springConfig}
        style={{
          position: 'absolute',
          top: '50%',
          y: '-50%',
          borderRadius: '50%',
          zIndex: 2,
          pointerEvents: 'none',
        }}
      />
    </span>
  );
};

// ═══════════════════════════════════════════════════════════════════════
//  M3NavRailIndicator — Animated pill for nav rail (CSS transition)
//  Renders as a full-height pill behind the entire active item
//  (icon + label together), matching the M3 navigation rail spec.
// ═══════════════════════════════════════════════════════════════════════

export interface M3NavRailIndicatorProps {
  activeIndex: number;
  count: number;
  itemHeight?: number;
  gap?: number;
  paddingTop?: number;
}

export const M3NavRailIndicator: React.FC<M3NavRailIndicatorProps> = ({
  activeIndex,
  itemHeight = 56,
  gap = 4,
  paddingTop = 0,
}) => {
  // Pill spans the full item height so icon + label read as one unit.
  const pillHeight = itemHeight;
  const y = paddingTop + activeIndex * (itemHeight + gap);

  return (
    <motion.span
      className="nav-rail-indicator"
      aria-hidden="true"
      initial={false}
      animate={{ y }}
      transition={springConfig}
      style={{
        position: 'absolute',
        left: '50%',
        x: '-50%',
        top: 0,
        width: '3.5rem',
        height: pillHeight,
        borderRadius: 'var(--md-shape-xl, 16px)',
        background: 'var(--md-secondary-container, #4A4458)',
        zIndex: 0,
        pointerEvents: 'none',
      }}
    />
  );
};

// ═══════════════════════════════════════════════════════════════════════
//  M3ToastAnimated — Toast/snackbar with Framer Motion
// ═══════════════════════════════════════════════════════════════════════

export interface M3ToastAnimatedProps {
  children: ReactNode;
  show: boolean;
  isError?: boolean;
}

export const M3ToastAnimated: React.FC<M3ToastAnimatedProps> = ({
  children,
  show,
  isError,
}) => {
  return (
    <FramerAnimatePresence>
      {show && (
        <motion.div
          className={`toast${isError ? ' err' : ''}`}
          initial={{ opacity: 0, y: 24, scale: 0.95 }}
          animate={{ opacity: 1, y: 0, scale: 1 }}
          exit={{ opacity: 0, y: 16, scale: 0.95 }}
          transition={springConfig}
          style={{
            position: 'fixed',
            bottom: 'var(--sp-4, 16px)',
            right: 'var(--sp-4, 16px)',
            zIndex: 100,
          }}
        >
          {children}
        </motion.div>
      )}
    </FramerAnimatePresence>
  );
};

// ═══════════════════════════════════════════════════════════════════════
//  M3SelectionBar — Animated bottom bar for selection actions
// ═══════════════════════════════════════════════════════════════════════

export const M3SelectionBar: React.FC<{ children: ReactNode; show: boolean; className?: string }> = ({
  children,
  show,
  className,
}) => {
  return (
    <FramerAnimatePresence>
      {show && (
        <motion.div
          className={`selection-bar${className ? ` ${className}` : ''}`}
          initial={{ opacity: 0, y: 40 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: 40 }}
          transition={springConfig}
          style={{
            position: 'sticky',
            bottom: 0,
            zIndex: 20,
          }}
        >
          {children}
        </motion.div>
      )}
    </FramerAnimatePresence>
  );
};

import { Ripple } from '../Ripple';

// ═══════════════════════════════════════════════════════════════════════
//  M3ChipAnimated — Filter chip with Framer Motion and Ripple
// ═══════════════════════════════════════════════════════════════════════

export interface M3ChipAnimatedProps {
  selected: boolean;
  children: ReactNode;
  onClick?: () => void;
  disabled?: boolean;
  className?: string;
}

export const M3ChipAnimated: React.FC<M3ChipAnimatedProps> = ({
  selected,
  children,
  onClick,
  disabled,
  className,
}) => {
  const reduced = useReducedMotion();
  const dur = reduced ? '0ms' : '0.2s';

  return (
    <motion.button
      whileTap={{ scale: 0.95 }}
      transition={{ type: 'spring', stiffness: 500, damping: 25 }}
      className={`chip${selected ? ' active' : ''}${className ? ` ${className}` : ''}`}
      onClick={onClick}
      disabled={disabled}
      type="button"
      style={{
        position: 'relative',
        overflow: 'hidden',
        background: selected ? 'var(--md-secondary-container, #4A4458)' : 'transparent',
        color: selected ? 'var(--md-on-secondary-container, #E8DEF8)' : 'var(--md-on-surface-variant, #CAC4D0)',
        borderColor: selected ? 'transparent' : 'var(--md-outline-variant, #49454F)',
        transition: `background ${dur}, color ${dur}, border-color ${dur}`,
      }}
    >
      <span style={{ position: 'relative', zIndex: 1, pointerEvents: 'none' }}>{children}</span>
      <Ripple />
    </motion.button>
  );
};

// ═══════════════════════════════════════════════════════════════════════
//  AnimatePresence — Passthrough wrapper (Alias for Framer)
// ═══════════════════════════════════════════════════════════════════════

export const AnimatePresence = FramerAnimatePresence;