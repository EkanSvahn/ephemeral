# Contributing

Ephemeral is a small project with a narrow job: share a secret once, then
forget it. Contributions that keep it small and easy to reason about are the
ones most likely to be merged.

## Before you start

- **Bug fixes and small improvements:** open a pull request directly.
- **New features or larger changes:** open an issue first so the approach can
  be agreed before you spend time on it.
- **Security problems:** do not open an issue. Follow
  [SECURITY.md](SECURITY.md).

## Development setup

You need Docker with the Compose plugin. To run tests outside Docker you also
need Go 1.27 or later and Node 22 or later.

```bash
cd infra
docker compose -f docker-compose.dev.yml up --build
```

The frontend is served on `http://localhost:5173` with hot reload and the API
on `http://localhost:8080`.

## Tests

Backend:

```bash
cd backend
go vet ./...
go test ./...
go test -tags=integration -timeout 5m ./...   # real Redis through testcontainers, needs Docker
```

Frontend:

```bash
cd frontend
npm ci
npm test
npm run build
```

CI runs all of the above, plus `govulncheck`, `npm audit` and Trivy scans of
both container images. HIGH and CRITICAL findings fail the build.

## Where things live

```
backend/
  cmd/api/                 entry point
  internal/domain/         secret types and rules
  internal/ports/          repository interface
  internal/app/            use cases
  internal/adapters/http/  routing, validation, middleware, rate limiting
  internal/adapters/redis/ storage and the atomic consume script
  internal/config/         environment parsing
frontend/
  src/api/crypto.ts        all encryption, decryption and key derivation
  src/api/client.ts        API calls
  src/views/               create, open and info pages
infra/                     compose files and proxy configuration
docs/                      threat model, hardening, runbook, roadmap
```

## Ground rules

- The server must never receive plaintext or keys. A change that sends more to
  the server than today needs an issue and a discussion first.
- Never log secret payloads, keys, passwords or URL fragments.
- Changes to `crypto.ts`, to validation or to the consume path come with
  tests.
- New dependencies need a reason. A small dependency tree is part of the
  security posture.
- Run `gofmt` on Go code. TypeScript and Vue use two-space indentation; see
  `.editorconfig`.
- When behaviour or configuration changes, update the README, the relevant
  file in `docs/` and both `.env.example` files in the same pull request.

## Commits and branches

Commits follow [Conventional Commits](https://www.conventionalcommits.org/)
with a scope:

```
feat(security): reject PBKDF2 for new secrets in password-only mode
test(http): add CORS preflight coverage
fix(docker): correct trusted proxy CIDR
```

Branches are named after the same types: `feat/...`, `fix/...`, `docs/...`,
`test/...`, `chore/...`.

## License

By contributing you agree that your contribution is licensed under the
[MIT License](LICENSE).
