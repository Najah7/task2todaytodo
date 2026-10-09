# List API の cursor pagination / field mask 実装計画

## 入力と完了条件

実装の公開契約は [SPEC.md](SPEC.md) を正とする。対象は現在 route がある6つの List API のみ。TodoList、Get、繰り返し補充・マスタ参照などの内部一覧は変更しない。

完了条件は、6つの List が同じ `page_size` / `page_token` / `fields` と `{items,next_page_token}` を扱い、既存の表示順・所有者分離を保ち、offset を使わずに次ページを取得できること。Swagger とテストも新契約に合わせる。

## 実現可能性と現状

実装可能。主な変更は HTTP 契約、共通部品、6つの List UseCase、repository と SQL、索引、生成物。新しいデータ表や固定スナップショットは不要。

| 現状 | 影響 |
| --- | --- |
| `task/usecase/pagination.go` は Task 専用の offset、既定50・最大100 | cursor の共通処理に置き換える。`ListTasks` と `ListProjectTasks` の旧 request/result とテストを更新 |
| Projects・Tags・ActionItems・TaskSchedules の List は全件返却 | 全6本を `LIMIT page_size + 1` の keyset query に変更 |
| Tasks 2本は `{items,next_offset}`、Tags は `{tags}`、残り3本は配列 | 全6本を `{items,next_page_token}` に統一 |
| Task と Project は `created_at,id`、Tag は `name,id`、ActionItem は `position,occurrence_date`、Schedule は `start_at` 順 | 後者2本に `id` を最終同値判定として追加し、各順序に対応する cursor 境界を使う |
| DB の時刻は `timestamptz`、公開 DAO の時刻は Unix 秒へ丸められる | cursor は DB の時刻精度を保持する。公開 `created_at` から cursor を再構成しない |
| `backend/AGENTS.md` は一操作一 UseCase と `shared` から task/auth への依存禁止を規定 | `List*UseCase` は残す。共有処理は UseCase ではない独立部品にする |

ローカルの Go は現時点で `go1.23.3`、`backend/go.mod` は Go 1.25.0 を要求する。Swagger 用 Docker image は Go 1.25 だが、実装とテストにも Go 1.25 の環境を用意する。作業ツリーに既存の未コミット変更が多数あるため、対象外の変更を戻さない。

## 設計と実装順

### 1. 共通のページ処理と token

- `internal/pagination` を新設する。ページサイズの検証（未指定・0なら50、負数は不正、100超は100）、`limit + 1` 行からのページ確定、cursor token の発行・検証を担う。リソース固有の SQL、並び順、所有者照合はこの package に入れない。
- token は標準ライブラリの AES-GCM と暗号学的乱数で作る nonce で暗号化・認証し、URL-safe な文字列にする。payload は version、発行・失効時刻、利用者ID、List 識別子、親ID、正規化した field mask、並び順の識別子、最後に返した item の完全精度の sort tuple とする。`page_size` は含めず、次ページで変更できるようにする。token の解析結果を認可に使わない。
- 専用の `PAGE_TOKEN_KEY`（32 byte の鍵を Base64 で渡す想定）を設定し、起動時に必須・長さを検証する。`backend/.env.example` と `backend/docker-compose.yml` の `air` 環境など、開発・テスト・CI・本番の実行構成に設定方法を記載する。未設定なら起動時に失敗させ、unit test は明示的なテスト鍵を注入する。複数インスタンスで同じ鍵を使う。鍵の変更で発行済み token が使えなくなる点を運用上明示し、必要なら旧鍵を最長1日保持する鍵ID付き復号に拡張できる構造にする。実値をリポジトリに保存しない。
- token の期限は発行から24時間。破損、改ざん、期限切れ、未知の version、異なる利用者・List・親ID・mask・並び順は安定した 400 の公開エラーへ変換する。token の文字数と復号後サイズも制限する。
- HTTP handler は query と認証済み利用者・親IDを読み、共通 codec で token を検証して型付き境界を各 List UseCase に渡す。UseCase は `page_size` と境界で repository を呼び、item と次の境界を返す。handler は token を発行する。UseCase に HTTP request/response 型を持ち込まない。
- 次の token は先読みの1行ではなく、**実際に返した最後の item** の tuple から作る。境界情報は field mask で `id` や時刻を非表示にしても取得できるよう、DB row 側で保持する。

