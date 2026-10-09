-- Rebuild the local demo dataset. This intentionally removes all application
-- data from the configured database, while preserving schema and master data.
SET TIME ZONE 'Asia/Tokyo';

BEGIN;

TRUNCATE TABLE
    personal_access_tokens,
    todo_list_schedules,
    todo_list_items,
    todo_lists,
    schedule_tag_assignments,
    schedule_frequencies,
    schedules,
    action_item_frequencies,
    action_items,
    task_tag_assignments,
    tags,
    tasks,
    projects,
    users
RESTART IDENTITY CASCADE;

INSERT INTO users (id, first_name, last_name, email, password, timezone) VALUES
    ('00000000000000000000000001', '太郎', '山田', 'demo@example.com', 'hashed_Password1!', 'Asia/Tokyo'),
    ('00000000000000000000000002', '花子', '佐藤', 'other@example.com', 'hashed_Password1!', 'Asia/Tokyo');

INSERT INTO projects (id, user_id, type, title, goal, description, priority, start_date, end_date) VALUES
    ('00000000000000000000000101', '00000000000000000000000001', 'work', 'プロダクト改善', '今四半期の主要改善をリリースする', '要件整理からリリースまでの仕事', 'high', CURRENT_DATE - 14, CURRENT_DATE + 45),
    ('00000000000000000000000102', '00000000000000000000000001', 'study', '英語学習', '年内に英語記事を辞書なしで読む', '毎週の学習を積み上げる', 'medium', CURRENT_DATE - 30, CURRENT_DATE + 90),
    ('00000000000000000000000103', '00000000000000000000000001', 'personal_project', '読書メモアプリ', NULL, '小さく作って使いながら改善する', 'low', CURRENT_DATE, NULL),
    ('00000000000000000000000104', '00000000000000000000000002', 'other', '個人用プロジェクト', NULL, NULL, 'low', NULL, NULL);

INSERT INTO projects (id, user_id, type, title, goal, description, priority, start_date, end_date)
SELECT
    lpad((105 + n)::text, 26, '0'),
    '00000000000000000000000001',
    (ARRAY['work', 'study', 'side_work', 'hobby', 'personal_project']::text[])[(n % 5) + 1],
    (ARRAY['週次レビュー', '資料づくり', '学習計画', '家の整理', '開発メモ']::text[])[(n % 5) + 1] || ' ' || to_char(n + 1, 'FM00'),
    (ARRAY['次の節目まで進める', '毎週少しずつ続ける', '使える形に仕上げる']::text[])[(n % 3) + 1],
    '一覧表示・ページング確認用のサンプルプロジェクト',
    (ARRAY['urgent', 'high', 'medium', 'low', 'someday']::text[])[(n % 5) + 1],
    CURRENT_DATE - ((n % 30) + 1),
    CURRENT_DATE + ((n % 60) + 14)
FROM generate_series(0, 15) AS series(n);

INSERT INTO tasks (id, user_id, project_id, title, description, due_date, estimated_minutes, actual_minutes, priority, status) VALUES
    ('00000000000000000000000201', '00000000000000000000000001', '00000000000000000000000101', 'ユーザーインタビューを整理する', '発見した課題をテーマごとにまとめる', CURRENT_DATE + 2, 90, 25, 'urgent', 'in_progress'),
    ('00000000000000000000000202', '00000000000000000000000001', '00000000000000000000000101', '改善案の優先順位を決める', '影響度と実装コストを比較する', CURRENT_DATE + 5, 60, NULL, 'high', 'open'),
    ('00000000000000000000000203', '00000000000000000000000001', '00000000000000000000000101', 'デザインレビューを依頼する', 'レビュー用の画面と論点を準備する', CURRENT_DATE + 1, 30, NULL, 'medium', 'waiting_on_others'),
    ('00000000000000000000000204', '00000000000000000000000001', '00000000000000000000000102', '英単語を復習する', '今週の単語カードを復習する', CURRENT_DATE, 20, 20, 'low', 'done'),
    ('00000000000000000000000205', '00000000000000000000000001', '00000000000000000000000102', '英語記事を1本読む', 'プロダクト開発の記事を読む', CURRENT_DATE + 1, 40, NULL, 'medium', 'open'),
    ('00000000000000000000000206', '00000000000000000000000001', '00000000000000000000000103', '画面構成をスケッチする', 'ホームと読書記録画面を考える', CURRENT_DATE + 7, 45, NULL, 'low', 'pending'),
    ('00000000000000000000000207', '00000000000000000000000001', NULL, '郵便を出す', '駅前の郵便局に寄る', CURRENT_DATE, 15, NULL, 'high', 'open'),
    ('00000000000000000000000208', '00000000000000000000000001', NULL, '古いメモを整理する', NULL, NULL, NULL, NULL, 'someday', 'open'),
    ('00000000000000000000000209', '00000000000000000000000002', NULL, '別ユーザーのサンプルタスク', NULL, NULL, 30, NULL, 'low', 'open');

