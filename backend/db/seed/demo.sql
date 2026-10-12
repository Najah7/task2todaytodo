-- Rebuild the local demo dataset. This intentionally removes all application
-- data from the configured database, while preserving schema and master data.
SET TIME ZONE 'Asia/Tokyo';

BEGIN;

TRUNCATE TABLE
    personal_access_tokens,
    auth_link_tokens,
    email_delivery_attempts,
    admin_operations,
    todo_items,
    todos,
    schedule_revisions,
    schedule_backfill_checkpoints,
    schedule_project_pause_intervals,
    schedules,
    schedule_rule_revisions,
    schedule_rules,
    action_items,
    tasks,
    projects,
    users
RESTART IDENTITY CASCADE;

INSERT INTO users (id, first_name, last_name, email, password_hash, email_verified_at, timezone) VALUES
    ('00000000000000000000000001', '太郎', '山田', 'demo@example.com', '$argon2id$v=19$m=19456,t=2,p=1$z8gO7D+jUgyJPqCiBuAqmQ$mB7kxjqyP0o/OAhMYBu0McbqOYa/DsuEbgCD5cUR4dE', now(), 'Asia/Tokyo'),
    ('00000000000000000000000002', '花子', '佐藤', 'other@example.com', '$argon2id$v=19$m=19456,t=2,p=1$Q+/+8Q9/OL0qgJfuNZE1hg$4m2NIOuWWY/r7xAVMy2LKR7mrtrTd7n6leELigXKP6w', now(), 'Asia/Tokyo');

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

INSERT INTO tasks (id, user_id, project_id, title, description, due_date, manual_estimated_minutes, actual_minutes, priority, status) VALUES
    ('00000000000000000000000201', '00000000000000000000000001', '00000000000000000000000101', 'ユーザーインタビューを整理する', '発見した課題をテーマごとにまとめる', CURRENT_DATE + 2, 90, 25, 'urgent', 'in_progress'),
    ('00000000000000000000000202', '00000000000000000000000001', '00000000000000000000000101', '改善案の優先順位を決める', '影響度と実装コストを比較する', CURRENT_DATE + 5, 60, NULL, 'high', 'open'),
    ('00000000000000000000000203', '00000000000000000000000001', '00000000000000000000000101', 'デザインレビューを依頼する', 'レビュー用の画面と論点を準備する', CURRENT_DATE + 1, 30, NULL, 'medium', 'waiting_on_others'),
    ('00000000000000000000000204', '00000000000000000000000001', '00000000000000000000000102', '英単語を復習する', '今週の単語カードを復習する', CURRENT_DATE, 20, 20, 'low', 'done'),
    ('00000000000000000000000205', '00000000000000000000000001', '00000000000000000000000102', '英語記事を1本読む', 'プロダクト開発の記事を読む', CURRENT_DATE + 1, 40, NULL, 'medium', 'open'),
    ('00000000000000000000000206', '00000000000000000000000001', '00000000000000000000000103', '画面構成をスケッチする', 'ホームと読書記録画面を考える', CURRENT_DATE + 7, 45, NULL, 'low', 'pending'),
    ('00000000000000000000000207', '00000000000000000000000001', NULL, '郵便を出す', '駅前の郵便局に寄る', CURRENT_DATE, 15, NULL, 'high', 'open'),
    ('00000000000000000000000208', '00000000000000000000000001', NULL, '古いメモを整理する', NULL, NULL, 30, NULL, 'someday', 'open'),
    ('00000000000000000000000209', '00000000000000000000000002', NULL, '別ユーザーのサンプルタスク', NULL, NULL, 30, NULL, 'low', 'open');

INSERT INTO tasks (id, user_id, project_id, title, description, due_date, manual_estimated_minutes, actual_minutes, priority, status)
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

-- ActionItems are single persisted rows; recurrence applies to Schedules only.
INSERT INTO action_items (id, task_id, title, description, due_date, completed, position, estimated_minutes) VALUES
    ('00000000000000000000000401', '00000000000000000000000201', 'インタビュー記録を読み返す', NULL, CURRENT_DATE, true, 0, 15),
    ('00000000000000000000000402', '00000000000000000000000201', '課題を3つに分類する', '利用者、業務、システムの観点で分類', CURRENT_DATE, false, 1, 30),
    ('00000000000000000000000403', '00000000000000000000000201', 'チームに要点を共有する', NULL, CURRENT_DATE + 1, false, 2, 20),
    ('00000000000000000000000404', '00000000000000000000000205', '記事の要点を3行で書く', NULL, CURRENT_DATE + 1, false, 0, 15),
    ('00000000000000000000000405', '00000000000000000000000204', '英単語を復習する', '平日に短時間の復習', CURRENT_DATE, true, 0, 10);

