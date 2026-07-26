-- name: ListTaskFrequencies :many
SELECT frequency, label, label_jp, created_at, updated_at
FROM task_frequency_master;
