// ═══════════════════════════════════════════════════════════════════════
//  BrandMarkIcon — the Entropy app mark, inline SVG.
//
//  A single cohesive glyph: video glyph flows into a rainbow energy stream
//  into a music glyph. No gaps, no separation — one unified mark where
//  the rainbow flows through the media shapes. The media outlines are
//  monochrome (currentColor); the internal fill is the rainbow gradient.
// ═══════════════════════════════════════════════════════════════════════

export default function BrandMarkIcon({ title = 'Entropy' }: { title?: string }) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 512 512"
      role="img"
      aria-label={title}
      style={{ width: '100%', height: '100%', display: 'block' }}
    >
      <title>{title}</title>
      <defs>
        <linearGradient id="brandRainbow" x1="0" y1="0" x2="1" y2="0">
          <stop offset="0%" stopColor="#B388FF" />
          <stop offset="25%" stopColor="#40C4FF" />
          <stop offset="45%" stopColor="#69F0AE" />
          <stop offset="65%" stopColor="#FFD740" />
          <stop offset="100%" stopColor="#FF5252" />
        </linearGradient>
      </defs>

      {/* ── Single unified shape: video→flow→music ── */}

      {/* Video / film body — rounded rect with cutout play triangle */}
      <path d="
        M 56 128
        Q 56 104, 80 104
        L 224 104
        Q 248 104, 248 128
        L 248 384
        Q 248 408, 224 408
        L 80 408
        Q 56 408, 56 384
        Z
      " fill="currentColor" />
      {/* Play notch */}
      <polygon points="128,200 128,312 208,256" fill="hsl(0,0%,6%)" />
      {/* Sprocket perforations */}
      <rect x="68"  y="112" width="16" height="16" rx="4" fill="hsl(0,0%,6%)" />
      <rect x="96"  y="112" width="16" height="16" rx="4" fill="hsl(0,0%,6%)" />
      <rect x="124" y="112" width="16" height="16" rx="4" fill="hsl(0,0%,6%)" />
      <rect x="152" y="112" width="16" height="16" rx="4" fill="hsl(0,0%,6%)" />
      <rect x="180" y="112" width="16" height="16" rx="4" fill="hsl(0,0%,6%)" />
      <rect x="208" y="112" width="16" height="16" rx="4" fill="hsl(0,0%,6%)" />
      <rect x="68"  y="384" width="16" height="16" rx="4" fill="hsl(0,0%,6%)" />
      <rect x="96"  y="384" width="16" height="16" rx="4" fill="hsl(0,0%,6%)" />
      <rect x="124" y="384" width="16" height="16" rx="4" fill="hsl(0,0%,6%)" />
      <rect x="152" y="384" width="16" height="16" rx="4" fill="hsl(0,0%,6%)" />
      <rect x="180" y="384" width="16" height="16" rx="4" fill="hsl(0,0%,6%)" />
      <rect x="208" y="384" width="16" height="16" rx="4" fill="hsl(0,0%,6%)" />

      {/* Rainbow energy stream — fills the gap between video and music,
          cut into an arrow shape that rises on the left and descends on the right,
          making the "up/down" motion you described. No separation — it butts
          right against the video body on the left and the music note on the right. */}
      <path d="
        M 248 140
        L 340 80
        L 340 140
        L 456 140
        L 456 200
        L 340 200
        L 340 260
        L 248 200
        Z
      " fill="url(#brandRainbow)" />

      <path d="
        M 248 312
        L 340 260
        L 340 312
        L 456 312
        L 456 372
        L 340 372
        L 340 432
        L 248 372
        Z
      " fill="url(#brandRainbow)" />

      {/* Down-pointing chevron on left face of each arrow —
          these sit *on top of* the rainbow, creating the up/down
          visual rhythm you asked for. */}
      <polygon points="268,156 296,180 268,204" fill="hsl(0,0%,100%)" opacity="0.9" />
      <polygon points="268,328 296,352 268,376" fill="hsl(0,0%,100%)" opacity="0.9" />

      {/* Up-pointing chevron on right face — flips direction to show
          the two-way flow. */}
      <polygon points="436,156 408,180 436,204" fill="hsl(0,0%,100%)" opacity="0.9" />
      <polygon points="436,328 408,352 436,376" fill="hsl(0,0%,100%)" opacity="0.9" />

      {/* Music note — stem, flag, and note head.
          The stem butts directly against the right face of the
          rainbow arrows so there's zero gap. */}
      <rect x="456" y="120" width="24" height="272" rx="12" fill="currentColor" />
      {/* Upper flag */}
      <path d="
        M 480 120
        C 480 120, 468 140, 464 160
        L 464 160
        C 468 148, 480 136, 480 120
        Z
      " fill="currentColor" />
      {/* Note head (tilted ellipse) */}
      <ellipse cx="468" cy="416" rx="56" ry="36"
        transform="rotate(-30 468 416)"
        fill="currentColor" />
    </svg>
  );
}
