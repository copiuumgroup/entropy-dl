# Design & Architecture

Entropy is built to be a lightweight, single-binary application that marries a high-performance backend with a modern, dynamic web frontend — and runs identically as a single-user desktop tool or a multi-user homelab service.

## High-Level Architecture

Entropy uses a client-server model wrapped into a single desktop application experience:

1. **Backend (Go):** A lightweight Go server manages application logic, executes shell commands, tracks job state, persists settings/users to bbolt, and serves the frontend assets.
2. **Frontend (React + Vite):** A modern Single Page Application providing a rich, responsive user interface, gated behind an auth screen when run in homelab mode.
3. **Single Binary Distribution:** Through Go's `embed` package, the compiled React frontend is embedded directly into the Go binary. The user only needs to download and run a single executable.

## Operating Modes

The same binary operates in one of two modes, decided at startup by the bind address:

- **Local mode** (`HOST` unset or `127.0.0.1`/`localhost`): no auth, no TLS, synthetic admin injected per request. The friction-free desktop experience.
- **Homelab mode** (any non-loopback `HOST`): TLS with an auto-generated self-signed cert, mandatory named-account auth, per-user job isolation. Browser auto-launch is skipped.

The bind-address guard (`bind.go`) makes the transition deliberate: a non-loopback bind refuses to start without `USE_HTTPS=1` and `ADMIN_PASSWORD` set. You cannot accidentally expose an unauthenticated server.

## The Frontend: Material You & React

### Dynamic Theming (HCT)
Entropy implements the **Material You (M3)** design system. Rather than hard-coding static themes, the frontend uses the official `@material/material-color-utilities` library to generate a comprehensive color palette at runtime using the HCT (Hue, Chroma, Tone) color space.
- The UI adapts seamlessly between light and dark modes based on the user's OS preference (`prefers-color-scheme`).
- The OS accent color is polled periodically so changes apply without an app restart.
- All M3 components (chips, switches, nav-rail indicator, toasts, view transitions, stagger lists, progress bars) are hand-built in `components/m3/` on top of Framer Motion springs — no heavyweight component framework is pulled in.

### Animation System
Motion is a first-class design surface. A shared spring config (`{ stiffness: 400, damping: 30 }`) drives view transitions, the navigation-rail indicator, toasts, chip taps, and staggered list entrances. `prefers-reduced-motion` is honored throughout — animations collapse to instant state changes.

### Auth Gate
A `LoginScreen` component wraps the entire app. On mount it calls `GET /api/me`:
- If the response carries `loopback: true` (local mode) or a valid session, the main app renders immediately.
- If `401`, it probes `/api/setup` to distinguish first-run (no users → setup form) from returning users (login form).
- A global axios response interceptor watches for `401` from any API call mid-session and routes the user back to the login screen — so an expired session never leaves the UI in a broken state.

### State Management
- React hooks and context are used for local UI state.
- Real-time updates from the backend (such as download progress) are pushed to the frontend via **Server-Sent Events (SSE)**, filtered per-subscriber by job owner.

## The Backend: Go

### Persistence (bbolt)
All durable state lives in a single `entropy.db` bbolt file:
- `auth_users` bucket — named accounts with bcrypt-hashed passwords.
- Jobs, settings, and smart-routing state in their own buckets.

This keeps the single-file state story intact: back up one file, back up everything.

### Authentication & Sessions
The `pkg/auth` package is loopback-unaware — the "skip auth on localhost" policy lives in the HTTP layer, not in the auth package itself.
- **Users** are created via `POST /api/setup` (first run only — returns `409` once any user exists) or the admin-only `POST /api/users` endpoint.
- **Sessions** are in-memory only: a `crypto/rand` 256-bit token mapped to a username, with a 24h expiry. A restart logs everyone out, which is the safest default for a household tool.
- The session cookie is `HttpOnly`, `SameSite=Lax`, and `Secure` under TLS.
- Deleting a user refuses to remove the last admin, preventing lockout.

### Job Management & SSE
- The backend maintains an in-memory queue and state tracker for all active and completed jobs.
- Every job carries an `Owner` field. All `Manager` methods are owner-scoped: `List(owner)`, `Stats(owner)`, `RetryAllFailed(owner)`, `Subscribe(owner)`. A user can only see and act on their own jobs.
- When a job's state changes (e.g. download percentage increases), the backend broadcasts an SSE event to subscribers, filtered so a user only receives events for their own jobs.

### Library
The `pkg/library` package provides traversal-safe directory listing and file resolution over the configured audio/video output directories.
- `ListDir` rejects `..` early, resolves with `filepath.Join`, and verifies the result is contained under the root via `HasPrefix`. Hidden files (`dead_links.csv`, `*.part`, `*.ytdl`, `*.tmp`, dotfiles) are filtered.
- `ResolveFile` applies the same containment check and is consumed by the file-serving handler, which additionally enforces an audio/video/image extension allowlist.
- File serving uses `http.ServeFile`, which natively handles HTTP Range/206 requests for seeking in `<audio>`/`<video>` elements.

### Subprocess Execution
- Go's `os/exec` package spawns and manages external processes like `yt-dlp`.
- Arguments are passed as a strict array — never a shell string — to eliminate injection.
- stdout and stderr are captured, parsed via regex to extract progress metrics (percentage, speed, ETA), and broadcast to the frontend.

## The Zero-Dependency Goal
By embedding the web assets into the Go binary and leveraging the browser as the rendering engine (via a standard web view, or by opening the user's default browser to the bound address), Entropy avoids the immense overhead of frameworks like Electron. The result is a lightning-fast application with a minimal memory footprint — equally happy on a desktop or a headless home server.
