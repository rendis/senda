# Security Policy

Security matters. If you believe you found a vulnerability in Senda, please report it **privately**.

## How to report a vulnerability

Please use one of these private channels:

1. **Preferred:** GitHub Private Vulnerability Reporting for this repository, if it is enabled: <https://github.com/rendis/senda/security/advisories/new>
2. **Fallback:** Contact the maintainer privately through <https://github.com/rendis>

Please **do not** open a public issue for security vulnerabilities.

## What to include

A strong report includes:

- A clear description of the vulnerability
- Impact and affected surface
- Reproduction steps or a proof of concept
- Any mitigations or workarounds you identified
- Whether credentials, tokens, or production data were involved

## Expected response timeline

We aim to:

- Acknowledge receipt within **5 business days**
- Provide an initial triage update within **10 business days**
- Coordinate disclosure after a fix or mitigation is available

These are best-effort targets, not guarantees.

## Supported versions

| Version | Supported |
| ------- | --------- |
| `main`  | Yes       |
| Older, unmaintained branches | No |

If the project starts publishing stable release branches or versioned security support windows, this policy will be updated.

## Disclosure policy

Please give maintainers a reasonable opportunity to investigate and remediate the issue before public disclosure.

Until a fix or mitigation is available:

- Do not publish exploit details
- Do not open a public issue
- Do not post proof-of-concept code publicly

## Out of scope

The following are generally out of scope unless they create a practical security impact:

- Requests for general hardening advice without a concrete vulnerability
- Missing best-practice headers or settings that are already intentionally documented
- Denial-of-service claims without a reproducible attack path
- Reports that depend on compromised local development environments

If you're unsure whether something is security-sensitive, report it privately first.
