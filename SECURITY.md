# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in this project, please report it
responsibly via **GitHub Security Advisories** (preferred) or by emailing
**thomastyzer@outlook.fr**.

**Do not** open a public issue for security vulnerabilities.

When reporting, please include:
- A clear description of the vulnerability
- Steps to reproduce the issue
- Your assessment of the potential impact

## Response Timeline

- **Acknowledgment**: within 3 business days
- **Investigation**: within 7 business days
- **Fix**: as soon as practical after confirmation

## Scope

This CLI reads data from a personal BoursoBank account via session cookies
extracted from the local Chrome browser. Security-relevant areas include:

- Cookie and bearer token handling (extraction, storage, transmission)
- Config file permissions (must remain 0600)
- HTTP transport (TLS, redirect allowlist, no cookie exfiltration)
- Temporary file cleanup (no secrets left on disk)

## Supported Versions

Only the latest release is supported with security updates.
