# Entropy Philosophy

## The Local-First Imperative
In an era increasingly dominated by subscription models, "cloud computing," and SaaS lock-in, **Entropy** takes a definitive stance: software should run locally, securely, and entirely under the user's control.

The modern internet has shifted power away from the user and into the hands of service providers. We are forced to upload our private files to servers we don't control, often paying monthly fees just to perform basic utilities like video conversion, file downloading, or metadata extraction.

Entropy exists to reverse this trend.

## Empowering the User
Powerful CLI utilities (like `yt-dlp`, `ffmpeg`, and `aria2c`) already exist to perform almost any media manipulation or downloading task for free, indefinitely. However, these tools suffer from steep learning curves, obscure flags, and terminal-only interfaces that lock out the vast majority of non-technical users.

Our philosophy is simple: **Democratize powerful utilities by wrapping them in a beautiful, intuitive interface.**

We believe that:
1. **You own your hardware.** You should use its compute power, not rent a fraction of a core in the cloud.
2. **You own your data.** Your files should never leave your machine unless you explicitly choose to share them.
3. **Software should be a tool, not a landlord.** Once you have the binary, it's yours. No telemetry, no DRM, no subscriptions, no forced updates.

## From Single Desktop to Household
Entropy started as a single-user desktop tool: run the binary, open `127.0.0.1:8001`, download. That experience is sacred and will never change — it is the default, and it requires zero configuration.

But the same hardware you own is often shared. A home server, an old laptop in the closet, a media box under the TV — these are all *your* machines, on *your* network. Entropy's homelab mode lets the same single binary serve every device in the house, behind TLS and named accounts, with no cloud in the loop.

The key design principle here is **opt-in exposure with safe defaults**: binding to the network is never accidental. The guard refuses to start unless TLS and auth are both configured, so the friction-free desktop story is preserved while the multi-device story is a deliberate, hardened choice.

## Design as a Feature
Local utilities shouldn't look or feel like second-class citizens. Entropy brings the polish, fluid animations, and dynamic theming (Material You) expected from top-tier mobile and web applications to the desktop environment. Beautiful software encourages exploration and makes complex tasks approachable.

## The Web as a Universal UI
By leveraging modern web technologies (Go + React + Vite) inside a self-contained, single-binary distribution, Entropy ensures that the application runs natively and identically across Linux, Windows, and macOS, without the bloat of an Electron wrapper — and equally well on a phone browser pointed at a homelab server.

## Summary
Entropy is a love letter to the power user, and an olive branch to the everyday user. It is local, private, beautifully designed, and fiercely independent — equally at home on a single laptop or serving a whole household.
