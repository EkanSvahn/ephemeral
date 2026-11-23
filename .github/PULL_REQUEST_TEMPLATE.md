## What and why

<!-- What does this change, and what problem does it solve? Link the issue if there is one. -->

## How it was tested

<!-- Commands you ran and what you checked by hand. -->

## Checklist

- [ ] `go vet ./...` and `go test ./...` pass in `backend/`
- [ ] `npm test` and `npm run build` pass in `frontend/`
- [ ] Changes to `crypto.ts`, validation or the consume path come with tests
- [ ] Nothing new is sent to the server, and nothing sensitive is logged
- [ ] README, `docs/` and both `.env.example` files are updated when behaviour or configuration changed
