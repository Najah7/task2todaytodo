-- name: ListTaskFrequencies :many
SELECT frequency, label, label_jp, created_at, updated_at
FROM frequency_master;
