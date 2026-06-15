# Entropy

Entropy is a beautiful, locally-hosted, single-binary web interface for powerful command-line tools like `yt-dlp` and `ffmpeg`.

It brings the polish, fluid animations, and dynamic Material You theming expected from top-tier modern applications to the desktop environment, without the bloat of Electron or the privacy concerns of cloud-based SaaS tools.

## Features

- **Single Binary**: The entire React/Vite frontend is embedded directly inside a lightweight Go backend. No Node.js runtime required.
- **Local & Private**: Everything runs strictly on `localhost:8001`. Your files and data never leave your machine.
- **Material You Design**: Features dynamic, expressive theming using the official Google `material-color-utilities` HCT algorithm. The app seamlessly adapts to light/dark mode.
- **Real-Time Progress**: Live updates for download speed, ETA, and progress via Server-Sent Events (SSE).
- **Power User Options**: Granular control over formats, bitrates, metadata embedding, and concurrent workers.

## Prerequisites

Entropy acts as a UI wrapper. To use its features, you must have the underlying CLI tools installed on your system and available in your `$PATH`:

- `yt-dlp` (Core downloading engine)
- `ffmpeg` (Media conversion and merging)
- `aria2c` (Optional, for accelerated downloading)

We strongly recommend installing these via your operating system's package manager (e.g., `pacman`, `apt`, `brew`).

## Running Entropy

If you have downloaded a pre-compiled release:

```bash
# Run the binary
./entropy
```

Then, open your web browser and navigate to:
`http://127.0.0.1:8001`

## Building from Source

You will need Go 1.21+ and Node.js installed.

1. **Build the frontend:**
   ```bash
   cd frontend
   npm install
   npm run build
   ```

2. **Build the backend:**
   ```bash
   cd ../backend
   # The Go compiler will embed the frontend/build directory
   go build -ldflags "-s -w" -o entropy .
   ```

3. **Run your custom build:**
   ```bash
   ./entropy
   ```

## Documentation

For more information on the core principles guiding this project, please read:
- [PHILOSOPHY.md](./PHILOSOPHY.md) - Why Entropy exists.
- [DESIGN.md](./DESIGN.md) - Architecture and technical choices.
- [SECURITY.md](./SECURITY.md) - Security principles and local-first boundaries.
