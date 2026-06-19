# Security

Entropy is designed from the ground up to operate securely — both as a single-user desktop tool and as a multi-user homelab service. Because it acts as an interface to powerful underlying command-line utilities (`yt-dlp`, `ffmpeg`, `aria2c`) and reads from your filesystem, security is treated with the utmost importance.

## Core Security Principles

### 1. Zero Telemetry & Air-Gapped Operation
Entropy operates entirely locally. It does not send analytics, usage data, or crash reports to any remote server. Once the binary and its CLI dependencies are downloaded, Entropy can function completely offline (with the obvious exception of tasks that inherently require network access, such as downloading video streams).

### 2. Two Operating Modes

Entropy has a single binary that operates in one of two mutually exclusive modes, determined entirely by the bind address.

**Local mode (default — loopback bind):**
- The server binds to `127.0.0.1`.
- It cannot be accessed by other devices on your LAN or the open internet.
- Authentication is bypassed: a synthetic admin identity is injected into every request, so the single-user desktop experience has zero friction.
- TLS is not required because the loopback interface is not network-reachable.

**Homelab mode (non-loopback bind):**
- The server binds to a LAN-facing interface (e.g. `HOST=0.0.0.0` or `HOST=192.168.1.10`).
- TLS is mandatory — a self-signed certificate (with the LAN IP in the SAN) is auto-generated.
- Named-account authentication is mandatory — there is no unauthenticated path to any API endpoint except `/api/login`, `/api/setup`, and `/api/me`.
- Browser auto-launch is disabled (headless servers don't have a desktop to open).

### 3. The Network-Exposure Guard
The most important safety mechanism in the codebase is the bind-address guard (`backend/bind.go`). It turns accidental network exposure into a **hard startup error** rather than a silent footgun.

If `HOST` is set to anything other than loopback, Entropy **refuses to start** unless both `USE_HTTPS=1` and `ADMIN_PASSWORD=<password>` are also set. You cannot accidentally ship an unauthenticated download orchestrator onto your network by flipping a single environment variable.

### 4. Authentication

**Credential storage:**
- Named users are persisted in bbolt (`entropy.db`) in a dedicated `auth_users` bucket.
- Passwords are hashed with **bcrypt at cost 12**. Plaintext is never stored or logged.
- User records returned by the API have their password hash field cleared before serialization.

**Sessions:**
- On successful login, the server issues a **256-bit cryptographically random session token** (`crypto/rand`).
- Sessions are **in-memory only** — a server restart logs everyone out. This is the safest default for a household tool.
- Session lifetime is **24 hours**. Expired tokens are lazily purged on verification.
- The session cookie is `HttpOnly` (no JS access), `SameSite=Lax`, and `Secure` when TLS is enabled.

**First-run setup:**
- `POST /api/setup` creates the first admin account. It returns `409 Conflict` once any user exists, so it cannot be used to create a second admin later.
- Alternatively, setting `ADMIN_PASSWORD` at startup bootstraps the admin non-interactively.
- The `Delete` user operation refuses to remove the **last** admin, so the box can never be locked out of admin access.

**Account verification:**
- `VerifyPassword` returns a generic `invalid username or password` error whether the username is wrong or the password is wrong, so an attacker cannot enumerate accounts via timing or error differences.

### 5. Per-User Job Isolation
In homelab mode every job carries an `Owner` field set to the authenticated username. All job-listing, retry, delete, and stats endpoints are **owner-scoped** — a user can only see and act on their own jobs. The live SSE feed filters events per subscriber so a user never sees another user's downloads.

### 6. Path-Traversal Protection (Library)
The library browser serves files from the configured audio/video output directories. It is hardened with three independent layers:

1. **Early rejection** — any path containing `..` is rejected in both `ListDir` and `ResolveFile` before any filesystem access.
2. **Prefix containment** — the resolved absolute path is checked with `filepath.Clean` + `filepath.HasPrefix` against the root directory. A path that escapes the root is refused even if it contains no literal `..`.
3. **Extension allowlist** — `handleLibraryFile` only serves files whose extension is on the audio/video/image allowlist. Even a path that somehow passed the containment check cannot serve arbitrary files (e.g. `.db`, `.exe`, `.txt`).

Hidden files (`dead_links.csv`, `*.part`, `*.ytdl`, `*.tmp`, dotfiles) are filtered from directory listings but remain resolvable by direct path — filtering is a listing concern, not an access-control concern.

### 7. Command-Injection Prevention
Entropy translates UI inputs into shell commands for tools like `yt-dlp`. To prevent arbitrary code execution and command injection:
- User inputs (URLs, output directories, filenames) are rigorously sanitized.
- The backend uses strict argument-array passing (`exec.Command("yt-dlp", args...)`) rather than passing raw strings to a shell, mitigating standard shell-injection attacks.
- Input validation ensures parameters match expected formats before they reach the underlying binaries.

### 8. CORS & Same-Origin
- CORS is restricted so malicious websites cannot interact with the local API from a browser.
- All API calls in the frontend go through a single axios instance, which carries the session cookie automatically on same-origin requests.

### 9. Dependency Isolation
Entropy requires third-party binaries to function.
- We strongly recommend installing these binaries (`yt-dlp`, `ffmpeg`, `aria2c`) through your OS's official package managers (e.g. `apt`, `pacman`, `brew`) to ensure they receive security updates through trusted channels.
- Entropy checks the system `$PATH` for these binaries and does not download or execute untrusted binaries at runtime.

### 10. Private by Default
Any files downloaded, manipulated, or created by Entropy are stored strictly within the user-defined output directories. The application does not index your hard drive, read files outside of its working scope, or upload your data to any cloud service.

## Reporting Vulnerabilities
If you discover a security vulnerability in Entropy, please report it privately via our issue tracker or by contacting the maintainers directly. Do not disclose vulnerabilities publicly until a patch has been issued.
