# Threat Model

What Ephemeral protects, against whom, and where the protection ends. Read it
before you decide what to send through an instance, and before you run one.

Ephemeral has not had an independent security audit.

## What is being protected

| Asset | Sensitivity |
| --- | --- |
| The secret itself: the text or the file content | Highest. Never leaves the browser unencrypted |
| The key, or the password the key is derived from | Highest. Never sent to the server |
| File name, MIME type and size | Lower, but confidential. The server sees them |
| That a secret exists, and when it was created and opened | Lower. The server sees it |
| Availability of the service and of secrets not yet opened | Best effort |

## How a secret travels

1. The sender's browser encrypts with AES-256-GCM and a random 96-bit IV. The
   key is either 256 random bits, or derived from a password with Argon2id
   (3 passes, 64 MiB, one lane, 16-byte random salt).
2. The browser sends the ciphertext and a metadata record to the API, which
   stores both in Redis under a random id, 192 bits by default.
3. The sender passes on a link. With a random key the link is
   `/s/<id>#<key>`. Browsers do not send the part after `#` to any server.
   With a password the link is `/s/<id>` and the password travels separately.
4. The recipient opens the link and presses a button. Only then does the
   browser ask the API for the ciphertext, which counts as a view. A Redis
   script returns the ciphertext and removes it in the same atomic step when
   the last view is spent.
5. The recipient's browser decrypts.

## Who sees what

| | Plaintext | Key or password | Ciphertext | Id | File name, type, size | Client address |
| --- | --- | --- | --- | --- | --- | --- |
| Sender and recipient | yes | yes | yes | yes | yes | their own |
| The API and Redis | no | no | yes | yes | yes | yes |
| The reverse proxy and the web server | no | no | in transit | yes | in transit | yes |
| A network observer (TLS intact) | no | no | no | no | no | yes |
| Whoever holds the link, random key | yes | yes | yes | yes | yes | |
| Whoever holds the link, password mode | no | no | yes, by using a view | yes | yes | |

## Adversaries

**A passive network observer.** TLS between browser and proxy hides
everything but the fact that someone talks to the instance. The shipped proxy
configuration sets HSTS.

**An attacker probing the public endpoints.** Ids are random, 192 bits by
default, and cannot be guessed or enumerated. Requests are validated strictly: unknown JSON
fields are rejected, the metadata has a fixed schema, payload sizes are capped,
KDF parameters must be within fixed ranges and files must match the operator's
size limit and MIME allowlist. The endpoints that read or write secrets are
rate limited per client address, and IPv6 clients per /64. Browsers are only
allowed to call the API from the configured origins.

**Someone who reads the server's storage or logs.** Redis holds ciphertext and
metadata, never keys. Redis runs without persistence, and the API refuses to
start against a Redis that writes to disk, so a stolen disk or backup holds no
secrets. The API log has the request id, method, route template, status,
duration and client address. It never has payloads, keys or ids, and paths
that match no route are not written out. The web server keeps no access log.

**Two people opening the same link.** Reading and counting a view is one
atomic Redis script. Under contention, a secret with one view is delivered
exactly once. An integration test runs this against a real Redis.

**Link previews and mail scanners.** Messaging apps and mail filters fetch
links to build previews or scan them. Fetching the link only loads the app.
The secret is used up when a person presses the button, so automated fetches
do not consume it.

**Cross-site scripting.** The content security policy allows scripts, styles
and fonts from the instance itself and nothing else. There is no inline
script and no third-party request. Decrypted text is placed in a read-only
text field, never rendered as HTML. Decrypted files are offered as downloads
and are not opened in the page.

## What Ephemeral does not protect against

**A malicious or compromised server.** The browser runs the JavaScript the
server sends. Whoever controls the server can send code that leaks the key.
This holds for every web application that encrypts in the browser. Only use an
instance run by someone you trust, or run your own.

**The operator, in password mode with a weak password.** The operator holds
the ciphertext and the salt and can try passwords offline. Argon2id makes each
guess expensive. It does not rescue a short or reused password. The same goes
for anyone who gets hold of a password-mode link: they can fetch the
ciphertext by using a view and then guess offline.

**Metadata changed by the server.** File name, MIME type and size are stored
next to the ciphertext and are not covered by the authentication tag. A
malicious server can change the name or type the recipient sees. It cannot
change the content without decryption failing.

**The link falling into the wrong hands.** A link with the key in it is the
secret. It stays in the sender's sent messages and in the recipient's browser
history. Whoever opens it first gets the content, and the intended recipient
then sees that it is gone, which at least makes the leak visible. Send the
link and a password through different channels when that matters.

**Compromised devices and browsers.** Malware and browser extensions on the
sender's or the recipient's device can read the secret before encryption or
after decryption.

**What the recipient does afterwards.** Once decrypted, a secret can be
copied, saved and forwarded.

**Knowing who sent it.** Anyone who can reach an instance can create a secret.
The recipient has no proof of who created it.

**Denial of service.** Rate limiting slows a single client down. It does not
stop a distributed flood, and each API process keeps its own counters.

## Properties worth knowing

- **A wrong password costs a view.** The ciphertext is fetched before the
  password is tried, so a mistyped password on a one-view secret destroys it.
  That is deliberate: guessing passwords against the server is not possible.
  Give a secret two or three views if typos are likely.
- **A view-limited secret has no time limit.** It stays in Redis until it is
  opened or Redis restarts. A secret with a time limit can be opened any number
  of times until it expires.
- **Unopened secrets are lost when Redis restarts.** Nothing is written to
  disk. A host reboot or a recreated Redis container empties the store.
- **Redis memory is not capped by the shipped configuration.** Every unopened
  secret occupies memory until it is opened or expires. On a public instance,
  set `MAX_FILE_BYTES` and the rate limits with the host's memory in mind.
- **Ids appear wherever URLs are logged.** The key never does, but the id is
  part of the path. The shipped web server and API keep it out of their logs.
  If you put another proxy, a CDN or a WAF in front, check what it logs: an id
  is enough to use up a secret.
- **Client addresses are logged by the API.** They are needed for rate
  limiting and for investigating abuse. Decide how long you keep the logs.
- **`/api/metrics` has no authentication.** It exposes four counters and is
  off by default. If you turn it on, restrict who can reach it at the proxy.
- **Old links keep working.** Secrets protected with PBKDF2 by earlier
  versions can still be opened. New secrets always use Argon2id, and
  password-only mode rejects PBKDF2 on upload.

## Supply chain

- Dependencies are pinned through `go.sum` and `package-lock.json`. The
  frontend has three direct runtime dependencies, the API one.
- CI runs `govulncheck`, `npm audit` and Trivy scans of both images, and fails
  on HIGH and CRITICAL findings. Dependabot proposes updates weekly.
- The API image is distroless and runs as a non-root user.
- Third-party GitHub Actions are pinned to commit SHAs.
- A known-answer test pins the Argon2id output, so a library update that
  changes key derivation fails the build before it can break existing links.

## Reporting a problem

See [SECURITY.md](../SECURITY.md).
