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

-- name: AddPhoneNumberInUsers :one
INSERT INTO users (
    phone_number
) VALUES (
    $1
)
RETURNING *;

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
    user_id, otp_code
) VALUES (
    $1, $2
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

-- name: UpsertUserFilters :one
INSERT INTO filters (
    user_id, who_you_want_to_see, radius_km, active_today, age_min, age_max
) VALUES (
    $1, $2, $3, $4, $5, $6
)
ON CONFLICT (user_id) DO UPDATE SET
    who_you_want_to_see = EXCLUDED.who_you_want_to_see,
    radius_km = EXCLUDED.radius_km,
    active_today = EXCLUDED.active_today,
    age_min = EXCLUDED.age_min,
    age_max = EXCLUDED.age_max,
    updated_at = NOW()
RETURNING *;

-- name: GetUserFilters :one
SELECT * FROM filters
WHERE user_id = $1 LIMIT 1;

-- name: LogAppOpen :exec
INSERT INTO app_open_logs (user_id)
VALUES ($1);

-- name: AddDislike :exec
INSERT INTO dislikes (disliker_user_id, disliked_user_id)
VALUES ($1, $2)
ON CONFLICT (disliker_user_id, disliked_user_id) DO NOTHING;

-- name: GetActiveSubscription :one
SELECT * FROM user_subscriptions
WHERE user_id = $1
  AND feature_type = $2 -- e.g., 'unlimited_likes'
  AND expires_at > NOW()
LIMIT 1;

-- name: GetUserConsumable :one
SELECT * FROM user_consumables
WHERE user_id = $1
  AND consumable_type = $2; -- e.g., 'rose'

-- name: DecrementUserConsumable :one
UPDATE user_consumables
SET quantity = quantity - 1,
    updated_at = NOW()
WHERE user_id = $1
  AND consumable_type = $2 -- e.g., 'rose'
  AND quantity > 0
RETURNING *;

-- name: UpsertUserConsumable :one
INSERT INTO user_consumables (user_id, consumable_type, quantity)
VALUES ($1, $2, $3) -- $3 is the quantity to add
ON CONFLICT (user_id, consumable_type) DO UPDATE
SET quantity = user_consumables.quantity + EXCLUDED.quantity,
    updated_at = NOW()
RETURNING *;

-- name: CountRecentStandardLikes :one
SELECT COUNT(*) FROM likes
WHERE liker_user_id = $1
  AND interaction_type = 'standard'
  AND created_at >= NOW() - INTERVAL '24 hours';

-- name: GetHomeFeed :many
WITH RequestingUser AS (
    SELECT
        u.id, u.latitude, u.longitude, u.gender, u.date_of_birth, u.spotlight_active_until
    FROM users u WHERE u.id = $1
), RequestingUserFilters AS (
    SELECT
        f.user_id, f.who_you_want_to_see, f.radius_km, f.active_today, f.age_min, f.age_max
    FROM filters f WHERE f.user_id = $1
)
SELECT
    target_user.*,
    haversine(ru.latitude, ru.longitude, target_user.latitude, target_user.longitude) AS distance_km
FROM users AS target_user
JOIN RequestingUser ru ON target_user.id != ru.id
JOIN RequestingUserFilters rf ON ru.id = rf.user_id
LEFT JOIN filters AS target_user_filters ON target_user.id = target_user_filters.user_id
WHERE
    target_user.latitude IS NOT NULL AND target_user.longitude IS NOT NULL
    AND (rf.radius_km IS NULL OR haversine(ru.latitude, ru.longitude, target_user.latitude, target_user.longitude) <= rf.radius_km)
    AND target_user.gender = rf.who_you_want_to_see
    AND (target_user_filters.user_id IS NULL OR target_user_filters.who_you_want_to_see IS NULL OR target_user_filters.who_you_want_to_see = ru.gender)
    AND target_user.date_of_birth IS NOT NULL
    AND EXTRACT(YEAR FROM AGE(target_user.date_of_birth)) BETWEEN rf.age_min AND rf.age_max
    AND (NOT rf.active_today OR EXISTS (
            SELECT 1 FROM app_open_logs aol
            WHERE aol.user_id = target_user.id AND aol.opened_at >= NOW() - INTERVAL '24 hours'
        ))
    AND NOT EXISTS (SELECT 1 FROM dislikes d WHERE d.disliker_user_id = ru.id AND d.disliked_user_id = target_user.id)
    AND NOT EXISTS (SELECT 1 FROM dislikes d WHERE d.disliker_user_id = target_user.id AND d.disliked_user_id = ru.id)
    AND NOT EXISTS (SELECT 1 FROM likes l WHERE l.liker_user_id = ru.id AND l.liked_user_id = target_user.id)
ORDER BY
    CASE WHEN target_user.spotlight_active_until > NOW() THEN 0 ELSE 1 END ASC,
    distance_km ASC,
    ABS(EXTRACT(YEAR FROM AGE(ru.date_of_birth)) - EXTRACT(YEAR FROM AGE(target_user.date_of_birth))) ASC NULLS LAST
LIMIT $2;

-- name: AddUserSubscription :one
INSERT INTO user_subscriptions (
    user_id, feature_type, expires_at, activated_at
) VALUES (
    $1, $2, $3, NOW()
)
RETURNING *;

-- name: AddContentLike :one
-- Inserts a like for a specific content item, potentially with a comment.
-- content_identifier is TEXT type in the DB.
INSERT INTO likes (
    liker_user_id,
    liked_user_id,
    content_type,
    content_identifier,
    comment,
    interaction_type
) VALUES (
    $1, $2, $3, $4, $5, $6
)
-- Prevent duplicate likes on the exact same item
ON CONFLICT (liker_user_id, liked_user_id, content_type, content_identifier)
DO NOTHING
RETURNING *;

-- Removed Check...Exists queries as validation moves to Go code

-- db/queries.sql

-- ... (keep existing queries) ...

-- name: GetLikersForUser :many
-- Fetches basic details of users who liked a specific user, ordered by Rose and then time.
SELECT
    l.liker_user_id,
    l.comment,
    l.interaction_type,
    l.created_at as liked_at,
    u.name,
    u.last_name,
    u.media_urls
FROM likes l
JOIN users u ON l.liker_user_id = u.id
WHERE l.liked_user_id = $1 -- The user receiving the likes (current user)
ORDER BY
    (l.interaction_type = 'rose') DESC, -- Roses first
    l.created_at DESC;                  -- Then by time

-- name: GetLikeDetails :one
-- Fetches specific details of a single like interaction.
SELECT
    comment,
    interaction_type
FROM likes
WHERE liker_user_id = $1 -- The user who sent the like
AND liked_user_id = $2   -- The user who received the like (current user)
LIMIT 1;
