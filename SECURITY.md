# Security

Entropy is designed from the ground up to operate securely on the user's local machine. Because it acts as an interface to powerful underlying command-line utilities (like `yt-dlp` and `ffmpeg`), security must be treated with the utmost importance.

## Core Security Principles

### 1. Zero Telemetry & Air-Gapped Operation
Entropy operates entirely locally. It does not send analytics, usage data, or crash reports to any remote server. Once the binary and its CLI dependencies are downloaded, Entropy can function completely offline (with the obvious exception of tasks that inherently require network access, such as downloading video streams).

### 2. Localhost Binding
The Entropy Go backend spawns a lightweight web server to serve the frontend UI and handle API requests.
- The server binds strictly to `127.0.0.1` (localhost).
- It cannot be accessed by other devices on your local network (LAN) or the open internet.
- CORS (Cross-Origin Resource Sharing) is restricted to prevent malicious websites from interacting with the local API.

### 3. Command Injection Prevention
Entropy translates UI inputs into shell commands for tools like `yt-dlp`. To prevent arbitrary code execution and command injection:
- User inputs (URLs, output directories, filenames) are rigorously sanitized.
- The backend uses strict argument array passing (`exec.Command("yt-dlp", args...)`) rather than passing raw strings to a shell, mitigating standard shell injection attacks.
- Input validation ensures that parameters match expected formats before they ever reach the underlying binaries.

### 4. Dependency Isolation
Entropy requires third-party binaries to function.
- We strongly recommend users install these binaries (`yt-dlp`, `ffmpeg`, `aria2c`) through their operating system's official package managers (e.g., `apt`, `pacman`, `brew`) to ensure they receive security updates through trusted channels.
- Entropy checks the system `$PATH` for these binaries and does not attempt to download or execute untrusted binaries dynamically at runtime.

### 5. Private by Default
Any files downloaded, manipulated, or created by Entropy are stored strictly within the user's defined output directories. The application does not index your hard drive, read files outside of its working scope, or upload your data to any cloud service.

## Reporting Vulnerabilities
If you discover a security vulnerability in Entropy, please report it privately via our issue tracker or by contacting the maintainers directly. Do not disclose vulnerabilities publicly until a patch has been issued.
