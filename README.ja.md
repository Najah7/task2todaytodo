# Task2TodayToDo

[English](README.md)

Task2TodayToDo は、タスクと時間の決まった予定から、今日実行できる計画を作るアプリです。作業可能時間、Project ごとの時間配分、優先度を考慮し、会議の合間に何をするか考える負担を減らします。生成した Todo は手動で調整でき、元のタスクを変更しても自動で作り直しません。

## ローカルで動かす

Docker と Compose、Node.js、pnpm 10.5.2 が必要です。リポジトリのルートから、ターミナルを2つ使います。

**ターミナル1 — バックエンド**

```bash
cd backend
test -f .env || cp .env.example .env
```

`PAGE_TOKEN_KEY` がサンプル値のままなら、`openssl rand -base64 32` の出力で `backend/.env` の値を置き換えます。次に PostgreSQL とローカルのメール受信箱を起動します。DB が healthy になったらマイグレーションと API を起動します。

```bash
make env-up
until [ "$(docker inspect --format '{{.State.Health.Status}}' task2todaytodo-db)" = healthy ]; do sleep 1; done
make migrate-up
make dev
```

**ターミナル2 — フロントエンド**

```bash
cd frontend
test -f .env || cp .env.example .env
pnpm install --frozen-lockfile
pnpm dev
```

<http://localhost:5173> で新規登録し、<http://localhost:8025> で確認メールを開きます。認証・ログイン後、Task や Schedule を作成し、**Today** で生成された計画を確認できます。

ローカル利用に Stripe 課金設定は不要です。`backend/.env` の Stripe 項目が空でもアプリは動きます。課金機能には Sandbox の認証情報と Webhook 転送が必要です。