INSERT INTO action_items (id, task_id, title, description, due_date, completed, position, estimated_minutes)
SELECT
    lpad((408 + n)::text, 26, '0'),
    lpad((220 + n)::text, 26, '0'),
    (ARRAY['作業内容を確認する', '必要な資料を集める', '作業を進める', '結果を記録する']::text[])[(n % 4) + 1],
    'ActionItem一覧・ページング確認用データ',
    CASE WHEN n < 12 THEN CURRENT_DATE - (n + 2) ELSE CURRENT_DATE + (n % 10) END,
    n < 12 OR n % 4 = 0,
    0,
    10 + ((n % 4) * 5)
FROM generate_series(0, 27) AS series(n);

-- A recurring rule keeps its recurrence definition in revisions; the backfill
-- job materializes concrete occurrences later. Effective revisions demonstrate a
-- future change without rewriting prior schedule occurrences.
INSERT INTO schedule_rules (id, user_id) VALUES
    ('00000000000000000000000501', '00000000000000000000000001');

INSERT INTO schedule_rule_revisions (
    id, rule_id, user_id, revision, effective_from, recurrence_anchor_date,
    local_start, local_end, timezone, frequency, interval_count, weekdays,
    title, description, location, project_id, assignee_id, changed_by
) VALUES
    ('00000000000000000000000511', '00000000000000000000000501', '00000000000000000000000001', 1,
     date_trunc('week', CURRENT_DATE)::date, date_trunc('week', CURRENT_DATE)::date,
     date_trunc('week', CURRENT_DATE)::date + time '09:30', date_trunc('week', CURRENT_DATE)::date + time '10:00', 'Asia/Tokyo', 'weekly', 1, ARRAY['mon'],
     '改善チーム朝会', '進捗と今日の優先事項を共有', 'オンライン', '00000000000000000000000101',
     '00000000000000000000000001', '00000000000000000000000001'),
    ('00000000000000000000000512', '00000000000000000000000501', '00000000000000000000000001', 2,
     date_trunc('week', CURRENT_DATE)::date + 14, date_trunc('week', CURRENT_DATE)::date,
     date_trunc('week', CURRENT_DATE)::date + 14 + time '10:00', date_trunc('week', CURRENT_DATE)::date + 14 + time '10:30', 'Asia/Tokyo', 'weekly', 1, ARRAY['mon'],
     '改善チーム朝会', '進捗と今日の優先事項を共有（時間変更）', 'オンライン', '00000000000000000000000101',
     '00000000000000000000000001', '00000000000000000000000001');

-- One-off appointments have both a rule/revision and their concrete occurrence.
WITH fixtures AS (
    SELECT * FROM (VALUES
        ('00000000000000000000000504', '00000000000000000000000514', '00000000000000000000000207', '郵便局に立ち寄る', '郵便を出してから帰宅', '駅前郵便局', CURRENT_DATE, time '16:00', time '16:20'),
        ('00000000000000000000000505', '00000000000000000000000515', '00000000000000000000000205', '英語オンラインレッスン', '会話レッスン', '自宅', CURRENT_DATE + 1, time '19:00', time '19:30')
    ) AS x(schedule_id, revision_id, task_id, title, description, location, occurrence_date, local_start, local_end)
), source AS (
    SELECT f.*, t.user_id, t.project_id, t.assignee_id
    FROM fixtures f JOIN tasks t ON t.id=f.task_id
)
INSERT INTO schedule_rules (id, user_id)
SELECT schedule_id, user_id FROM source;

WITH fixtures AS (
    SELECT * FROM (VALUES
        ('00000000000000000000000504', '00000000000000000000000514', '00000000000000000000000207', '郵便局に立ち寄る', '郵便を出してから帰宅', '駅前郵便局', CURRENT_DATE, time '16:00', time '16:20'),
        ('00000000000000000000000505', '00000000000000000000000515', '00000000000000000000000205', '英語オンラインレッスン', '会話レッスン', '自宅', CURRENT_DATE + 1, time '19:00', time '19:30')
    ) AS x(schedule_id, revision_id, task_id, title, description, location, occurrence_date, local_start, local_end)
), source AS (
    SELECT f.*, t.user_id, t.project_id, t.assignee_id
    FROM fixtures f JOIN tasks t ON t.id=f.task_id
)
INSERT INTO schedule_rule_revisions (
    id, rule_id, user_id, revision, effective_from, recurrence_anchor_date,
    local_start, local_end, timezone, frequency, interval_count, weekdays,
    title, description, location, project_id, assignee_id, changed_by
)
SELECT revision_id, schedule_id, user_id, 1, occurrence_date, occurrence_date,
       occurrence_date + local_start, occurrence_date + local_end, 'Asia/Tokyo',
       'once', 0, '{}', title, description, location, project_id, assignee_id, user_id
FROM source;

