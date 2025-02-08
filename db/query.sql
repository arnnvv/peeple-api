-- name: CreateUser :one
INSERT INTO peeple_users (phone_number)
VALUES ($1)
RETURNING *;

-- name: GetUserByPhoneNumber :one
SELECT * FROM peeple_users 
WHERE phone_number = $1 LIMIT 1;

-- name: GetUser :one
SELECT * FROM peeple_users 
WHERE id = $1 LIMIT 1;
