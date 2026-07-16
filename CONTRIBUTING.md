# Contributing to dcloud

Thanks for taking the time to contribute! This document walks through
how to get a working development environment, what we expect from
patches, and how the review flow works.

## Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md).
By participating, you are expected to uphold it. Please report
unacceptable behaviour to the maintainers (see the CoC for the current
address).

## Getting help

- **Bug reports and feature requests** → open a GitHub Issue with the
  provided template.
- **Security issues** → follow [SECURITY.md](SECURITY.md); do not open
  a public issue.
- **Questions and design discussion** → GitHub Discussions.

## Development environment

### Prerequisites

- Go 1.25+
- Node 22+ (for the console)
- [buf](https://buf.build/) for protobuf codegen
- [sqlc](https://sqlc.dev/) for SQL bindings
- Docker or an equivalent OCI runtime
- Access to a Kubernetes cluster with the platform dependencies (see
  [README](README.md#requirements)) if you want to exercise the
  container / compute / storage / database services end-to-end. A
  [kind](https://kind.sigs.k8s.io/) cluster is enough for the console,
  identity and project services — see
  [docs/getting-started.md](docs/getting-started.md).

### Building

```bash
make build       # every Go binary
make proto       # regenerate proto Go + Connect + Python bindings
make sqlc        # regenerate SQL bindings
make test        # go test ./...
```

The console lives in `console/`. Use `npm ci && npm run dev` there for
a hot-reload SPA served on `localhost:5173` (proxy `/api/v1/*` to your
running services via `vite.config.ts`).

## Making a change

1. Fork the repository and create a feature branch off `main`.
2. Keep pull requests focused — one bug or one feature per PR is
   easier to review.
3. Add or update tests when you change behaviour. `make test` must
   pass.
4. Run `go vet ./...` and format the code (`gofmt -s`). The CI job
   verifies both.
5. Update the docs (`README.md`, `docs/`, chart values comments) when
   you introduce or rename a public knob.

### Commit messages

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <short summary>

<optional body>

<optional footer>
```

Types we use: `feat`, `fix`, `refactor`, `docs`, `chore`, `test`, `ci`.
The scope is the component name (`identity`, `project`, `container`,
`compute`, `storage`, `database`, `console`, `chart`, `ci`).

### Sign your commits (DCO)

Every commit must include a `Signed-off-by` line to certify that you
have the right to submit the code under the project's licence (see the
[Developer Certificate of Origin](https://developercertificate.org/)).
`git commit -s` adds the trailer automatically:

```
Signed-off-by: Your Name <your.email@example.com>
```

## Submitting a pull request

- Push your branch to your fork and open a PR against `main`.
- Fill out the PR template — the reviewer will look at the checklist
  before diving into the diff.
- Keep the branch up to date with `main` by rebasing, not merging.
- CI must be green before merging.

Reviewers try to respond within a few days. Small nits are usually
inlined; larger design conversations happen on the PR thread or in a
follow-up discussion.

## Release process

Releases follow [semver](https://semver.org/). Maintainers cut releases
by tagging `main` with `vX.Y.Z`; the CI publishes container images to
`ghcr.io/<owner>/<component>:vX.Y.Z` and pushes the Helm chart to
`oci://ghcr.io/<owner>/charts/dcloud`.

## Licence of contributions

By contributing you agree that your contributions will be licensed
under the [Apache License 2.0](LICENSE) that covers the rest of the
project.