### 2. 読み取り用 field mask

- `internal/fieldmask` を新設する。`fields` の解析、項目名の正規化、公開レスポンス schema に対する検証、JSON 投影を担う。task の DTO や usecase を import しない。handler が自分の List response 型を渡す。
- [Google Docs API の構文](https://developers.google.com/workspace/docs/api/how-tos/field-masks)に合わせ、カンマ、ドット、括弧、`*`、snake_case / camelCase を受け付ける。同じ意味の指定は同じ正規形にし、token の mask 照合に使う。レスポンスの JSON 名は既存の snake_case に固定する。
- `fields` 省略・`*` は全体選択。`items` だけなら各 item の全項目、`items(id,title)` なら各 item の指定項目、`next_page_token` だけなら token のみを返す。入れ子の object/array も同じ規則で処理する。選ばれた項目の `null`、`false`、`0`、空配列を欠落扱いしない。
- 未知の項目、誤った入れ子、構文エラー、空の明示値、重複した `fields` クエリなどは400。入力長・入れ子深さを制限する。エラーは field 名を示す安全な公開 detail にする。
- 最初は既存 DTO を作った後の JSON 投影に限定する。JSON の選別では `json.RawMessage` などを使い、Unix 時刻や整数を `float64` 経由で変換しない。SQL の SELECT 列・結合先を mask ごとに切り替えない。item 数と並び順、cursor 境界に影響させない。

### 3. 各 List の keyset query

`db/queries/*.query.sql` にページ単位の query を定義し、各 repository が item の DAO と完全精度の sort tuple を返す。いずれも所有者条件を SQL で維持し、`LIMIT page_size + 1` を使う。次の境界は下表の比較と `ORDER BY` を一致させる。

| List | 次ページの境界 | 対象 SQL / repository |
| --- | --- | --- |
| Projects | `(created_at, id) < (:at, :id)` | `projects.query.sql` / `repository/projects.go` |
| Tasks、Project 内 Tasks | `(created_at, id) < (:at, :id)` | `tasks.query.sql` / `repository/tasks.go` |
| Tags | `name > :name OR (name = :name AND id > :id)`。`citext` の DB 比較をそのまま使う | `task_tags.query.sql` / `repository/task_tags.go` |
| ActionItems | `(position, occurrence_date, id) > (:position, :date, :id)` | `action_items.query.sql` / `repository/action_items.go` |
| TaskSchedules | `(start_at, id) > (:at, :id)` | `task_schedules.query.sql` / `repository/task_schedules.go` |

- `created_at` と `start_at` は DB の `timestamptz` 精度で token に格納する。公開 DAO の Unix 秒や RFC3339 の表示値を cursor に使わない。`occurrence_date` は現行 migration 適用後に NOT NULL。ActionItem の query は削除済みの回を除く既存条件を保つ。
- 最初のページと続きのページで SQL を分けてもよい。任意の cursor 条件を一つの `OR` に押し込むより、対象索引を使う計画が安定する形を `EXPLAIN` で選ぶ。
- Project 内 Tasks と Task 配下の2 List は、既存の親所有確認と子 query の `user_id` 制約を保つ。グローバルな Tasks 一覧も `user_id` で限定する。token の scope 照合はこれらの代替にしない。
- 既存の内部用 `List*` SQL は呼び出し先を確認し、公開 List 用 query だけを置換する。SQL ソース変更後に `make sqlc-gen` を実行し、生成コードは手編集しない。
- 新しい migration（現行の次番号）で、少なくとも Projects `(user_id, created_at DESC, id DESC)`、Tasks `(user_id, created_at DESC, id DESC)` と `(user_id, project_id, created_at DESC, id DESC)`、未削除 ActionItems `(task_id, position, occurrence_date, id)`、未削除 Schedules `(task_id, start_at, id)` を候補にする。Tags の既存 `(user_id, name)` unique index と既存 Schedules index を含め、実データに近い `EXPLAIN` で重複・不要な索引を判断してから確定する。migration には down を付ける。

この索引確認は cursor pagination の検索性能を保つための作業。field mask に応じて DB の取得列・関連データを省く最適化とは別である。

### 4. UseCase・handler・配線

- `usecase/list_tasks.go`、`list_project_tasks.go`、`list_projects.go`、`list_task_tags.go`、`list_action_items.go`、`list_task_schedules.go` の各 `Execute` をページ入力・ページ結果に変更する。旧 `task/usecase/pagination.go` の offset 契約を削除する。各 UseCase は対応する keyset repository port を持つ。
- `handler/{tasks,projects,task_tags,action_items,task_schedules}.go` の List のみを変更し、全6本で共通 query の読み取り・エラー・envelope・field mask 投影を使う。`limit` と `offset` が来たら400とし、黙って無視しない。親IDの不正と所有者不一致は現行の公開エラー方針を保つ。
- `application/application.go` で新しい List UseCase / repository port と token codec を配線し、`cmd/api/main.go` から必要な handler に同じ codec を渡す。新 route は追加しない。秘密鍵の設定は起動時に検証する。
- List response DTO と Swagger 注釈を `{items,next_page_token}`、`page_size`、`page_token`、`fields`、400 応答に更新し、`make swagger-gen` で生成物を更新する。Task と Project 内 Task の item 表現の差は今回変更しない。

## 検証

1. 共通部品の unit test: ページサイズの既定・上限・不正値、`limit + 1` の境界、token の暗号化・URL 安全性・改ざん・24時間期限・scope/親ID/mask/order 不一致・`page_size` 変更、field mask の nested/alias/wildcard/無指定/不正構文/未知項目/zero・null 値保持。
2. 各 List UseCase の test: 空ページ、最後のページ、同値 sort key、所有者・親の確認、DB エラー、最後に返した item を次境界に使うこと。Task と Schedule の時刻は同一秒内の異なる値でも境界が壊れないこと。
3. PostgreSQL を使う repository integration test: 6 List の `ORDER BY` と keyset predicate の一致、ページ間に同一 item が出ないこと、Tag の `citext` 順、ActionItem の position/occurrence_date、Schedule の start_at、未削除条件とユーザー分離を確認する。並べ替え・時刻変更中のページ取得は SPEC の live read 契約として確認する。
4. HTTP test: 全6 route の共通形式、各 item schema で有効な mask（Tag は `items(id,name)`、他は `items(id,title)` など）と `next_page_token` の併用、mask から token を外した応答、旧 offset 引数・異常 token・不正 mask の400、親の404、認証401。既存 handler test の旧配列・`next_offset` 期待値を更新する。
5. Go 1.25 で `go test ./...`、`go build ./cmd/api`、migration up/down、`make sqlc-gen` と `make swagger-gen` 後の差分を確認する。必要な索引は `EXPLAIN` で検証し、最終差分に無関係な変更を混ぜない。

## 受け入れ時の注意

- 旧 offset 契約と無制限配列を利用している外部 client には互換性のない変更。リポジトリ内の現行 frontend には旧契約の呼び出しはない。
- token の鍵は全 API インスタンスで共有し、起動をまたいで維持する。期限内の鍵変更では token が無効になる。
- field mask による転送量削減は達成するが、DB 取得量の削減は今回の完了条件に含めない。
