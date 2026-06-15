import React, { useEffect, useRef } from 'react';

// Orb configuration — driven by CSS vars so they auto-update on theme change
const ORBS = [
  {
    // Top-left — downloads color (blue-ish)
    color: 'var(--md-status-downloading)',
    width: '55vw', height: '55vh',
    top: '-12%', left: '-8%',
    opacity: 0.12,
    blur: '130px',
    animDuration: '22s',
    animName: 'mesh-orb-a',
  },
  {
    // Bottom-right — processing color (amber)
    color: 'var(--md-status-processing)',
    width: '60vw', height: '60vh',
    bottom: '-15%', right: '-10%',
    opacity: 0.09,
    blur: '150px',
    animDuration: '28s',
    animName: 'mesh-orb-b',
  },
  {
    // Center-left — done color (green)
    color: 'var(--md-status-done)',
    width: '40vw', height: '45vh',
    top: '35%', left: '20%',
    opacity: 0.06,
    blur: '110px',
    animDuration: '35s',
    animName: 'mesh-orb-c',
  },
  {
    // Top-right — subtle tint of primary
    color: 'var(--md-primary)',
    width: '35vw', height: '35vh',
    top: '-5%', right: '5%',
    opacity: 0.07,
    blur: '100px',
    animDuration: '30s',
    animName: 'mesh-orb-d',
  },
];

export default function MeshBackground() {
  return (
    <>
      <style>{`
        @keyframes mesh-orb-a {
          0%   { transform: translate(0,    0)    scale(1);    border-radius: 60% 40% 55% 45% / 50% 60% 40% 50%; }
          25%  { transform: translate(5%,   8%)   scale(1.05); border-radius: 50% 50% 60% 40% / 40% 55% 45% 60%; }
          50%  { transform: translate(10%,  5%)   scale(0.95); border-radius: 45% 55% 40% 60% / 60% 40% 50% 50%; }
          75%  { transform: translate(3%,   12%)  scale(1.08); border-radius: 55% 45% 50% 50% / 45% 60% 40% 55%; }
          100% { transform: translate(0,    0)    scale(1);    border-radius: 60% 40% 55% 45% / 50% 60% 40% 50%; }
        }
        @keyframes mesh-orb-b {
          0%   { transform: translate(0,    0)    scale(1);    border-radius: 40% 60% 45% 55% / 60% 40% 50% 50%; }
          33%  { transform: translate(-8%,  -6%)  scale(1.1);  border-radius: 55% 45% 60% 40% / 45% 55% 40% 60%; }
          66%  { transform: translate(-4%,  -12%) scale(0.92); border-radius: 50% 50% 45% 55% / 55% 45% 60% 40%; }
          100% { transform: translate(0,    0)    scale(1);    border-radius: 40% 60% 45% 55% / 60% 40% 50% 50%; }
        }
        @keyframes mesh-orb-c {
          0%   { transform: translate(0,    0)    scale(1)    rotate(0deg);   }
          40%  { transform: translate(-6%,  -8%)  scale(1.12) rotate(8deg);   }
          80%  { transform: translate(8%,   4%)   scale(0.9)  rotate(-5deg);  }
          100% { transform: translate(0,    0)    scale(1)    rotate(0deg);   }
        }
        @keyframes mesh-orb-d {
          0%   { transform: translate(0,    0)    scale(1);    }
          50%  { transform: translate(-10%, 10%)  scale(1.15); }
          100% { transform: translate(0,    0)    scale(1);    }
        }
      `}</style>

      {/* Base layer — picks up background color from CSS vars */}
      <div style={{
        position: 'fixed',
        inset: 0,
        zIndex: -2,
        background: 'var(--md-background)',
        pointerEvents: 'none',
      }} />

      {/* Orbs layer */}
      <div style={{
        position: 'fixed',
        inset: 0,
        zIndex: -1,
        overflow: 'hidden',
        pointerEvents: 'none',
      }}>
        {ORBS.map((orb, i) => (
          <div
            key={i}
            style={{
              position: 'absolute',
              width: orb.width,
              height: orb.height,
              top: (orb as any).top,
              bottom: (orb as any).bottom,
              left: (orb as any).left,
              right: (orb as any).right,
              background: orb.color,
              opacity: orb.opacity,
              filter: `blur(${orb.blur})`,
              animation: `${orb.animName} ${orb.animDuration} ease-in-out infinite`,
              willChange: 'transform',
            }}
          />
        ))}
      </div>
    </>
  );
}
