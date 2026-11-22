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

## Next: operations

Make a running instance easier to observe and to trust.

- **`security.txt`.** A `/.well-known/security.txt` per RFC 9116. It names a
  contact for one specific deployment, so it has to come from deployment
  configuration and cannot be a static file in this repository.
- **Alerting guide.** `docs/alerting.md` with example Prometheus rules: 4xx
  and 5xx spikes, a rising `rate_limited` counter, Redis ping failures,
  container restart loops, certificate expiry.
- **Threat model additions.** Spell out adversaries that are covered today but
  not written down: an operator with Redis access, a malicious browser
  extension (out of scope), log poisoning through crafted paths and headers,
  and supply chain risk in container base images.
- **Version in `/api/health`.** Return the deployed version so operators do
  not have to trust the badge in the UI. Consider per-dependency detail in
  `/api/ready`.
- **Abuse protection for public instances.** IP rate limiting is the only
  control today. An opt-in captcha or proof-of-work step on
  `POST /api/v1/secrets`, behind a configuration flag and off by default.

## Then: polish

- **Proxy configuration as a template.** Ship `infra/Caddyfile.prod.example`
  and keep the real file out of version control, the same way `infra/.env`
  works. Operators who run several sites behind one proxy currently have to
  carry a local diff.
- **Branding through configuration.** Logo, product name and the source link
  set at build or run time, so a fork does not have to edit source files to
  look like its own.
- **Argon2id on slow devices.** Key derivation takes a few seconds on older
  phones. The plan is to keep the parameters fixed and explain the wait in the
  UI. Weakening them per device would defeat the point.
- **Smaller code issues.** `writeJSON` drops encoding errors without logging
  them, and expired rate-limit buckets are only cleaned up on every 128th
  request, which leaves them around for a long time under low traffic.

## Later, maybe

- **A managed-cloud deployment option.** Containers on a serverless platform
  with managed Redis, a managed load balancer for TLS and a WAF in front. It
  removes OS patching and brings observability with it, at roughly two to
  three times the cost of a single VM. Docker Compose on one host stays the
  reference setup either way.
