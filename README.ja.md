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

概要と実装・設定ガイドへのリンクは、
[バックエンドのログとトレースの概要](backend/README.ja.md#ログとトレース)を参照してください。

## フロントエンドのクイックスタート

前提条件: Node.js、pnpm 10.5.2。リポジトリルートから、別のターミナルで、上記の手順でバックエンドを起動した後にフロントエンドを起動します。Vite は既定で `/api` を `http://127.0.0.1:8080` に転送します。

```bash
cd frontend
pnpm install --frozen-lockfile
cp .env.example .env # 初回セットアップ時のみ
pnpm dev
```

<http://localhost:5173> を開きます。バックエンドの接続先が異なる場合は `frontend/.env` の `API_PROXY_TARGET` を設定してください。コマンドや詳細は[フロントエンド README](frontend/README.md)を参照してください。
