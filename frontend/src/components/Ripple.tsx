import React, { useState, useLayoutEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';

interface RippleType {
  x: number;
  y: number;
  size: number;
  id: number;
}

export const Ripple: React.FC<{ color?: string }> = ({ color = 'var(--md-on-surface, #1C1B1F)' }) => {
  const [ripples, setRipples] = useState<RippleType[]>([]);

  useLayoutEffect(() => {
    let bounce: ReturnType<typeof setTimeout>;
    if (ripples.length > 0) {
      bounce = setTimeout(() => {
        setRipples([]);
      }, 1000); // clear after animation completes
    }
    return () => clearTimeout(bounce);
  }, [ripples.length]);

  const addRipple = (e: React.PointerEvent<HTMLDivElement>) => {
    const container = e.currentTarget.getBoundingClientRect();
    const size = Math.max(container.width, container.height);
    const x = e.clientX - container.left - size / 2;
    const y = e.clientY - container.top - size / 2;
    const newRipple = { x, y, size, id: Date.now() };

    setRipples((prev) => [...prev, newRipple]);
  };

  return (
    <div
      onPointerDown={addRipple}
      style={{
        position: 'absolute',
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        overflow: 'hidden',
        pointerEvents: 'auto',
        borderRadius: 'inherit',
        zIndex: 0,
      }}
    >
      <AnimatePresence>
        {ripples.map((r) => (
          <motion.span
            key={r.id}
            initial={{ top: r.y, left: r.x, width: r.size, height: r.size, opacity: 0.15, scale: 0 }}
            animate={{ opacity: 0, scale: 2.5 }}
            transition={{ duration: 0.5, ease: 'easeOut' }}
            style={{
              position: 'absolute',
              backgroundColor: color,
              borderRadius: '100%',
              pointerEvents: 'none',
            }}
          />
        ))}
      </AnimatePresence>
    </div>
  );
};
