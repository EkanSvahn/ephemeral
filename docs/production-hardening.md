# Production Hardening Baseline

## 1) Reverse Proxy and TLS
- Copy `infra/Caddyfile.prod.example` to `infra/Caddyfile.prod` and set your real domain.
- Terminate TLS at the proxy and enable HSTS.
- Route `/api/*` to backend and everything else to frontend.
- The shipped configuration replaces `X-Forwarded-For` with the address the proxy sees. Keep that if you write your own, so a client cannot choose the address it is rate limited by.

## 2) Required Environment Policy
- `REQUIRE_PASSWORD=true` for organization/internal deployments.
- `ALLOWED_ORIGINS` set to your real HTTPS origin.
- `TRUSTED_PROXY_CIDR` set to your reverse-proxy network for correct client IP handling.
- `STRICT_REDIS_EPHEMERAL=true` to fail startup if Redis persistence is enabled.
- `METRICS_ENABLED=false` on public deployments unless explicitly needed.
- Set `MAX_FILE_BYTES` and `ALLOWED_FILE_MIME_TYPES` to your policy limits.
- Keep Redis private (no public ports exposed).

## 3) Header Baseline
- `Strict-Transport-Security`
- `Content-Security-Policy`
- `X-Content-Type-Options`
- `X-Frame-Options`
- `Referrer-Policy`
- `Permissions-Policy`

## 4) Operational Baseline
- Enable CI dependency scanning (npm + Go vulnerability checks).
- Keep dependency updates active via Dependabot.
- Review logs only for metadata: request id, route template, status, duration.
- Never log secret payloads, decryption keys, or URL fragments.
