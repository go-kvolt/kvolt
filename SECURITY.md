# Security Policy

## Supported versions

| Version | Supported |
| ------- | --------- |
| 2.3.x   | Yes |
| 2.2.x   | Yes |
| < 2.2   | Best-effort patches only |

## Reporting a vulnerability

Please report security issues privately — do not open a public GitHub issue for exploitable bugs.

- Email: open a private security advisory on the [go-kvolt/kvolt](https://github.com/go-kvolt/kvolt) repository (GitHub Security Advisories), or contact the maintainers listed in the repo.
- Include: affected version/tag, reproduction steps, and impact.

We aim to acknowledge reports within a few days and ship fixes in a patch release when confirmed.

## Hardening notes

- Prefer JWT `Claim`-based tenant resolution over trusting `X-Tenant-ID` alone.
- Use a strong `JWT` / CSRF `Secret` in production; rotate on compromise.
- Bound idempotency and rate-limit stores (`MaxKeys`); use Redis stores for multi-instance.
- Keep dependencies updated (Dependabot / `govulncheck`).
