import React from 'react';

interface CircularProgressProps {
  size?: number;
  thickness?: number;
  color?: string;
  className?: string;
}

export const CircularProgress: React.FC<CircularProgressProps> = ({
  size = 40,
  thickness = 3.6,
  color = 'var(--md-sys-color-primary)',
  className = '',
}) => {
  const center = size / 2;
  const radius = center - thickness / 2;
  const circumference = 2 * Math.PI * radius;

  return (
    <div
      className={`circular-progress-container ${className}`}
      style={{
        width: size,
        height: size,
        display: 'inline-flex',
        alignItems: 'center',
        justifyContent: 'center',
        position: 'relative',
        animation: 'circular-rotate 1.4s linear infinite',
      }}
    >
      <svg viewBox={`0 0 ${size} ${size}`}>
        <circle
          cx={center}
          cy={center}
          r={radius}
          fill="none"
          stroke={color}
          strokeWidth={thickness}
          strokeLinecap="round"
          style={{
            strokeDasharray: circumference,
            strokeDashoffset: 0,
            animation: 'circular-dash 1.4s ease-in-out infinite',
          }}
        />
      </svg>
      <style>{`
        @keyframes circular-rotate {
          100% {
            transform: rotate(360deg);
          }
        }
        @keyframes circular-dash {
          0% {
            stroke-dasharray: 1px, 200px;
            stroke-dashoffset: 0;
          }
          50% {
            stroke-dasharray: 100px, 200px;
            stroke-dashoffset: -15px;
          }
          100% {
            stroke-dasharray: 100px, 200px;
            stroke-dashoffset: -125px;
          }
        }
      `}</style>
    </div>
  );
};