INSERT INTO tasks (id, user_id, project_id, title, description, due_date, estimated_minutes, actual_minutes, priority, status)
SELECT
    lpad((220 + n)::text, 26, '0'),
    '00000000000000000000000001',
    CASE
        WHEN n < 32 THEN lpad((105 + (n % 16))::text, 26, '0')
        WHEN n < 35 THEN '00000000000000000000000101'
        ELSE NULL
    END,
    (ARRAY['次の作業を進める', '内容を確認する', '関係者に共有する', '必要な情報を集める', '結果を記録する', 'レビューを依頼する']::text[])[(n % 6) + 1] || ' ' || to_char(n + 1, 'FM00'),
    'GET /api/tasks の一覧・カーソルページング確認用データ',
    CURRENT_DATE + (n % 21),
    15 + ((n % 6) * 15),
    CASE WHEN n % 5 IN (2, 3) THEN 15 + ((n % 4) * 15) ELSE NULL END,
    (ARRAY['urgent', 'high', 'medium', 'low', 'someday']::text[])[(n % 5) + 1],
    (ARRAY['open', 'pending', 'waiting_on_others', 'in_progress', 'done']::text[])[(n % 5) + 1]
FROM generate_series(0, 35) AS series(n);

INSERT INTO tags (id, user_id, name) VALUES
    ('00000000000000000000000301', '00000000000000000000000001', '重要'),
    ('00000000000000000000000302', '00000000000000000000000001', '短時間'),
    ('00000000000000000000000303', '00000000000000000000000001', '集中'),
    ('00000000000000000000000304', '00000000000000000000000002', '個人用');

INSERT INTO tags (id, user_id, name)
SELECT
    lpad((305 + n)::text, 26, '0'),
    '00000000000000000000000001',
    (ARRAY['定例', '調査', 'レビュー', '連絡', '作成', '確認', '学習', '生活', '今週', '来週', '朝', '午後', '軽作業', '深い作業', '待ち', 'アイデア']::text[])[n + 1]
FROM generate_series(0, 15) AS series(n);

INSERT INTO task_tag_assignments (task_id, tag_id) VALUES
    ('00000000000000000000000201', '00000000000000000000000301'),
    ('00000000000000000000000201', '00000000000000000000000303'),
    ('00000000000000000000000203', '00000000000000000000000302'),
    ('00000000000000000000000205', '00000000000000000000000303'),
    ('00000000000000000000000207', '00000000000000000000000302'),
    ('00000000000000000000000209', '00000000000000000000000304');

INSERT INTO task_tag_assignments (task_id, tag_id)
SELECT
    lpad((220 + n)::text, 26, '0'),
    lpad((305 + (n % 16))::text, 26, '0')
FROM generate_series(0, 35) AS series(n);

-- One-off and recurring ActionItems. Only the first occurrence is stored for a series.
INSERT INTO action_items (id, task_id, title, description, due_date, completed, position, series_id, occurrence_date, timezone, is_exception) VALUES
    ('00000000000000000000000401', '00000000000000000000000201', 'インタビュー記録を読み返す', NULL, CURRENT_DATE, true, 0, '00000000000000000000000401', CURRENT_DATE, 'Asia/Tokyo', false),
    ('00000000000000000000000402', '00000000000000000000000201', '課題を3つに分類する', '利用者、業務、システムの観点で分類', CURRENT_DATE, false, 1, '00000000000000000000000402', CURRENT_DATE, 'Asia/Tokyo', false),
    ('00000000000000000000000403', '00000000000000000000000201', 'チームに要点を共有する', NULL, CURRENT_DATE + 1, false, 2, '00000000000000000000000403', CURRENT_DATE, 'Asia/Tokyo', false),
    ('00000000000000000000000404', '00000000000000000000000205', '記事の要点を3行で書く', NULL, CURRENT_DATE + 1, false, 0, '00000000000000000000000404', CURRENT_DATE, 'Asia/Tokyo', false),
    ('00000000000000000000000405', '00000000000000000000000204', '英単語を復習する', '平日に短時間の復習', CURRENT_DATE, true, 0, '00000000000000000000000405', date_trunc('week', CURRENT_DATE)::date, 'Asia/Tokyo', false);

