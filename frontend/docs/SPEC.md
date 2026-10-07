- `/login`: メールアドレスとパスワードでログイン。
- `/signup`: 登録後、同じ入力でログイン。
- 成功後は `localStorage` の `personal_access_token` にバックエンドの `personal_access_token` を保存し、`/today` に遷移します。
- その他のページはログイン不要です。期限判定やrefresh処理はフロントで行いません。
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
