-- name: CleanOTP :exec
DELETE FROM otps
WHERE expires_at < NOW();

-- name: DeleteOTPsByPhoneNumber :exec
DELETE FROM otps
WHERE user_id = (SELECT id FROM users WHERE phone_number = $1);

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1 LIMIT 1;

-- name: GetUserByPhone :one
SELECT * FROM users
WHERE phone_number = $1 LIMIT 1;

-- name: GetPendingVerificationUsers :many
SELECT * FROM users
WHERE verification_status = $1; -- $1 should be 'pending'

-- name: CreateUserMinimal :one
INSERT INTO users (
    phone_number, gender
) VALUES (
    $1, $2
)
RETURNING *;

-- name: UpdateUserProfile :one
UPDATE users SET
    name = $1,
    last_name = $2,
    date_of_birth = $3,
    latitude = $4,
    longitude = $5,
    gender = $6,
    dating_intention = $7,
    height = $8,
    hometown = $9,
    job_title = $10,
    education = $11,
    religious_beliefs = $12,
    drinking_habit = $13,
    smoking_habit = $14
WHERE id = $15
RETURNING *;

-- name: UpdateUserRole :one
UPDATE users
SET role = $1
WHERE id = $2
RETURNING *;

-- name: ClearUserMediaURLs :exec
UPDATE users
SET media_urls = '{}'
WHERE id = $1;

-- name: UpdateUserMediaURLs :exec
UPDATE users
SET media_urls = $1
WHERE id = $2;

-- name: UpdateUserVerificationStatus :one
UPDATE users
SET verification_status = $1
WHERE id = $2
RETURNING *;

-- name: UpdateUserVerificationDetails :one
UPDATE users
SET
    verification_pic = $1,
    verification_status = $2
WHERE id = $3
RETURNING *;

-- name: GetOTPByUser :one
SELECT * FROM otps
WHERE user_id = $1
ORDER BY id DESC
LIMIT 1;

-- name: CreateOTP :one
INSERT INTO otps (
    user_id, otp_code, expires_at
) VALUES (
    $1, $2, $3
)
RETURNING *;

-- name: DeleteOTPByID :exec
DELETE FROM otps
WHERE id = $1;

-- name: DeleteOTPByUser :exec
DELETE FROM otps
WHERE user_id = $1;

-- name: CleanOTPs :exec
DELETE FROM otps
WHERE expires_at < NOW();

-- name: DeleteUserStoryTimePrompts :exec
DELETE FROM story_time_prompts WHERE user_id = $1;

-- name: DeleteUserMyTypePrompts :exec
DELETE FROM my_type_prompts WHERE user_id = $1;

-- name: DeleteUserGettingPersonalPrompts :exec
DELETE FROM getting_personal_prompts WHERE user_id = $1;

-- name: DeleteUserDateVibesPrompts :exec
DELETE FROM date_vibes_prompts WHERE user_id = $1;

-- name: CreateStoryTimePrompt :one
INSERT INTO story_time_prompts (user_id, question, answer)
VALUES ($1, $2, $3)
RETURNING *;

-- name: CreateMyTypePrompt :one
INSERT INTO my_type_prompts (user_id, question, answer)
VALUES ($1, $2, $3)
RETURNING *;

-- name: CreateGettingPersonalPrompt :one
INSERT INTO getting_personal_prompts (user_id, question, answer)
VALUES ($1, $2, $3)
RETURNING *;

-- name: CreateDateVibesPrompt :one
INSERT INTO date_vibes_prompts (user_id, question, answer)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetUserStoryTimePrompts :many
SELECT * FROM story_time_prompts WHERE user_id = $1;

-- name: GetUserMyTypePrompts :many
SELECT * FROM my_type_prompts WHERE user_id = $1;

-- name: GetUserGettingPersonalPrompts :many
SELECT * FROM getting_personal_prompts WHERE user_id = $1;

-- name: GetUserDateVibesPrompts :many
SELECT * FROM date_vibes_prompts WHERE user_id = $1;

-- name: GetUserAudioPrompt :one
SELECT id, audio_prompt_question, audio_prompt_answer
FROM users
WHERE id = $1 LIMIT 1;

-- name: UpdateAudioPrompt :one
UPDATE users
SET audio_prompt_question = $1, audio_prompt_answer = $2
WHERE id = $3
RETURNING id, audio_prompt_question, audio_prompt_answer;
