# Entropy

Entropy is a beautiful, self-hostable, single-binary web interface for powerful command-line tools like `yt-dlp` and `ffmpeg`.

It brings the polish, fluid animations, and dynamic Material You theming expected from top-tier modern applications to your desktop — without the bloat of Electron or the privacy concerns of cloud-based SaaS tools.

## Features

- **Single Binary**: The entire React/Vite frontend is embedded directly inside a lightweight Go backend. No Node.js runtime required.
- **Two deployment modes**:
  - **Local mode** (default): binds to `127.0.0.1`. No auth, no TLS — the friction-free single-user desktop experience. Just run the binary.
  - **Homelab mode**: bind to a LAN interface, serve over self-signed TLS, and authenticate with named accounts. Run the same binary on a server and use it from every device in the house.
- **Named accounts & sessions**: bcrypt-hashed credentials in bbolt, 256-bit in-memory session tokens, `HttpOnly` + `SameSite=Lax` cookies. Local mode auto-injects a synthetic admin so the desktop experience is unchanged.
- **Per-user isolation**: every job carries an owner; the queue, stats, and live SSE feed are scoped to the authenticated user.
- **Built-in library**: browse and stream your downloaded audio/video straight from the browser, with traversal-safe path handling, hidden-file filtering, and HTTP Range/206 support.
- **Material You design**: dynamic theming using the official `@material/material-color-utilities` HCT algorithm. The app seamlessly adapts to light/dark mode and your OS accent color.
- **Real-time progress**: live updates for download speed, ETA, and progress via Server-Sent Events (SSE).
- **Power-user options**: granular control over formats, bitrates, metadata embedding, smart media-type routing, and concurrent workers.

## Prerequisites

Entropy acts as a UI wrapper. To use its features, you must have the underlying CLI tools installed on your system and available in your `$PATH`:

- `yt-dlp` (core downloading engine)
- `ffmpeg` (media conversion and merging)
- `aria2c` (optional, for accelerated downloading)

We strongly recommend installing these via your operating system's package manager (e.g. `pacman`, `apt`, `brew`).

## Running Entropy

### Local mode (default)

If you have downloaded a pre-compiled release:

```bash
./entropy
```

Then open your web browser at `http://127.0.0.1:8001`. No configuration is needed — auth is bypassed on loopback.

### Homelab mode

To expose Entropy on your local network (so phones, laptops, and other machines on the LAN can use it), set three environment variables:

```bash
# Linux / macOS
HOST=0.0.0.0 USE_HTTPS=1 ADMIN_PASSWORD=change-me ./entropy

# Windows (cmd)
set HOST=0.0.0.0
set USE_HTTPS=1
set ADMIN_PASSWORD=change-me
entropy
```

- `HOST` — the interface to bind to. Anything other than `127.0.0.1`/`localhost` is treated as network-exposed.
- `USE_HTTPS=1` — enables TLS with an auto-generated self-signed certificate (with your LAN IP in the SAN).
- `ADMIN_PASSWORD` — bootstraps the first admin account. You can also create the admin via the first-run setup screen in the browser.

Entropy **refuses to start** if you set `HOST` to a non-loopback address without also enabling TLS and auth. This turns accidental exposure into a loud error instead of a silent footgun.

Then browse to `https://<your-LAN-IP>:8001`, accept the self-signed cert warning, and sign in.

### Environment variables

| Variable | Default | Purpose |
|---|---|---|
| `HOST` | `127.0.0.1` | Network interface to bind to. Loopback = local mode; anything else = homelab mode (requires TLS + auth). |
| `PORT` | `8001` | TCP port to listen on. |
| `USE_HTTPS` | unset | Set to `1` to enable TLS with an auto-generated self-signed certificate. |
| `ADMIN_PASSWORD` | unset | Bootstraps the first admin user on startup (also satisfies the "auth configured" guard for non-loopback binds). |
| `ENTROPY_NO_LAUNCH` | unset | Set to any value to skip auto-opening a browser window on startup. Always skipped in homelab mode. |

## Building from Source

You will need Go 1.21+ and Node.js installed.

1. **Build the frontend:**
   ```bash
   cd frontend
   npm install
   npm run build
   ```

2. **Copy the build into the embed directory:**
   ```bash
   # from the repo root
   rm -rf backend/webdist/assets && cp -r frontend/build/. backend/webdist/
   ```

3. **Build the backend** (the Go compiler embeds `backend/webdist`):
   ```bash
   cd backend
   go build -ldflags "-s -w" -o entropy .
   ```

4. **Run your custom build:**
   ```bash
   ./entropy
   ```

## Documentation

For more information, please read:
- [PHILOSOPHY.md](./PHILOSOPHY.md) — Why Entropy exists.
- [DESIGN.md](./DESIGN.md) — Architecture and technical choices.
- [SECURITY.md](./SECURITY.md) — Security model, network-exposure guard, auth, and library traversal protection.
