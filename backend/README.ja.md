# バックエンド

バックエンドは Go 製の HTTP API です。ビジネスルールは
`internal/application`、REST 境界は `internal/port/rest` に置きます。
PostgreSQL のマイグレーションとクエリソースは `db/` 以下にあります。
実行時設定、構造化ログ、トレース設定はそれぞれ
`internal/config`、`internal/logging`、`internal/telemetry` が担当します。

起動手順は[リポジトリのクイックスタート](../README.ja.md#バックエンドのクイックスタート)を参照してください。
このディレクトリで `make help` を実行すると、利用可能なコマンドを確認できます。
よく使う開発コマンド:

```bash
make test                 # ユニットテスト
make test-integration     # 分離した PostgreSQL 統合テスト（Docker が必要）
make build                # API をビルド
make sqlc-gen             # SQL から sqlc を再生成
make swagger-gen          # OpenAPI ドキュメントを再生成
```

サーバー起動中は <http://localhost:8080/swagger/index.html> で API リファレンスを
確認できます。実装境界は[データベースガイド](db/AGENTS.md)、
[アプリケーションガイド](internal/application/AGENTS.md)、
[REST ガイド](internal/port/rest/AGENTS.md)を参照してください。

## ログとトレース

API は構造化 JSON ログを標準出力に書き込みます。リクエストログには
リクエスト ID が含まれ、利用可能な場合はトレースコンテキストも含まれます。
ローカルの [`.env.example`](.env.example) ではトレースのエクスポートを無効にしています。
エクスポートには外部 Collector を設定してください。このリポジトリは
Collector を起動しません。実装規約は
[ログガイド](internal/logging/AGENTS.md)と
[テレメトリーガイド](internal/telemetry/AGENTS.md)を参照してください。
