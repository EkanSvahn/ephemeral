# Changelog

Notable changes to Ephemeral, newest first. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html). While the version
is below 1.0, a minor release may require a manual step when upgrading. Such
steps are listed under "Upgrading".

## [Unreleased]

## [0.2.0] - 2026-11-23

The first release under the name Ephemeral and under the MIT license.

### Upgrading

- **Proxy configuration.** `infra/Caddyfile.prod` is no longer tracked. A
  deployment that edited that file needs a few commands once, before it checks
  out this version. See "Coming from 0.1.x" in the
  [deployment runbook](docs/deployment-runbook.md#4-update).
- **Repository address.** The repository moved to
  `github.com/EkanSvahn/ephemeral`. GitHub redirects the old address, but
  update the remote of existing clones:
  `git remote set-url origin git@github.com:EkanSvahn/ephemeral.git`.
- **Appearance.** Deployed instances now render in Space Mono, as the
  development server always did. See "Fixed" below.
- Links created with earlier versions keep working, including
  password-protected ones.

### Added

- MIT license, security policy, contribution guide, code of conduct, issue
  forms and a pull request template.
- `infra/Caddyfile.prod.example`: the proxy configuration as a template. The
  real file is gitignored, like `infra/.env`, so deploying no longer means
  editing a tracked file.
- `VITE_CODE_URL`: the source link in the navigation is a build setting.
- `infra/.env.example` documents `TRUSTED_PROXY_CIDR` and `METRICS_ENABLED`,
  and how the size limits depend on each other.
- The API image builds for the architecture of the host, so the stack runs on
  arm64 as well as amd64.
- A known-answer test for Argon2id. A library update that changes key
  derivation now fails the build before it can break links already shared.
- CI: `gofmt` and `go vet`, the frontend tested on Node 22 and 26, a job that
  follows the self-hosting steps and validates both compose files and the
  proxy configuration, a smoke test that starts the frontend image, and a
  check that a release tag matches the version shown in the UI.
- A full [threat model](docs/threat-model.md).

### Changed

- The project is named Ephemeral throughout. The Go module is
  `ephemeral/backend`.
- Go 1.27, Vite 8, Vitest 5, TypeScript 7, `@noble/hashes` 2.4, go-redis 9.23
  and testcontainers 0.45. The frontend image builds on Node 26 and is served
  by nginx 1.31.
- The rate limiter drops a client's bucket as soon as it has refilled, and
  sweeps once a minute. Before, buckets were kept for 30 minutes and swept on
  every 128th request.
- The deployment runbook is rewritten around the new configuration flow and
  explains what happens to unopened secrets during an update.

### Fixed

- **The font never loaded on a deployed instance.** The page linked Space Mono
  from Google Fonts, which the content security policy blocks. The font now
  ships with the app, and the page makes no request to a third party.
- **The development size limits contradicted each other.** They allowed
  250 MiB files but capped the ciphertext at 300 MiB, less than a 250 MiB file
  needs once it is encrypted and base64-encoded.
- The `/code` link pointed at `https://github.com` and not at the repository.
- The API no longer drops errors silently when a response cannot be written.

### Security

None of these exposed secret content. The server only ever holds ciphertext.

- **Client address behind a proxy.** The API took the first entry of
  `X-Forwarded-For`. Behind a proxy that appends to that header, a client
  could choose the address it was rate limited by. The header is now read from
  the right, skipping trusted proxies. Deployments using the shipped proxy
  configuration were not affected, because it replaces the header.
- **`HEAD` requests were not rate limited** on
  `/api/v1/secrets/{id}`.
- **IPv6 clients are rate limited per /64** and no longer per address.
- **Secret ids in logs.** The web server wrote share-link paths, which contain
  the id, to its access log. That log is now off. The API logs paths that
  match no route as `unmatched`, and accepts a client-supplied `X-Request-Id`
  only if it is short and made of harmless characters.

### Removed

- Two unused logo files, 4 MB together, from the frontend image.

## [0.1.6] - 2026-05-05

### Changed

- `/api/metrics` serves the Prometheus text format. The JSON output is gone,
  and metric names follow the `ephemeral_*_total` convention.
- With `REQUIRE_PASSWORD=true`, new secrets must use Argon2id with at least
  64 MiB of memory. PBKDF2 is rejected on upload. Existing PBKDF2 links can
  still be opened.

### Added

- Tests for CORS preflight, rate-limit isolation and `X-Forwarded-For` trust,
  and an integration suite against a real Redis that checks that a secret is
  delivered only once under contention.

## [0.1.5] - 2026-05-04

### Changed

- The production compose file holds no deployment-specific values. They live
  in `infra/.env` on the host, and `ALLOWED_ORIGINS` is required.

### Added

- `GET /api/ready`, which checks Redis. `GET /api/health` stays a pure
  liveness check.
- Tests for the browser-side encryption.
- Trivy scans of both container images in CI.

## [0.1.4] - 2026-04-29

- Version number updated. No functional change.

## [0.1.3] - 2026-04-26

### Changed

- Reworked create and open flows in the UI.
- File limit raised to 50 MiB.

### Fixed

- Share links did not resolve when opened directly.

## [0.1.2] - 2026-02-11

### Added

- `TRUSTED_PROXY_CIDR`: the client address behind a reverse proxy is used for
  logging and rate limiting.
- `METRICS_ENABLED`: `/api/metrics` is off unless enabled.
- API responses are sent with `Cache-Control: no-store`.
- Stale rate-limiter buckets are cleaned up.

## [0.1.1] - 2026-02-11

### Added

- Deployment runbook with rollback and release procedure.
- The UI shows the version it was built from.

## [0.1.0] - 2026-02-11

First release. Text and file secrets encrypted in the browser, one view by
default, optional password with Argon2id, strict request validation, security
headers, and a Redis self-check that refuses persistence.

[Unreleased]: https://github.com/EkanSvahn/ephemeral/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/EkanSvahn/ephemeral/compare/v0.1.6...v0.2.0
[0.1.6]: https://github.com/EkanSvahn/ephemeral/compare/v0.1.5...v0.1.6
[0.1.5]: https://github.com/EkanSvahn/ephemeral/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/EkanSvahn/ephemeral/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/EkanSvahn/ephemeral/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/EkanSvahn/ephemeral/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/EkanSvahn/ephemeral/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/EkanSvahn/ephemeral/releases/tag/v0.1.0
