# Security Checklist

For whoever runs an instance. The first list is what the shipped
configuration already does. The second is what only you can do.

## Handled by the shipped configuration

- [x] TLS at the reverse proxy, with HSTS.
- [x] Security headers and a content security policy on every response.
- [x] Redis on the private compose network, with no published port and no
      persistence. The API refuses to start if Redis writes to disk.
- [x] Rate limiting per client address on the endpoints that read or write
      secrets.
- [x] Strict request validation, payload caps and a MIME allowlist.
- [x] Logs without payloads, keys or secret ids.
- [x] `/api/metrics` off by default.
- [x] Dependency and container image scanning in CI.
- [x] A disclosure policy (`SECURITY.md`).

## For you to do before exposing an instance

- [ ] Set `ALLOWED_ORIGINS` to the origin the site is served from.
- [ ] Check that `TRUSTED_PROXY_CIDR` covers the Docker network the proxy is
      on. Without it, all visitors share one rate limit.
- [ ] Decide on `REQUIRE_PASSWORD`. `true` is the safer choice when everyone
      using the instance belongs to one organization.
- [ ] Set `MAX_FILE_BYTES` and `ALLOWED_FILE_MIME_TYPES` to what you are
      willing to store and pass on. Every unopened secret occupies memory.
- [ ] Open only ports `80` and `443` in the firewall, and restrict SSH.
- [ ] If anything sits in front of the shipped proxy (another proxy, a CDN, a
      WAF), check that it does not log request paths. Share links contain the
      secret id.
- [ ] Decide how long logs are kept. API logs contain client addresses.
- [ ] Read the [threat model](threat-model.md) and tell your users what the
      instance does and does not protect.

## Worth adding

- [ ] Alerting on error rates, on the `ephemeral_rate_limited_total` counter
      and on certificate expiry.
- [ ] A `security.txt` for your deployment, with a contact address of your
      own.
- [ ] A reminder to update: watch the repository's releases and rebuild when
      one comes out.
