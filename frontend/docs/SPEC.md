- `/login`: メールアドレスとパスワードでログイン。
- `/signup`: 登録後、同じ入力でログイン。
- 成功後は `localStorage` の `personal_access_token` にバックエンドの `personal_access_token` を保存し、`/today` に遷移します。
- `/login` と `/signup` 以外のページは、保存済みのアクセストークンがない場合に `/login` へ置き換えて遷移します。期限判定やrefresh処理はフロントで行いません。
- Google / X / GitHub認証は未接続です。画面上のボタンは無効化しています。

認証フォームはReact Hook Formの `useForm` と `zodResolver` を使います。入力条件は `components/LoginForm/schema.ts` と `components/SignupForm/schema.ts` のZodスキーマに定義し、入力型もスキーマから推論します。メールアドレスの前後の空白を除去して形式を検証し、パスワードはバックエンドに合わせて8文字以上・英大文字・英小文字・数字・記号を要求します。新規登録では確認用パスワードとの一致も検証します。送信時に検証し、エラーが出た入力は変更時に再検証します。エラー表示は選択中の言語に追従します。

## 表示言語

日本語と英語に対応しています。初回はブラウザの言語設定を使い、日本語環境以外は英語で表示します。ヘッダー右側の言語切替で選んだ言語は `localStorage` に保存します。言語キーと文言は `src/features/i18n/messages.ts`、`i18n("page.today.title")` のような呼び出しと状態管理は `src/features/i18n/` に置いています。翻訳対象はアプリが表示する固定文言です。タスク名など利用者が入力した内容は変換しません。

## 表示モード

ライト／ダークに対応しています。保存済みの選択がなければ、起動時のOS設定 (`prefers-color-scheme`) を使います。ヘッダー右側で言語切替の隣にあるDisplaySwitcherから選んだモードは、`localStorage` の `display_mode` に保存します。保存できない環境でも、開いている間の切替は使えます。

実装は `src/features/display/`、状態は `src/store/mode.ts` のJotai atomに置いています。Reactの初回描画前にモードを初期化し、`html` の `data-display` を通じてグローバルCSS変数とブラウザ標準コントロールの配色を切り替えます。配色は `src/styles/color.css`、影・輪郭線は `src/styles/effects.css` がSSOTです。参照画像は `design/light/` と `design/dark/` にあります。

言語と表示モードの切替は `src/features/shared/components/Switcher/` を共有します。EN / JP、太陽 / 月の並びを固定し、現在の値を丸いつまみに表示します。共通コンポーネントがレイアウト・アニメーション・キーボード操作を持ち、LanguageSwitcherとDisplaySwitcherが内容と設定値を接続します。

認証関連の実装は `src/features/auth/` にまとめています。`components/LoginForm/` と `components/SignupForm/` がそれぞれフォーム・バリデーション・エラー表示・送信処理とCSS Modulesを持ちます。`pages/Login/` と `pages/Signup/` はヘッダー・フォームの配置・ページ間のリンクを持ち、レイアウトのスタイルも各ページに定義します。入力欄と送信ボタンは `components/EmailField/`・`PasswordField/`・`SubmitButton/`、ログイン・トークン保存・Todayへの遷移は `hooks/useLogin.ts` を共有します。全体で共有するコントロールスタイルは `src/styles/controls.module.css` を使います。

本番はフロントと同じoriginで `/api` をバックエンドへ転送してください。別originを使う場合は `VITE_API_BASE_URL` にoriginを指定し、バックエンドでCORSを設定する必要があります。Viteの開発プロキシは本番ビルドには含まれません。

## Projects

`/login` と `/signup` 以外のApp配下routeはReact Routerの共通loaderで保護し、保存済みtokenがない・空の場合はページを表示する前に `/login` へ置き換えて遷移します。route間の移動でもtokenを確認します。ログイン後の復帰URLは追加しません。

`/projects` のProject APIは現在どおりBearer tokenを要求し、未認証・通信・操作エラーは共通Toastで知らせます。

一覧には `in_progress`・`pending`・`done`・`open`・`waiting_on_others`・`trash` のタブをこの順で表示し、初期タブは `in_progress` とします。`trash` は同じ一覧レイアウト内のタブです。行にはProject名・goal・progress・期限・残日数・今日のタスク数のプレースホルダー・status select・操作を表示します。Project名の行クリックは将来のProject別Task表示用TODOのみとし、遷移しません。サブタスク列、完了数、合計行、期限間近のサブタスクセクションは表示しません。

一覧上部の4項目は、選択中タブの全件数、今日から14日後までの期限件数、期限超過件数、今日のProject数プレースホルダー `- 件` です。件数は表示中のcursorページに限らず、選択中タブの全件を対象にします。行の今日のタスク数も `-` とし、TodoListの集計は実装しません。期限超過はサーバーが返す残日数を使って「超過xx日」と表示します。

一覧は単一列の昇順・降順sort、20件のcursorページングに対応します。status・sort・order・cursorはURLに保持し、statusまたはsort変更で先頭ページへ戻します。サーバーが全対象データをsortし、全件summaryを集計してからページを返します。status変更は保存ボタンなしで反映し、対象タブから行を外します。失敗時はToastを表示し、画面をrollbackしたうえで関連一覧を再取得します。

ゴミ箱移動前に、Project配下が通常表示から隠れ、復元すると戻る説明を含む確認を出します。ゴミ箱の行では復元だけを表示し、元のstatusへ戻して該当statusタブを開きます。完全削除・自動削除は行いません。操作ボタンはAPIが返すProject capabilityに従います。

新規・編集フォームはそれぞれ `/projects/new`、`/projects/:id/edit` に配置し、同じフォーム部品を共有します。項目は必須のtitle、ゴール（WHY）、詳細、type、priority、start date、end dateの順です。詳細はゴールの直後に表示します。APIの `goal` と `description` は維持します。typeとpriorityの初期値は `other` と `low`、日付は未設定です。end dateがstart dateより前の場合は入力欄のそばで検証エラーを示し、過去日は許可します。作成・保存は明示submitです。作成成功後は `open` タブへ移動します。

編集中のSPA遷移・キャンセルは独自確認Dialog、再読み込み・タブ終了は変更中だけブラウザー標準確認を使います。保存済み・変更なしでは確認を出しません。編集のrevision conflictではdraftを保持してToastを出し、最新内容を明示的に読み込む操作を提示します。draftを置き換える前に破棄確認を出し、自動上書きや自動retryはしません。

作成・保存・status変更・trash移動・復元の成功と失敗、およびネットワーク・認証エラーを共通Toastで通知します。title・日付などAPIが特定したvalidation fieldも入力欄のそばに示します。Toast実装は `src/features/shared/notification/` の小さな共通interface経由で利用します。
