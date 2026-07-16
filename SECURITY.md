# Security Policy

## Supported versions

dcloud is a young project. Until a `v1.0.0` release is tagged, only the
latest commit on the `main` branch and the most recent tagged release
receive security fixes.

| Version         | Supported |
|-----------------|-----------|
| `main`          | ✅         |
| Latest release  | ✅         |
| Older releases  | ❌         |

## Reporting a vulnerability

**Please do not report security issues in public GitHub Issues or in
Pull Requests.** Public disclosure before a fix is available puts users
at risk.

Instead, use GitHub's **Private vulnerability reporting**:

1. Open the [Security tab](../../security) of this repository.
2. Click **Report a vulnerability**.
3. Fill out the form. Include reproduction steps, impact, and any
   suggested mitigations.

If you cannot access GitHub's private advisory workflow, email the
maintainers privately — see the address in `.github/CODEOWNERS` if
available, otherwise contact the repository owner directly.

## What to include

To help us triage quickly, include:

- The dcloud version (git commit SHA or chart version).
- The affected component (identity / project / container / compute /
  storage / database / console / helm chart / CI).
- A clear description of the vulnerability.
- Steps to reproduce, including any required configuration.
- The impact (data disclosure, privilege escalation, denial of service,
  supply chain, etc.).
- Any suggested fix or mitigation, if you have one.

## Response process

- **Acknowledgement**: within 3 business days of the report.
- **Triage**: within 7 business days we will confirm whether the report
  is a valid vulnerability and its severity.
- **Fix window**: high/critical severity issues are patched as soon as
  possible; medium/low severity issues are folded into the next regular
  release.
- **Coordinated disclosure**: we will work with you on a disclosure
  timeline. Once a fix is released we publish a GitHub Security
  Advisory that credits the reporter unless they prefer to remain
  anonymous.

## Scope

The following are in scope:

- Authentication / authorization bypasses in any dcloud service.
- Cross-tenant data disclosure or resource access (project isolation).
- JWT signing / verification flaws in `internal/auth/jwtverify` or
  `internal/identity/keys`.
- SQL injection or unauthenticated data access.
- Container escape or privilege escalation via manifests dcloud renders
  for Knative / KubeVirt / KubeBlocks.
- Vulnerabilities in the console (XSS, CSRF, session fixation, etc.).
- Supply-chain issues in the build (`Dockerfile`, `go.mod`,
  `package.json`).

Out of scope:

- Vulnerabilities in third-party dependencies without a proof of exploit
  in the context of dcloud (please report those to the upstream
  project).
- Denial of service that requires cluster-admin credentials.
- Reports generated purely by automated scanners without a reproduction.

Thank you for helping to keep dcloud and its users safe.
