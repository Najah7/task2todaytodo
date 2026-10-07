# Task2TodayToDo

[English](README.md)

***明日のために生きるな、今日を全力で生きよう。***

task2todaytodo は、プロジェクト、タスク、TodoItem、独立したスケジュールを管理し、日々の実行計画を生成します。

週単位や月単位で仕事を計画することは、始めるのは簡単でも、維持し続けるのは難しいものです。割り込みによって計画はずれ込み、長期的なカレンダーを手作業で修正し続けることはすぐに大きな負担になります。task2todaytodo は、プロジェクト、タスク、TodoItem、スケジュールを構造化された情報源として管理し、必要なときに今日の TodoList を生成することで、その負担を避けます。

長期的に管理される仕事と、日々の実行計画を分けて扱うことで、このプロダクトは完璧な長期スケジュールの維持をユーザーに強いることなく、その日に合わせた動的で柔軟な TodoList を作成できます。

# 価値

- タスク管理: Task とその TodoItem を管理します。繰り返し作業と単発作業に対応します。
- スケジュール管理: Project に属する、または個人で管理する固定時間の Schedule を管理します。将来的に Google Calendar などと同期できます。
- 今日の TodoList 生成: 管理されているタスクとスケジュールをもとに、その日の実行計画となる TodoList を生成します。

## バックエンドのクイックスタート

前提条件: Docker、Go、Air。

```bash
cd backend
cp .env.example .env # 初回セットアップ時のみ
openssl rand -base64 32 # 出力を backend/.env の PAGE_TOKEN_KEY に設定
make env-up
make migrate-up
air
```

API ドキュメント: <http://localhost:8080/swagger/index.html>

`PAGE_TOKEN_KEY` は API インスタンス間で同じ値を使い、変更すると有効期限内（最大24時間）のページトークンは使えなくなります。

便利なバックエンドコマンドの確認:

```bash
make help        # ヘルプを表示
```

### ログとトレース

API は構造化 JSON ログを標準出力に書き込みます。`LOG_LEVEL` は
`DEBUG`、`INFO`、`WARN`、`ERROR` から設定できます。HTTP リクエストログには
リクエスト ID が含まれます。有効な UUID の受信 ID は保持し、無効または未指定の
場合は UUID を生成します。
アクティブなトレーススパンがある場合、リクエストログには `trace_id`、`span_id`、
`trace_flags` も含まれます。

API は OpenTelemetry のサーバースパンを作成し、W3C Trace Context を伝播します。
トレースは既定で OTLP (`OTEL_TRACES_EXPORTER=otlp`) を使い、既定のプロトコルは
`http/protobuf` です。`OTEL_EXPORTER_OTLP_PROTOCOL=grpc` で gRPC を選べます。
標準の OTLP 環境変数でエンドポイント、ヘッダー、タイムアウト、圧縮、TLS を設定
できます。Air コンテナから接続できる外部 Collector を指定してください。例えば
`OTEL_EXPORTER_OTLP_ENDPOINT=http://host.docker.internal:4318` を設定します。
このリポジトリでは Collector を起動しません。
サンプリングの既定値は `parentbased_always_on` です。`OTEL_TRACES_SAMPLER` と
`OTEL_TRACES_SAMPLER_ARG` で別の対応サンプラーや比率を指定できます。

ローカルの `.env.example` は `OTEL_TRACES_EXPORTER=none` を既定値にしています。
Collector が使える場合は `otlp` に変更してください。`OTEL_SERVICE_NAME` の既定値は
`api`、`ENVIRONMENT` は `development`、`APP_VERSION` は `1.0.0` です。
OpenTelemetry のリソース属性で環境名とバージョンを上書きできます。サービス名は
標準仕様に従い `OTEL_SERVICE_NAME` が優先されます。解決後の値はログとトレースで
共有されます。

PostgreSQL の診断には PostgreSQL サーバーのコンテナログを使ってください。
アプリケーションログに SQL パラメーターや認証情報を出さないでください。Collector
を使うと、API トレースと PostgreSQL サーバーログを任意の保存先に転送できます。
