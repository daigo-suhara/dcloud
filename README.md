# dcp
## 内容
k8sの上にGCPのようなクラウドを作るOSSプロジェクト
## 技術
- Go
- Helm
- React
- oci
## 注意
- DESIGN.mdを参照すること
- actionsでghcrにプッシュする
- マイクロサービスアーキテクチャで実装する．各コンポーネントをociコンテナ化

## 開発スケジュール
1. 基盤を作る
2. webコンソールを作る
3. cloudrunを作る

## 開発コマンド
```sh
make test
make build
cd web/console && npm ci && npm run build
helm template dcp charts/dcp
```

## コンポーネント
- `services/control-plane`: プラットフォーム管理API
- `services/cloudrun`: CloudRun相当サービスAPI
- `web/console`: React製webコンソール
- `charts/dcp`: Kubernetes配信用Helm chart