INSERT INTO action_items (id, task_id, title, description, due_date, completed, position, series_id, occurrence_date, timezone)
SELECT
    lpad((408 + n)::text, 26, '0'),
    lpad((220 + n)::text, 26, '0'),
    (ARRAY['作業内容を確認する', '必要な資料を集める', '作業を進める', '結果を記録する']::text[])[(n % 4) + 1],
    'ActionItem一覧・ページング確認用データ',
    CASE WHEN n < 12 THEN CURRENT_DATE - (n + 2) ELSE CURRENT_DATE + (n % 10) END,
    n < 12 OR n % 4 = 0,
    0,
    lpad((408 + n)::text, 26, '0'),
    CASE WHEN n < 12 THEN CURRENT_DATE - (n + 2) ELSE CURRENT_DATE END,
    'Asia/Tokyo'
FROM generate_series(0, 27) AS series(n);

UPDATE action_items
SET repeat_state = 'active', frequency_anchor_date = occurrence_date, interval_weeks = 1
WHERE id = '00000000000000000000000405';

INSERT INTO action_item_frequencies (action_item_id, frequency) VALUES
    ('00000000000000000000000405', 'mon'),
    ('00000000000000000000000405', 'wed'),
    ('00000000000000000000000405', 'fri');

-- Fixed appointments, a weekly recurring series, and one edited occurrence.
INSERT INTO schedules (id, user_id, project_id, assignee_id, title, description, location, start_at, end_at, series_id, occurrence_date, timezone, is_exception, repeat_state, frequency_anchor_date, interval_weeks)
SELECT x.id, t.user_id, t.project_id, t.assignee_id, x.title, x.description, x.location, x.start_at, x.end_at, x.series_id, x.occurrence_date, x.timezone, x.is_exception, x.repeat_state, x.frequency_anchor_date, x.interval_weeks
FROM (VALUES
    ('00000000000000000000000501', '00000000000000000000000201', '改善チーム朝会', '進捗と今日の優先事項を共有', 'オンライン', (date_trunc('week', CURRENT_DATE)::date + time '09:30') AT TIME ZONE 'Asia/Tokyo', (date_trunc('week', CURRENT_DATE)::date + time '10:00') AT TIME ZONE 'Asia/Tokyo', '00000000000000000000000501', date_trunc('week', CURRENT_DATE)::date, 'Asia/Tokyo', false, 'active', date_trunc('week', CURRENT_DATE)::date, 1),
    ('00000000000000000000000503', '00000000000000000000000201', '改善チーム朝会（時間変更）', '今週のみ時間を変更', 'オンライン', (date_trunc('week', CURRENT_DATE)::date + 14 + time '10:00') AT TIME ZONE 'Asia/Tokyo', (date_trunc('week', CURRENT_DATE)::date + 14 + time '10:30') AT TIME ZONE 'Asia/Tokyo', '00000000000000000000000501', date_trunc('week', CURRENT_DATE)::date + 14, 'Asia/Tokyo', true, NULL, NULL, 0),
    ('00000000000000000000000504', '00000000000000000000000207', '郵便局に立ち寄る', '郵便を出してから帰宅', '駅前郵便局', (CURRENT_DATE + time '16:00') AT TIME ZONE 'Asia/Tokyo', (CURRENT_DATE + time '16:20') AT TIME ZONE 'Asia/Tokyo', '00000000000000000000000504', CURRENT_DATE, 'Asia/Tokyo', false, 'one_off', NULL, 0),
    ('00000000000000000000000505', '00000000000000000000000205', '英語オンラインレッスン', '会話レッスン', '自宅', (CURRENT_DATE + 1 + time '19:00') AT TIME ZONE 'Asia/Tokyo', (CURRENT_DATE + 1 + time '19:30') AT TIME ZONE 'Asia/Tokyo', '00000000000000000000000505', CURRENT_DATE + 1, 'Asia/Tokyo', false, 'one_off', NULL, 0)
) AS x(id, task_id, title, description, location, start_at, end_at, series_id, occurrence_date, timezone, is_exception, repeat_state, frequency_anchor_date, interval_weeks)
JOIN tasks t ON t.id = x.task_id;

