# 運用手順書

Helm チャートで自動化されていない手動設定をまとめたドキュメント。

---

## 1. Cloudflare Tunnel

### トンネルの作成

Cloudflare Zero Trust → Networks → Tunnels → Create a tunnel

作成後、トンネルトークンを Kubernetes Secret として保存する。

```bash
kubectl create secret generic cloudflared-tunnel-token \
  -n cloudflare \
  --from-literal=token=<トンネルトークン>
```

### Ingress ルールの設定（必須）

Cloudflare Zero Trust → Tunnels → トンネルを選択 → Edit → Public Hostname タブ

以下の順番で設定する（順序重要）：

| # | Hostname | Service | noTLSVerify |
|---|----------|---------|-------------|
| 1 | `argo.daigo-suhara.com` | `https://172.16.100.10` | ✓ |
| 2 | `cloud.daigo-suhara.com` | `http://172.16.100.11:8080` | |
| 3 | `*.drkatana.com` | `http://istio-ingressgateway.istio-system.svc.cluster.local:80` | |
| catch-all | （空欄） | `http://istio-ingressgateway.istio-system.svc.cluster.local:80` | |

**catch-all を Istio IngressGateway に設定することで、ユーザーのカスタムドメインが自動的にルーティングされる。**

IP アドレスが変わった場合は MetalLB の割り当てを確認して更新する。

```bash
kubectl get svc -A | grep "172.16.100"
```

---

## 2. DNS（drkatana.com）

Cloudflare DNS → drkatana.com ゾーン

| 種別 | 名前 | 内容 |
|------|------|------|
| CNAME | `*.drkatana.com` | トンネルのドメイン（`<tunnel-id>.cfargotunnel.com`） |

Cloudflare Tunnel を使う場合、トンネル側で DNS レコードを自動作成するため通常は手動設定不要。

---

## 3. Helm デプロイ前の Secret 作成

```bash
# データベースパスワード
kubectl create secret generic dcloud-database \
  -n dcloud-system \
  --from-literal=password=<DBパスワード> \
  --from-literal=postgres-password=<postgresパスワード> \
  --from-literal=repmgr-password=<repmgrパスワード> \
  --from-literal=sr-check-password=<srcheckパスワード> \
  --from-literal=admin-password=<adminパスワード> \
  --from-literal=DCLD_DATABASE_URL="postgresql://dcloud:<DBパスワード>@dcloud-postgresql-ha-pgpool:5432/dcloud?sslmode=disable" \
  --from-literal=DCLD_DATABASE_MIGRATION_URL="postgresql://dcloud:<DBパスワード>@dcloud-postgresql-ha-postgresql:5432/dcloud?sslmode=disable"
```

---

## 4. Helm デプロイ

```bash
helm upgrade --install dcloud ./charts/dcloud \
  -n dcloud-system \
  --create-namespace
```

### ArgoCD で管理している場合

ArgoCD アプリケーションの Sync ボタンを押すか：

```bash
argocd app sync dcloud
```

---

## 5. カスタムドメインの追加（ユーザー向け手順）

1. dcloud コンソールでサービスの「カスタムドメイン」欄にドメインを入力して設定
2. DNS プロバイダーで CNAME レコードを追加：
   - **名前**: `hello`（サブドメイン部分）
   - **種別**: CNAME
   - **値**: `<サービス名>.drkatana.com`（コンソールに表示される値）
3. DNS が伝播すれば自動的にアクセス可能になる（Cloudflare Tunnel の catch-all ルールが受け取る）

> Cloudflare を使う場合、プロキシ（オレンジ雲）をオンにして SSL モードを **Full** に設定する。

---

## 6. ノードへの SSH

```bash
# 管理クラスター経由
ssh mgmt

# ワークロードクラスターの kubeconfig 取得
kubectl get secret cluster-kubeconfig -n tinkerbell \
  -o jsonpath="{.data.value}" | base64 -d > ~/.kube/wl-config

export KUBECONFIG=~/.kube/wl-config
```
