# dcloud

A self-hosted, GCP-shaped platform for Kubernetes.

dcloud is an open-source control plane that gives your Kubernetes
cluster a small set of familiar cloud primitives — **container**
(serverless functions on Knative), **compute** (VMs on KubeVirt),
**storage** (S3-compatible buckets on Rook Ceph RGW),
**database** (managed instances on KubeBlocks) — plus **projects**
that tie resources together and **identity** for authenticating end
users. Everything is exposed through a browser console and a REST /
ConnectRPC / gRPC API served by the services themselves.

## Table of contents

- [Architecture](#architecture)
- [Repository layout](#repository-layout)
- [Requirements](#requirements)
- [Getting started](#getting-started)
- [Configuration](#configuration)
- [Development](#development)
- [Contributing](#contributing)
- [License](#license)

## Architecture

Each domain is a standalone Go binary that speaks three protocols on
two ports:

| Service   | gRPC | HTTP (h2c) | Backing system |
|-----------|-----:|-----------:|----------------|
| identity  | 8083 | 8093       | PostgreSQL + Ed25519 JWTs |
| project   | 8081 | 8091       | PostgreSQL |
| container | 8082 | 8092       | Knative Serving |
| compute   | 8084 | 8094       | KubeVirt |
| storage   | 8085 | 8095       | Rook Ceph RGW (ObjectBucketClaim) |
| database  | 8086 | 8096       | KubeBlocks (Postgres / MySQL / Redis) |

The HTTP port serves three things behind one h2c listener:

- **ConnectRPC** — `/dcloud.<pkg>.v1.<Service>/<Method>` for typed clients
- **REST** — `/api/v1/…` for the console and any REST client
- **Extras** — JWKS on identity, a WebSocket VM console proxy on compute,
  an S3 object-level proxy on storage

Authentication is stateless: identity signs Ed25519 JWTs, exposes them
as JWKS on `/.well-known/jwks.json`, and every other service validates
them locally with a shared verifier (`internal/auth/jwtverify`).

The console is a Vite + React SPA served by nginx; the chart mounts a
routing ConfigMap that fans `/api/v1/*` out to the right service.

Detailed component notes live in [docs/components.md](docs/components.md).

## Repository layout

```
dcloud/
├── proto/                        # gRPC service definitions
├── internal/
│   ├── auth/jwtverify/           # shared Ed25519 JWT verifier
│   ├── apihelp/                  # shared REST helpers
│   ├── db/sqlc/                  # shared SQL bindings (generated)
│   ├── pb/                       # proto-generated Go + Connect clients
│   ├── {identity,project,container,compute,storage,database}/
│   │   ├── cmd/server/           # thin composition root (main)
│   │   ├── service/              # RPC implementations
│   │   ├── handler/              # gRPC + Connect + REST bridges
│   │   ├── domain/               # DTOs
│   │   └── repository/           # external system clients
│   └── identity/keys/            # JWT signing key material
├── console/                      # React + Vite SPA
├── charts/dcloud/                # Helm chart
└── docs/
```

## Requirements

dcloud runs on top of the following cluster components (bring your own,
or install them from the ArgoCD manifests in
[gitops-metal3](https://github.com/daigo-suhara/gitops-metal3)):

- Kubernetes 1.29 or newer
- [Rook Ceph](https://rook.io/) with a `CephBlockPool` and a
  `CephObjectStore` (block storage for PVCs, RGW for buckets)
- [Istio](https://istio.io/) 1.24+ (Knative net-istio target)
- [Knative Serving](https://knative.dev/) 1.16+
- [KubeVirt](https://kubevirt.io/) 1.4+
- [KubeBlocks](https://kubeblocks.io/) 1.0+
- [cloudflared](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/)
  or another public ingress

For local hacking a `kind` cluster works for the console, identity and
project services; container/compute/storage/database require their
external systems to be installed to be exercised end-to-end.

## Getting started

### 1. Provision the platform prerequisites

Follow the operations guide in [`docs/operations.md`](docs/operations.md).

### 2. Create the database secret

```bash
kubectl create secret generic dcloud-database -n dcloud-system \
  --from-literal=password=<DB_PASSWORD> \
  --from-literal=postgres-password=<POSTGRES_PASSWORD> \
  --from-literal=repmgr-password=<REPMGR_PASSWORD> \
  --from-literal=sr-check-password=<SR_CHECK_PASSWORD> \
  --from-literal=admin-password=<ADMIN_PASSWORD> \
  --from-literal=DCLD_DATABASE_URL="postgresql://dcloud:<DB_PASSWORD>@dcloud-postgresql-ha-pgpool:5432/dcloud?sslmode=disable" \
  --from-literal=DCLD_DATABASE_MIGRATION_URL="postgresql://dcloud:<DB_PASSWORD>@dcloud-postgresql-ha-postgresql:5432/dcloud?sslmode=disable"
```

### 3. Install the chart

```bash
helm upgrade --install dcloud ./charts/dcloud \
  -n dcloud-system --create-namespace \
  --set publicServiceDomain=example.com \
  --set console.publicHostname=cloud.example.com
```

The console is then reachable at
`https://<console.publicHostname>` once your public ingress (Cloudflare
Tunnel, LoadBalancer, etc.) is wired up.

## Configuration

Every knob lives in [`charts/dcloud/values.yaml`](charts/dcloud/values.yaml).
The most common overrides:

| Key | Default | Purpose |
|-----|---------|---------|
| `image.registry` / `image.repositoryOwner` / `image.tag` | `ghcr.io/dcloud/*:latest` | Where to pull the component images from. |
| `publicServiceDomain` | `example.com` | Zone tenant `container` services are exposed under (`<svc>.<zone>`). |
| `console.publicHostname` | `cloud.example.com` | Hostname the console itself is served at. |
| `storageClass.className` | `ceph-rbd` | Storage class dcloud creates for PostgreSQL + KubeBlocks. |
| `storage.rgwEndpoint` | *empty* | Ceph RGW endpoint the storage service returns to bucket clients. |
| `database.storageClass` | `ceph-rbd` | Storage class the database service asks KubeBlocks to use. |

## Development

```bash
# Build every Go binary
make build

# Regenerate proto (Go + Connect + Python) after touching proto/*.proto
make proto

# Regenerate sqlc bindings after touching internal/db/sqlc/*.sql
make sqlc

# Run tests
make test
```

Prerequisites: Go 1.25, Node 22 (for the console), `buf` and `sqlc`.

## Contributing

Contributions are welcome. Please read
[CONTRIBUTING.md](CONTRIBUTING.md) (coming soon) for guidelines and
[SECURITY.md](SECURITY.md) (coming soon) if you would like to report a
security issue privately.

## License

dcloud is licensed under the [Apache License 2.0](LICENSE).