WITH fixtures AS (
    SELECT * FROM (VALUES
        ('00000000000000000000000504', '00000000000000000000000514', '00000000000000000000000207', '郵便局に立ち寄る', '郵便を出してから帰宅', '駅前郵便局', CURRENT_DATE, time '16:00', time '16:20'),
        ('00000000000000000000000505', '00000000000000000000000515', '00000000000000000000000205', '英語オンラインレッスン', '会話レッスン', '自宅', CURRENT_DATE + 1, time '19:00', time '19:30')
    ) AS x(schedule_id, revision_id, task_id, title, description, location, occurrence_date, local_start, local_end)
), source AS (
    SELECT f.*, t.user_id, t.project_id, t.assignee_id
    FROM fixtures f JOIN tasks t ON t.id=f.task_id
)
INSERT INTO schedules (
    id, rule_id, source_revision_id, user_id, occurrence_date,
    start_at, end_at, title, description, location, project_id,
    assignee_id, timezone, changed_by
)
SELECT schedule_id, schedule_id, revision_id, user_id, occurrence_date,
       (occurrence_date + local_start) AT TIME ZONE 'Asia/Tokyo',
       (occurrence_date + local_end) AT TIME ZONE 'Asia/Tokyo',
       title, description, location, project_id, assignee_id, 'Asia/Tokyo', user_id
FROM source;

-- Additional one-off fixtures support list pagination while keeping the same
-- rule -> revision -> occurrence relationship as the primary examples.
WITH source AS (
    SELECT lpad((506+n)::text, 26, '0') AS schedule_id,
           lpad((806+n)::text, 26, '0') AS revision_id,
           lpad((220+n)::text, 26, '0') AS task_id,
           CURRENT_DATE+n+1 AS occurrence_date,
           t.user_id, t.project_id, t.assignee_id,
           (ARRAY['作業時間', '確認ミーティング', '学習時間', 'レビュー枠']::text[])[(n % 4)+1] || ' ' || to_char(n+1, 'FM00') AS title,
           'Schedule一覧確認用の単発予定' AS description,
           (ARRAY['自宅', 'オンライン', 'オフィス', '図書館']::text[])[(n % 4)+1] AS location
    FROM generate_series(0, 19) AS series(n)
    JOIN tasks t ON t.id=lpad((220+n)::text, 26, '0')
)
INSERT INTO schedule_rules (id, user_id)
SELECT schedule_id, user_id FROM source;

WITH source AS (
    SELECT lpad((506+n)::text, 26, '0') AS schedule_id,
           lpad((806+n)::text, 26, '0') AS revision_id,
           lpad((220+n)::text, 26, '0') AS task_id,
           CURRENT_DATE+n+1 AS occurrence_date,
           t.user_id, t.project_id, t.assignee_id,
           (ARRAY['作業時間', '確認ミーティング', '学習時間', 'レビュー枠']::text[])[(n % 4)+1] || ' ' || to_char(n+1, 'FM00') AS title,
           'Schedule一覧確認用の単発予定' AS description,
           (ARRAY['自宅', 'オンライン', 'オフィス', '図書館']::text[])[(n % 4)+1] AS location
    FROM generate_series(0, 19) AS series(n)
    JOIN tasks t ON t.id=lpad((220+n)::text, 26, '0')
)
INSERT INTO schedule_rule_revisions (
    id, rule_id, user_id, revision, effective_from, recurrence_anchor_date,
    local_start, local_end, timezone, frequency, interval_count, weekdays,
    title, description, location, project_id, assignee_id, changed_by
)
SELECT revision_id, schedule_id, user_id, 1, occurrence_date, occurrence_date,
       occurrence_date + time '09:00', occurrence_date + time '09:45',
       'Asia/Tokyo', 'once', 0, '{}', title, description, location,
       project_id, assignee_id, user_id
FROM source;

WITH source AS (
    SELECT lpad((506+n)::text, 26, '0') AS schedule_id,
           lpad((806+n)::text, 26, '0') AS revision_id,
           lpad((220+n)::text, 26, '0') AS task_id,
           CURRENT_DATE+n+1 AS occurrence_date,
           t.user_id, t.project_id, t.assignee_id,
           (ARRAY['作業時間', '確認ミーティング', '学習時間', 'レビュー枠']::text[])[(n % 4)+1] || ' ' || to_char(n+1, 'FM00') AS title,
           'Schedule一覧確認用の単発予定' AS description,
           (ARRAY['自宅', 'オンライン', 'オフィス', '図書館']::text[])[(n % 4)+1] AS location
    FROM generate_series(0, 19) AS series(n)
    JOIN tasks t ON t.id=lpad((220+n)::text, 26, '0')
)
INSERT INTO schedules (
    id, rule_id, source_revision_id, user_id, occurrence_date,
    start_at, end_at, title, description, location, project_id,
    assignee_id, timezone, changed_by
)
SELECT schedule_id, schedule_id, revision_id, user_id, occurrence_date,
       (occurrence_date + time '09:00') AT TIME ZONE 'Asia/Tokyo',
       (occurrence_date + time '09:45') AT TIME ZONE 'Asia/Tokyo',
       title, description, location, project_id, assignee_id, 'Asia/Tokyo', user_id
FROM source;


COMMIT;
