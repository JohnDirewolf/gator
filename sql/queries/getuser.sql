-- name: GetUser :one
SELECT id, created_at, updated_at, name FROM users 
WHERE name = $1;