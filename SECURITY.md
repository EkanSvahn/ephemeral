# Security Policy

Ephemeral exists to move secrets from one person to another, so security
reports go ahead of everything else.

## Supported versions

Security fixes land on `main` and in the latest tagged release. Older tags are
not patched.

## Reporting a vulnerability

Please do not open a public issue for a security problem.

Use GitHub's private vulnerability reporting instead: open the repository's
**Security** tab and choose **Report a vulnerability**. The report is visible
only to the maintainer.

It helps if the report includes:

- the commit or tag you tested
- the relevant configuration (for example `REQUIRE_PASSWORD`,
  `TRUSTED_PROXY_CIDR`, the reverse proxy in front)
- steps to reproduce, or a proof of concept
- what an attacker gains

Ephemeral is maintained by one person in their spare time. You can expect an
acknowledgement within a week and an honest estimate of when a fix can ship.
You will be credited in the release notes unless you ask not to be.

## Scope

In scope:

- anything that lets the server, the operator or a network observer recover
  plaintext or keys in the default flow
- reading a secret more times, or for longer, than its policy allows
- weaknesses in how the browser encrypts, derives keys or handles the link
  fragment
- bypassing server-side validation: size limits, MIME allowlist, metadata
  schema, KDF parameter floors
- bypassing rate limiting, for example through spoofed forwarding headers
- XSS or injection in the frontend
- insecure defaults in the compose files, Dockerfiles or proxy configuration
  shipped in this repository

Out of scope:

- compromised sender or recipient devices, browsers or browser extensions
- weak or reused passwords chosen by users
- links or passwords leaked through the channel used to send them
- deployments that ignore the documented baseline (no TLS, public Redis,
  permissive CORS)
- denial of service through sheer request volume
- scanner output without a demonstrated impact

The limits of what Ephemeral protects against are described in
[docs/threat-model.md](docs/threat-model.md).

## Instances run by others

Each instance is the responsibility of whoever operates it. If the problem is
specific to one deployment and not to the code here, report it to that
operator.
