# dcloud

## 構成

```text
dcloud/
├── buf.yaml
├── buf.gen.yaml
├── protos/
│   ├── identity.proto
│   ├── project.proto
│   └── container.proto
├── console/
├── api/
│   ├── app/
│   └── generated/
├── internal/
│   ├── pb/
│   ├── project/
│   └── container/
└── charts/
```

## 役割

- `protos/`: 全サービス共通の gRPC API 定義
- `console`: React + Vite の UI
- `api/`: FastAPI の HTTP エントリポイント。`identity` に session を問い合わせて認証済みユーザとして扱います
- `internal/pb/`: proto 由来の Go 型定義と service descriptor
- `internal/identity`: username/password と session を管理する gRPC サービス
- `internal/project`: project / platform 系の gRPC サービス
- `internal/container`: container / service 系の gRPC サービス
- component の粒度は [`docs/components.md`](docs/components.md) を参照してください。
- `console` と `api` の対応表は [`docs/api-contract.md`](docs/api-contract.md) を参照してください。

## 状態

このリポジトリは構成を新しいレイアウトへ移行中です。
`internal/project` と `internal/container` は PostgreSQL を共有しながら `internal/pb` の共通型を使う形に寄せています。
`internal/identity` は PostgreSQL にユーザと session を保存し、`api` は cookie を受けて `identity` に問い合わせます。
`api` も同じ PostgreSQL HA クラスタへ接続して、プロジェクトとコンテナの状態を一元管理しています。
PostgreSQL は Bitnami の `postgresql-ha` Helm chart でデプロイしています。
SQL 由来の Go コードは [`internal/db/sqlc/`](internal/db/sqlc/) にあり、`make sqlc` で再生成できます。
gRPC の Go / Python 生成物は `make proto` で再生成できます。
