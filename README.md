# Ephemeral

Self-hosted secret sharing for notes and files. A secret is encrypted in the
browser, stored on the server as ciphertext, and deleted once it has been
opened or when its time runs out.

Ephemeral follows the same pattern as PrivNote and Cryptgeon. It is built on Go
and Vue, and ships with hardened defaults for running it yourself.

## How it works

1. The browser creates a random AES-256-GCM key, or derives one from a
   password with Argon2id, and encrypts the note or file locally.
2. Only the ciphertext and a small metadata record are sent to the server,
   which stores them in Redis with a view limit or a time limit.
3. The share link carries the key in the URL fragment (`#...`), which browsers
   do not send to the server. In password mode the link carries no key at all.
4. Opening the link uses up a view. A Redis script returns the ciphertext and,
   when the last view is spent, deletes it in the same atomic step. Secrets
   with a time limit expire on their own. The browser decrypts.

## Features

- Text notes and single files, with a size limit and MIME allowlist set by the
  operator
- One view by default, or a chosen number of views, or a time limit
- Optional password, and a password-only mode for deployments that do not want
  keys in links
- Argon2id for password-derived keys; older PBKDF2 links can still be opened
- Per-IP rate limiting, strict request validation and payload caps
- Security headers, a content security policy and structured logs that never
  contain payloads or keys
- Redis runs without persistence, and the API refuses to start against a Redis
  that persists to disk
- Health, readiness and status endpoints, plus optional Prometheus metrics

## Quick start

You need Docker with the Compose plugin. Clone the repository, then:

```bash
cd infra
docker compose -f docker-compose.dev.yml up --build
```

Open `http://localhost:5173`. The API listens on `http://localhost:8080`.

This is a development setup with hot reload, generous limits and Redis exposed
on the host. Do not put it on the internet.

## Self-hosting

The production template runs four containers: Caddy for TLS and security
headers, nginx serving the built frontend, the Go API, and Redis without
persistence.

1. Point DNS for your domain at the host.
2. Create your configuration from the two templates:
   ```bash
   cp infra/.env.example infra/.env
   cp infra/Caddyfile.prod.example infra/Caddyfile.prod
   ```
3. Set `ALLOWED_ORIGINS=https://your-domain` in `infra/.env` and review the
   rest. Replace the example domain in `infra/Caddyfile.prod`.
4. Start it:
   ```bash
   cd infra
   docker compose -f docker-compose.prod.yml up -d --build
   ```
5. Check that it is ready:
   ```bash
   curl -fsS https://your-domain/api/ready
   ```

Both files are gitignored. No tracked file has to be edited, so a later update
is a `git pull` and a rebuild.

Before you expose an instance, read the
[production hardening baseline](docs/production-hardening.md), the
[security checklist](docs/security-checklist.md) and the
[deployment runbook](docs/deployment-runbook.md).

## Configuration

Everything is configured through environment variables.
[`infra/.env.example`](infra/.env.example) documents the production set, and
[`.env.example`](.env.example) lists every variable with local development
values. The ones that change behaviour the most:

| Variable | Default | Purpose |
| --- | --- | --- |
| `ALLOWED_ORIGINS` | required in production | Origins allowed to call the API |
| `REQUIRE_PASSWORD` | `true` in the production template | Password-only mode. `VITE_REQUIRE_PASSWORD` must match |
| `MAX_FILE_BYTES` | 50 MiB | Largest file accepted |
| `ALLOWED_FILE_MIME_TYPES` | PDF, PNG, JPEG, plain text, octet-stream | File types accepted |
| `MAX_TTL_SECONDS` | `86400` | Longest lifetime a secret can be given |
| `MAX_VIEWS` | `100` | Highest view count a secret can be given |
| `RATE_LIMIT_RPM`, `RATE_LIMIT_BURST` | `60`, `20` | Per-IP limit on each secret endpoint |
| `TRUSTED_PROXY_CIDR` | `172.16.0.0/12` in the production template | Proxy network whose `X-Forwarded-For` is trusted. Without it all visitors share one rate limit |
| `STRICT_REDIS_EPHEMERAL` | `true` | Refuse to start if Redis persistence is on |
| `METRICS_ENABLED` | `false` | Expose `/api/metrics` |

Raising the file limit means raising `MAX_CIPHERTEXT_BYTES` and
`MAX_BODY_BYTES` with it, because an encrypted file is about a third larger on
the wire. [`infra/.env.example`](infra/.env.example) shows the arithmetic.
Files are encrypted in browser memory and stored in Redis, so the design suits
secrets and small to medium files. For something like a database backup, send
the file another way and use Ephemeral for the key.

## API

| Method and path | Purpose |
| --- | --- |
| `GET /api/health` | Liveness. Returns 200 while the process is up |
| `GET /api/ready` | Readiness. Returns 200 when Redis is reachable, otherwise 503 |
| `GET /api/status` | Policy and limits for this deployment |
| `GET /api/metrics` | Prometheus text format, only when `METRICS_ENABLED=true` |
| `POST /api/v1/secrets` | Store a secret. Takes `views` (default 1) or `ttl_seconds` |
| `GET /api/v1/secrets/{id}` | Check that a secret exists without using up a view |
| `POST /api/v1/secrets/{id}/consume` | Return the ciphertext and metadata, and count the view |

## Security model

What the server stores:

- the ciphertext
- a metadata record with the format version, the secret type, the algorithm,
  and for password-protected secrets the KDF parameters and salt
- for files, the file name, MIME type and size, which the server needs in
  order to enforce the upload policy

What the server never receives in the default flow: the plaintext, the key, or
the password.

Ephemeral does not protect against:

- compromised sender or recipient devices and browsers
- weak or reused passwords
- a secret being copied somewhere insecure after it has been opened
- the link and the password being sent together through an insecure channel
- a misconfigured deployment: DNS, TLS, headers, logging or policy settings

Treat it as a hardened way to hand a secret over. It does not replace endpoint
security. The full picture is in the [threat model](docs/threat-model.md).

## Project status

Ephemeral is maintained by one person and runs in production on a small scale.
It has not had an independent security audit. Read the threat model and decide
whether that fits what you want to use it for.

Planned work is in the [roadmap](docs/roadmap.md).

## Making it your own

Forks are welcome. The places you will most likely want to change:

- **Domain and policy:** `infra/.env` and `infra/Caddyfile.prod`
- **Source link in the navigation:** `VITE_CODE_URL` in `infra/.env`
- **Logo and favicon:** `frontend/public/ephemeral_logo.svg`
- **Colours and typography:** `frontend/src/assets/main.css`
- **Text on the info page:** `frontend/src/views/InfoView.vue`

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the development setup, tests and
conventions.

## Security

Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE)
