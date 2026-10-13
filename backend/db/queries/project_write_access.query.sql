-- name: GetProjectWriteAccessFacts :one
SELECT p.user_id AS owner_user_id,
       count(m.user_id)::bigint AS member_count
FROM projects AS p
LEFT JOIN project_members AS m ON m.project_id = p.id
WHERE p.id = sqlc.arg(project_id)::text
GROUP BY p.user_id;
