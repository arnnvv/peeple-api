-- name: ListAllUsers :many
SELECT * FROM peeple_api_users;

-- name: GetUserByEmail :one
SELECT * FROM peeple_api_users
WHERE email = $1;

-- name: CreateUser :one
INSERT INTO peeple_api_users (email)
VALUES ($1)
RETURNING *;

-- name: UpdateUser :one
UPDATE peeple_api_users
SET name = $1,
    location = $2,
    gender = $3,
    relationshiptype = $4,
    height = $5,
    religion = $6,
    occupation_field = $7,
    occupation_area = $8,
    drink = $9,
    smoke = $10,
    bio = $11,
    date = $12,
    month = $13,
    year = $14,
    instaid = $15,
    phone = $16
WHERE email = $17
RETURNING *;

-- name: GetUserPictures :many
SELECT url FROM peeple_api_pictures
WHERE userEmail = $1;

-- name: AddUserPicture :one
INSERT INTO peeple_api_pictures (userEmail, url)
VALUES ($1, $2)
RETURNING *;

-- name: AddLike :one
INSERT INTO peeple_api_likes (likerEmail, likedEmail)
VALUES ($1, $2)
RETURNING *;

-- name: GetUserLikers :many
SELECT u.email, u.name
FROM peeple_api_likes l
JOIN peeple_api_users u ON l.likerEmail = u.email
WHERE l.likedEmail = $1;

-- name: GetMutualLikes :many
SELECT DISTINCT u.name, u.instaid, u.phone, p.url as photo_url
FROM peeple_api_likes l1
JOIN peeple_api_likes l2 ON l1.likerEmail = l2.likedEmail AND l1.likedEmail = l2.likerEmail
JOIN peeple_api_users u ON l2.likerEmail = u.email
LEFT JOIN peeple_api_pictures p ON u.email = p.email
WHERE l1.likerEmail = $1;

-- name: GetSubscription :one
SELECT subscription FROM peeple_api_users
WHERE email = $1;

-- name: UpdateSubscription :one
UPDATE peeple_api_users
SET subscription = $1
WHERE email = $2
RETURNING *;
