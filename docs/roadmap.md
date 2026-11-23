# Roadmap

Where Ephemeral is and what is planned. The order reflects priority, and none
of it is a promise with a date attached. If you want to pick something up,
open an issue first so the work is not done twice.

## Done

**Production readiness baseline**

- Test suite for the browser crypto: round trips for text and files with and
  without a password, legacy PBKDF2 decryption, tamper rejection, base64url
  edge cases, salt and IV uniqueness
- `GET /api/ready` checks Redis; `GET /api/health` stays a pure liveness check
- Server write timeout follows `REQUEST_TIMEOUT_MS`, so large file responses
  are no longer cut off
- Frontend image built with `npm ci` and served by nginx; both images scanned
  with Trivy in CI, which fails on HIGH and CRITICAL findings
- Production values moved out of the compose file into `infra/.env`

**Test and robustness coverage**

- Tests for spoofed `X-Forwarded-For` with missing or invalid
  `TRUSTED_PROXY_CIDR`, for rate-limit isolation between clients, and for CORS
  preflight with allowed and disallowed origins
- Integration suite against a real Redis through testcontainers, covering the
  consume script under contention
- Password-only mode rejects PBKDF2 for new secrets and raises the Argon2id
  memory floor; existing PBKDF2 links still open
- `/api/metrics` serves Prometheus text format
- Third-party CI actions pinned to commit SHAs

**Deploying without editing tracked files**

- Proxy configuration ships as `infra/Caddyfile.prod.example`. The real file
  is gitignored, the same way `infra/.env` is, so an operator who runs several
  sites behind one proxy no longer carries a local diff
- The source link in the navigation is a build setting, `VITE_CODE_URL`
- The API image builds for the architecture of the host, amd64 or arm64
- CI follows the self-hosting steps and validates both compose files and the
  proxy configuration

**Hardening before the first public release**

- Client address behind a proxy is read from the right of
  `X-Forwarded-For`, `HEAD` requests are rate limited, IPv6 clients are
  counted per /64, and refilled rate-limit buckets are dropped every minute
- Secret ids no longer appear in any log the shipped stack writes
- The font ships with the app. The page makes no third-party request
- A full threat model, a changelog, and issue and pull request templates

## Next: operations

Make a running instance easier to observe and to trust.

- **`security.txt`.** A `/.well-known/security.txt` per RFC 9116. It names a
  contact for one specific deployment, so it has to come from deployment
  configuration and cannot be a static file in this repository.
- **Alerting guide.** `docs/alerting.md` with example Prometheus rules: 4xx
  and 5xx spikes, a rising `rate_limited` counter, Redis ping failures,
  container restart loops, certificate expiry.
- **An upper lifetime for view-limited secrets.** A secret with a view limit
  stays in Redis until it is opened or Redis restarts. Applying
  `MAX_TTL_SECONDS` to every secret would put a ceiling on that. It changes
  behaviour for links that are opened days later, so it needs a setting and a
  note in the UI.
- **A memory cap for Redis.** The shipped configuration sets an eviction
  policy but no `maxmemory`. Decide on a default, and on what the API should
  answer when the store is full.
- **Version in `/api/health`.** Return the deployed version so operators do
  not have to trust the badge in the UI. Consider per-dependency detail in
  `/api/ready`.
- **Abuse protection for public instances.** IP rate limiting is the only
  control today. An opt-in captcha or proof-of-work step on
  `POST /api/v1/secrets`, behind a configuration flag and off by default.

## Then: polish

- **Branding through configuration.** Logo and product name set at build or
  run time, so a fork does not have to edit source files to look like its
  own. The source link already works this way.
- **Argon2id on slow devices.** Key derivation takes a few seconds on older
  phones. The plan is to keep the parameters fixed and explain the wait in the
  UI. Weakening them per device would defeat the point.
- **Authenticated metadata.** File name, MIME type and size are stored next
  to the ciphertext without being covered by the authentication tag. Binding
  them to the ciphertext needs a new metadata version, with the current one
  still readable.

## Later, maybe

- **Published container images.** Images on a registry, so that running an
  instance does not require building one. The frontend takes two settings at
  build time, the source link and the password-mode fallback, and they have to
  move to run time first.
- **A managed-cloud deployment option.** Containers on a serverless platform
  with managed Redis, a managed load balancer for TLS and a WAF in front. It
  removes OS patching and brings observability with it, at roughly two to
  three times the cost of a single VM. Docker Compose on one host stays the
  reference setup either way.
