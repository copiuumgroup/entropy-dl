# Design & Architecture

Entropy is built to be a lightweight, single-binary application that marries a high-performance backend with a modern, dynamic web frontend.

## High-Level Architecture

Entropy uses a client-server model wrapped into a single desktop application experience:

1. **Backend (Go):** A lightweight Go server manages the application logic, executes shell commands, tracks job state, and serves the frontend assets.
2. **Frontend (React + Vite):** A modern Single Page Application (SPA) providing a rich, responsive user interface.
3. **Single Binary Distribution:** Through Go's `embed` package, the compiled React frontend is embedded directly into the Go binary. This means the user only needs to download and run a single executable file.

## The Frontend: Material You & React

### Dynamic Theming (HCT)
Entropy implements the **Material You (M3)** design system. Rather than hard-coding static themes, the frontend uses the official `@material/material-color-utilities` library to generate a comprehensive color palette at runtime using the HCT (Hue, Chroma, Tone) color space.
- The UI adapts seamlessly between light and dark modes based on the user's OS preference (`prefers-color-scheme`).
- A highly cohesive, accessible, and premium visual experience is guaranteed by the underlying color science of the HCT algorithm.

### State Management
- React hooks and context are used for local UI state.
- Real-time updates from the backend (such as download progress) are pushed to the frontend via **Server-Sent Events (SSE)**.

## The Backend: Go

### Job Management & SSE
- The backend maintains an in-memory queue and state tracker for all active and completed jobs.
- When a job's state changes (e.g., download percentage increases), the backend broadcasts an SSE event to the connected frontend clients, ensuring the UI remains perfectly in sync with the underlying terminal process without polling.

### Subprocess Execution
- Go's `os/exec` package is used to spawn and manage external processes like `yt-dlp`.
- stdout and stderr from these processes are captured, parsed via regex to extract meaningful progress metrics (percentage, speed, ETA), and broadcasted to the frontend.

## The Zero-Dependency Goal
By embedding the web assets into the Go binary and leveraging the browser as the rendering engine (via a standard web view or by opening the user's default browser to `127.0.0.1:8001`), Entropy avoids the immense overhead of frameworks like Electron. The result is a lightning-fast application with a minimal memory footprint.