INSERT INTO schedules (id, user_id, project_id, assignee_id, title, description, location, start_at, end_at, series_id, occurrence_date, timezone)
SELECT
    lpad((506 + n)::text, 26, '0'),
    t.user_id,
    t.project_id,
    t.assignee_id,
    (ARRAY['作業時間', '確認ミーティング', '学習時間', 'レビュー枠']::text[])[(n % 4) + 1] || ' ' || to_char(n + 1, 'FM00'),
    'Schedule一覧確認用の単発予定',
    (ARRAY['自宅', 'オンライン', 'オフィス', '図書館']::text[])[(n % 4) + 1],
    ((CURRENT_DATE + n + 1)::timestamp + time '09:00') AT TIME ZONE 'Asia/Tokyo',
    ((CURRENT_DATE + n + 1)::timestamp + time '09:45') AT TIME ZONE 'Asia/Tokyo',
    lpad((506 + n)::text, 26, '0'),
    CURRENT_DATE + n + 1,
    'Asia/Tokyo'
FROM generate_series(0, 19) AS series(n)
JOIN tasks t ON t.id = lpad((220 + n)::text, 26, '0');

INSERT INTO schedule_frequencies (schedule_id, frequency) VALUES
    ('00000000000000000000000501', 'mon');

-- Active-root snapshots are inserted after the frequency set is complete.
INSERT INTO schedule_revisions (
    id, revision, user_id, project_id, assignee_id, title, description, location,
    start_at, end_at, series_id, occurrence_date, timezone, is_exception,
    repeat_state, frequency_anchor_date, interval_weeks, completed, deleted_at,
    skipped_at, frequencies, created_at, updated_at, changed_by, changed_at
)
SELECT s.id, s.revision, s.user_id, s.project_id, s.assignee_id, s.title, s.description, s.location,
       s.start_at, s.end_at, s.series_id, s.occurrence_date, s.timezone, s.is_exception,
       s.repeat_state, s.frequency_anchor_date, s.interval_weeks, s.completed, s.deleted_at,
       s.skipped_at,
       ARRAY(SELECT sf.frequency FROM schedule_frequencies sf WHERE sf.schedule_id = s.id ORDER BY sf.frequency),
       s.created_at, s.updated_at, s.changed_by, s.updated_at
FROM schedules s
WHERE s.id = s.series_id AND s.repeat_state = 'active';

INSERT INTO todo_lists (id, user_id, list_date) VALUES
    ('00000000000000000000000601', '00000000000000000000000001', CURRENT_DATE),
    ('00000000000000000000000602', '00000000000000000000000001', CURRENT_DATE - 1),
    ('00000000000000000000000603', '00000000000000000000000002', CURRENT_DATE);

INSERT INTO todo_lists (id, user_id, list_date)
SELECT
    lpad((604 + n)::text, 26, '0'),
    '00000000000000000000000001',
    CURRENT_DATE - (n + 2)
FROM generate_series(0, 11) AS series(n);

INSERT INTO todo_list_items (todo_list_id, action_item_id, position) VALUES
    ('00000000000000000000000601', '00000000000000000000000401', 0),
    ('00000000000000000000000601', '00000000000000000000000402', 1),
    ('00000000000000000000000602', '00000000000000000000000405', 0);

INSERT INTO todo_list_items (todo_list_id, action_item_id, position)
SELECT
    lpad((604 + n)::text, 26, '0'),
    lpad((408 + n)::text, 26, '0'),
    0
FROM generate_series(0, 11) AS series(n);

INSERT INTO todo_list_schedules (todo_list_id, schedule_id) VALUES
    ('00000000000000000000000601', '00000000000000000000000501'),
    ('00000000000000000000000601', '00000000000000000000000504');

COMMIT;
