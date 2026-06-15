import React from 'react';

interface LinearProgressProps {
  value?: number; // 0 to 100 for determinate
  determinate?: boolean;
  color?: string;
  className?: string;
}

export const LinearProgress: React.FC<LinearProgressProps> = ({
  value = 0,
  determinate = false,
  color = 'var(--md-sys-color-primary)',
  className = '',
}) => {
  return (
    <div
      className={`linear-progress-container ${className}`}
      style={{
        position: 'relative',
        height: '4px',
        width: '100%',
        backgroundColor: 'var(--md-sys-color-surface-variant)',
        borderRadius: '2px',
        overflow: 'hidden',
      }}
    >
      {determinate ? (
        <div
          className="linear-progress-bar determinate"
          style={{
            position: 'absolute',
            top: 0,
            left: 0,
            height: '100%',
            backgroundColor: color,
            width: `${Math.max(0, Math.min(100, value))}%`,
            transition: 'width 0.2s linear',
          }}
        />
      ) : (
        <>
          <div
            className="linear-progress-bar indeterminate-1"
            style={{
              position: 'absolute',
              top: 0,
              left: 0,
              height: '100%',
              backgroundColor: color,
              width: 'auto',
              animation: 'linear-indeterminate-1 2.1s cubic-bezier(0.65, 0.815, 0.735, 0.395) infinite',
            }}
          />
          <div
            className="linear-progress-bar indeterminate-2"
            style={{
              position: 'absolute',
              top: 0,
              left: 0,
              height: '100%',
              backgroundColor: color,
              width: 'auto',
              animation: 'linear-indeterminate-2 2.1s cubic-bezier(0.165, 0.84, 0.44, 1) 1.15s infinite',
            }}
          />
        </>
      )}
      <style>{`
        @keyframes linear-indeterminate-1 {
          0% {
            left: -35%;
            right: 100%;
          }
          60% {
            left: 100%;
            right: -90%;
          }
          100% {
            left: 100%;
            right: -90%;
          }
        }
        @keyframes linear-indeterminate-2 {
          0% {
            left: -200%;
            right: 100%;
          }
          60% {
            left: 107%;
            right: -8%;
          }
          100% {
            left: 107%;
            right: -8%;
          }
        }
      `}</style>
    </div>
  );
};
