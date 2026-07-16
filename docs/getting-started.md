# Getting started (local kind cluster)

This guide brings up a *minimal* dcloud on a local
[kind](https://kind.sigs.k8s.io/) cluster so you can poke at the
console, identity, and project services without the heavier platform
prerequisites (Knative, KubeVirt, KubeBlocks, Rook Ceph).

The container, compute, storage and database services will fail their
readiness probes because their upstream systems are not installed —
that is expected. See [Enabling the rest](#enabling-the-rest-of-the-services)
for how to layer them on top later.

Everything below assumes `kubectl`, `helm`, and `kind` are on your PATH.

## 1. Create a kind cluster

```bash
kind create cluster --name dcloud
kubectl cluster-info --context kind-dcloud
```

## 2. Install PostgreSQL

dcloud persists project, session, and container metadata in a shared
PostgreSQL HA that ships as a sub-chart of the dcloud chart. For local
kicking-the-tyres a single-node bitnami postgresql install is enough;
override the sub-chart's replica count with `values-local.yaml`.

```yaml
# values-local.yaml
image:
  # Point at the images the CI publishes. Replace <owner> with your fork.
  registry: ghcr.io
  repositoryOwner: <owner>
  tag: latest

publicServiceDomain: local.dcloud.dev
console:
  publicHostname: cloud.local.dcloud.dev
  service:
    type: NodePort

storageClass:
  # kind exposes a `standard` StorageClass out of the box.
  createClass: false
  className: standard

postgresql-ha:
  postgresql:
    replicaCount: 1
  pgpool:
    replicaCount: 1
```

## 3. Create the database secret

```bash
kubectl create namespace dcloud-system

DB_PASSWORD="$(openssl rand -hex 16)"
kubectl create secret generic dcloud-database -n dcloud-system \
  --from-literal=password="${DB_PASSWORD}" \
  --from-literal=postgres-password="$(openssl rand -hex 16)" \
  --from-literal=repmgr-password="$(openssl rand -hex 16)" \
  --from-literal=sr-check-password="$(openssl rand -hex 16)" \
  --from-literal=admin-password="$(openssl rand -hex 16)" \
  --from-literal=DCLD_DATABASE_URL="postgresql://dcloud:${DB_PASSWORD}@dcloud-postgresql-ha-pgpool:5432/dcloud?sslmode=disable" \
  --from-literal=DCLD_DATABASE_MIGRATION_URL="postgresql://dcloud:${DB_PASSWORD}@dcloud-postgresql-ha-postgresql:5432/dcloud?sslmode=disable"
```

## 4. Install the chart

```bash
helm dependency update charts/dcloud
helm upgrade --install dcloud ./charts/dcloud \
  -n dcloud-system \
  --create-namespace \
  --values values-local.yaml
```

Wait for the core pods to become ready. On a laptop this is usually
under two minutes once the images are pulled:

```bash
kubectl -n dcloud-system rollout status deploy/dcloud-identity
kubectl -n dcloud-system rollout status deploy/dcloud-project
kubectl -n dcloud-system rollout status deploy/dcloud-console
```

## 5. Reach the console

`console.service.type=NodePort` in `values-local.yaml` above assigns
a random NodePort; port-forward for a stable URL instead:

```bash
kubectl -n dcloud-system port-forward svc/dcloud-console 8080:8080
```

Open http://localhost:8080 in a browser and register a user. You should
be able to create a project on the *Projects* screen. Anything that
requires the container / compute / storage / database services will
error out until those are enabled.

## 6. Enabling the rest of the services

To exercise the full set of services, install their upstream systems
on the same cluster (or on a bigger cluster). Rough order:

1. **Rook Ceph** with a `CephBlockPool` and a `CephObjectStore` — the
   storage service's `ObjectBucketClaim` calls need RGW; the database
   service asks for PVCs on the block pool.
2. **Istio** (base + istiod + a ClusterIP ingressgateway) — Knative
   Serving's net-istio target.
3. **Knative Operator** and a `KnativeServing` CR — required by
   container.
4. **KubeVirt Operator** and a `KubeVirt` CR — required by compute.
5. **KubeBlocks** — required by database.

Sample ArgoCD manifests for a bare-metal install of all of the above
live at
[gitops-metal3/homelab](https://github.com/daigo-suhara/gitops-metal3/tree/main/homelab).

Once those are up, rerun `helm upgrade` to re-render the chart against
the running platform:

```bash
helm upgrade --install dcloud ./charts/dcloud \
  -n dcloud-system \
  --values values-local.yaml
```

## Troubleshooting

- **Console login returns 401** — JWKS URL misconfigured. Confirm
  `DCLD_IDENTITY_JWKS_URL` on each service defaults to
  `http://dcloud-identity:8093/.well-known/jwks.json`; override in
  `values-local.yaml` if you changed the service name.
- **`create project` returns 502** — the identity → project → other
  services hop expects the ConnectRPC h2c ports to be reachable. Check
  `kubectl -n dcloud-system get svc` for `dcloud-project` service on
  port 8091 and confirm the pod is Ready.
- **`postgresql-ha` install fails on kind** — the sub-chart defaults to
  three replicas; the override in `values-local.yaml` drops it to one.

## Uninstalling

```bash
helm uninstall dcloud -n dcloud-system
kubectl delete namespace dcloud-system
kind delete cluster --name dcloud
```
