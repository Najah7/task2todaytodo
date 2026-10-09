# List API の cursor pagination と field mask

## 目的と範囲

現在公開されている次の7つの List API に、同じページング契約と読み取り用 field mask を適用する。パスはいずれも `/api` 配下。レコードを条件で絞り込む `filter`、Get API、内部処理用の一覧は対象外。

| List API | 表示順（同値時の `id` は cursor のために追加） |
| --- | --- |
| `GET /projects` | `created_at DESC, id DESC` |
| `GET /tasks` | `created_at DESC, id DESC` |
| `GET /projects/{id}/tasks` | `created_at DESC, id DESC` |
| `GET /tags` | `name ASC, id ASC` |
| `GET /tasks/{taskId}/action-items` | `position ASC, occurrence_date ASC, id ASC` |
| `GET /schedules` | `start_at ASC, id ASC` |
| `GET /projects/{id}/schedules` | `start_at ASC, id ASC` |

Tag の名前順、ActionItem の手動順、Schedule の開始時刻順を維持する。`created_at + id` は作成順の3つの List に使う。他の List では、その表示順に対応する値と `id` を cursor に使う。

## ページング契約

- リクエストは `page_size` と `page_token` を使用する。初回は `page_token` を省略する。
- field mask を指定しない場合のレスポンスは全 List で `{ "items": [...], "next_page_token": "..." }`。最終ページの `next_page_token` は空文字列。`items` を返す場合、空でも配列とする。総件数は返さない。
- `page_size` の既定値は50、上限は100。省略または0は50、100超は100に丸める。負数と整数でない値は400。
- `page_token` は暗号化された URL-safe な不透明文字列で、有効期間は発行から1日。期限切れ、破損、不正な token は400とし、先頭ページからの取得を求める。
- token は利用者、List の種類、親リソース、並び順、`fields` の指定に結び付ける。次ページではこれらを変えられない。`page_size` だけ変更できる。token 自体を認可情報として扱わず、毎回所有者を確認する。
- ページごとに取得時点のデータを読む。固定スナップショットは提供しない。ページ取得中に項目の並べ替えや時刻変更などがあれば、続きのページで重複や取りこぼしが起こり得る。
- 既存の `limit`、`offset`、`next_offset` は廃止する。旧リクエスト引数を送った場合は400。既存の配列や Tag 専用の一覧ラッパーも、共通の `items` ラッパーに切り替える。field mask を指定しない場合、各 item の公開項目と値は変更しない。

## 読み取り用 field mask

- `fields` クエリは返却する JSON 項目を選ぶ。対象は `items` や `next_page_token` を含むレスポンス全体。例: `?fields=items(id,title),next_page_token`。
- `fields` 省略時と `fields=*` は全項目を返す。明示した場合は選ばれなかった項目を返さない。続きの token が必要なら `next_page_token` も選ぶ。
- [Google Docs API の field mask 構文](https://developers.google.com/workspace/docs/api/how-tos/field-masks)に合わせ、カンマ区切り、入れ子のドットと括弧、`*`、snake_case と camelCase の項目名を受け付ける。レスポンスの項目名は既存の snake_case のままにする。
- 存在しない項目や不正な構文は400。`fields` は item の選択や表示順を変えない。
- 今回は HTTP レスポンスの項目選択までを対象とする。DB の列や関連データの取得を field mask に応じて省く最適化は含めない。

ページングの公開形式は [Google AIP-158](https://google.aip.dev/158)、List レスポンス全体を対象とする `fields` の例は [Google Drive API のガイド](https://developers.google.com/workspace/drive/api/guides/fields-parameter)も参考にする。
