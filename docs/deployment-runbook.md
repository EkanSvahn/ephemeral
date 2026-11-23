# Deployment Runbook

A simple production flow for Ephemeral on one Ubuntu host, physical or a cloud
VM, with Docker Compose.

## 1) Prerequisites

- Ubuntu host with Docker Engine and the Docker Compose plugin.
- DNS for your domain pointing at the host.
- Inbound firewall open for ports `80` and `443` only. SSH restricted to
  trusted addresses.
- Host updates applied regularly.

## 2) Configuration

Everything that is specific to a deployment lives in two files on the host.
Both are gitignored, so the repository and the host never disagree about a
tracked file and `git pull` never stops on a local edit.

1. Create them from the templates:
   ```bash
   cp infra/.env.example infra/.env
   cp infra/Caddyfile.prod.example infra/Caddyfile.prod
   ```
2. Edit `infra/.env`:
   - `ALLOWED_ORIGINS=https://your-domain` is required and has no fallback
   - `TRUSTED_PROXY_CIDR` must cover the Docker network the proxy is on, or
     every visitor shares one rate limit
   - `REQUIRE_PASSWORD=true` for deployments inside an organization
   - adjust `MAX_FILE_BYTES`, `ALLOWED_FILE_MIME_TYPES` and the rest as needed
3. Edit `infra/Caddyfile.prod`: replace the example domain. If the same proxy
   serves other sites, add their site blocks here.
4. Keep Redis on the private compose network. The production compose file
   publishes no Redis port. Leave it that way.

## 3) Deploy

From the repository root, on the tag you want to run:

```bash
git fetch --tags
git checkout vX.Y.Z
cd infra
docker compose -f docker-compose.prod.yml up -d --build
```

Check the result:

```bash
curl -fsS https://<your-domain>/api/health   # liveness: the process is up
curl -fsS https://<your-domain>/api/ready    # readiness: Redis is reachable
curl -fsS https://<your-domain>/api/status   # the policy this deployment enforces
```

Point load balancers and health probes at `/api/ready`. Use `/api/health` only
for liveness checks that restart the container.

The version badge in the bottom left corner of the UI shows what is running.

## 4) Update

An update is the same as a deploy: check out the new tag and run
`docker compose ... up -d --build`. Compose rebuilds the API and the frontend
and leaves containers alone whose configuration and image did not change.

Two things to know:

- **Secrets that have not been opened yet live only in Redis memory.** They
  survive an update as long as the Redis container keeps running. They are lost
  when it is recreated or restarted, for example after
  `docker compose pull` fetched a newer Redis image, or after a host reboot.
  That is by design. Tell your users before you do either.
- **`docker compose pull` refreshes the Redis and Caddy base images.** Run it
  on purpose, when you want those patches, and not as part of every update.

### Coming from 0.1.x

Up to 0.1.6 the proxy configuration was a tracked file, `infra/Caddyfile.prod`,
that had to be edited in place. From 0.2.0 it is a gitignored copy of
`infra/Caddyfile.prod.example`. A deployment with an edited `Caddyfile.prod`
needs these steps once, in place of the plain checkout:

```bash
cp infra/Caddyfile.prod ~/Caddyfile.prod.keep    # your live proxy configuration
git checkout -- infra/Caddyfile.prod             # drop the local edit so git can update
git fetch --tags
git checkout v0.2.0
cp ~/Caddyfile.prod.keep infra/Caddyfile.prod    # same path, now ignored by git
```

Run them in one go. The proxy container keeps running throughout with the
configuration it started with, and the file it finds on its next start has the
same content as before.

## 5) Rollback

If a release causes problems, go back to the last tag that worked.
`git tag --sort=-v:refname` lists the tags, newest first.

```bash
git fetch --tags
git checkout vX.Y.Z   # the last known good tag
cd infra
docker compose -f docker-compose.prod.yml up -d --build
```

Then repeat the checks from section 3.

**Rolling back from 0.2.0 or later to 0.1.x replaces your proxy
configuration.** In 0.1.x `infra/Caddyfile.prod` is a tracked file, and
because your copy is ignored, git overwrites it with the example without
asking. Copy it aside first and put it back after the checkout:

```bash
cp infra/Caddyfile.prod ~/Caddyfile.prod.keep
git checkout v0.1.6
cp ~/Caddyfile.prod.keep infra/Caddyfile.prod
```

The running proxy is not affected until it restarts, so check the file before
you move on.

## 6) Release procedure (`vX.Y.Z`)

1. Make sure CI on `main` is green.
2. Set the new version in `frontend/package.json` and
   `frontend/package-lock.json`, and move the entries under "Unreleased" in
   `CHANGELOG.md` to a section for the version.
3. Merge that as a release pull request.
4. Create the tag and the GitHub release from `main`.
5. Deploy from the tag and check the version badge, `/api/ready` and
   `/api/status`.

The UI takes its version from `frontend/package.json` when the image is built
with Docker. CI fails a tag build whose tag does not match that version.
