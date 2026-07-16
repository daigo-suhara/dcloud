# Operations Guide

Manual steps that are not automated by the Helm chart.

Throughout this document `example.com` is used as the placeholder for
your public DNS zone (matches the default `publicServiceDomain` in
`values.yaml`); replace it with your own domain in every command and
config example.

---

## 1. Cloudflare Tunnel

### Create a tunnel

Cloudflare Zero Trust → Networks → Tunnels → **Create a tunnel**

Store the tunnel token as a Kubernetes Secret:

```bash
kubectl create secret generic cloudflared-tunnel-token \
  -n cloudflare \
  --from-literal=token=<TUNNEL_TOKEN>
```

### Public hostname rules

Cloudflare Zero Trust → Tunnels → *your tunnel* → Edit → **Public Hostname**

Register these routes in order (order matters — the catch-all must be last):

| # | Hostname | Service | noTLSVerify |
|---|----------|---------|-------------|
| 1 | `argo.example.com` | `https://<ArgoCD LoadBalancer IP>` | ✓ |
| 2 | `cloud.example.com` | `http://<dcloud console LoadBalancer IP>:8080` | |
| 3 | `*.example.com` | `http://istio-ingressgateway.istio-system.svc.cluster.local:80` | |
| catch-all | (empty) | `http://istio-ingressgateway.istio-system.svc.cluster.local:80` | |

The `*.example.com` route (and catch-all) sends tenant custom-domain
traffic through Istio's ingress gateway, where Knative reconciles the
per-service Route and matches it by Host header.

Check the LoadBalancer allocations when they change:

```bash
kubectl get svc -A
```

---

## 2. DNS (example.com zone)

Cloudflare DNS → *example.com* zone

| Type | Name | Value |
|------|------|-------|
| CNAME | `*.example.com` | tunnel domain (`<tunnel-id>.cfargotunnel.com`) |

When you use a Cloudflare Tunnel with the wildcard hostname above,
Cloudflare provisions the CNAME automatically; a manual entry is rarely
required.

---

## 3. Secrets required before `helm install`

```bash
# Database passwords consumed by the PostgreSQL HA sub-chart and by
# each dcloud service via DCLD_DATABASE_URL / DCLD_DATABASE_MIGRATION_URL.
kubectl create secret generic dcloud-database \
  -n dcloud-system \
  --from-literal=password=<DB_PASSWORD> \
  --from-literal=postgres-password=<POSTGRES_PASSWORD> \
  --from-literal=repmgr-password=<REPMGR_PASSWORD> \
  --from-literal=sr-check-password=<SR_CHECK_PASSWORD> \
  --from-literal=admin-password=<ADMIN_PASSWORD> \
  --from-literal=DCLD_DATABASE_URL="postgresql://dcloud:<DB_PASSWORD>@dcloud-postgresql-ha-pgpool:5432/dcloud?sslmode=disable" \
  --from-literal=DCLD_DATABASE_MIGRATION_URL="postgresql://dcloud:<DB_PASSWORD>@dcloud-postgresql-ha-postgresql:5432/dcloud?sslmode=disable"
```

---

## 4. Helm install

```bash
helm upgrade --install dcloud ./charts/dcloud \
  -n dcloud-system \
  --create-namespace \
  --set publicServiceDomain=example.com \
  --set console.publicHostname=cloud.example.com
```

### Under ArgoCD

Trigger a sync in the ArgoCD UI, or:

```bash
argocd app sync dcloud
```

---

## 5. Adding a custom domain (end-user flow)

1. In the dcloud console, open the service's **Custom domain** field and
   enter the domain you own (for example `hello.your-domain.com`).
2. At your DNS provider, add a CNAME record:
   - **Name**: `hello` (the subdomain)
   - **Type**: CNAME
   - **Value**: `<service-name>.example.com` (shown by the console)
3. Once DNS propagates the service is reachable — the Cloudflare Tunnel
   catch-all forwards the request to Istio's ingress gateway, and Knative
   matches the Host header to the Route.

> If you are using Cloudflare, enable the orange-cloud proxy and set the
> zone's SSL mode to **Full**.

---

## 6. Accessing cluster nodes

```bash
ssh <mgmt-node>

# Grab the workload kubeconfig (adjust namespace to your cluster manager)
kubectl get secret <cluster>-kubeconfig -n <cluster-mgmt-ns> \
  -o jsonpath="{.data.value}" | base64 -d > ~/.kube/wl-config

export KUBECONFIG=~/.kube/wl-config
```
