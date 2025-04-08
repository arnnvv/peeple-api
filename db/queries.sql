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

-- =============================================
-- START: Queries for Like/Dislike & Premium
-- =============================================

-- name: AddLike :exec
-- Modified to include interaction_type
INSERT INTO likes (liker_user_id, liked_user_id, interaction_type)
VALUES ($1, $2, $3) -- $3 is the like_interaction_type ('standard' or 'rose')
ON CONFLICT (liker_user_id, liked_user_id) DO NOTHING; -- Ignore if like already exists

-- name: AddDislike :exec
INSERT INTO dislikes (disliker_user_id, disliked_user_id)
VALUES ($1, $2)
ON CONFLICT (disliker_user_id, disliked_user_id) DO NOTHING;

-- name: GetActiveSubscription :one
-- Checks if a user has a specific *active* subscription
SELECT * FROM user_subscriptions -- Select all columns for the generated struct
WHERE user_id = $1
  AND feature_type = $2 -- e.g., 'unlimited_likes'
  AND expires_at > NOW()
LIMIT 1;

-- name: GetUserConsumable :one
-- Gets the current quantity of a specific consumable for a user
SELECT * FROM user_consumables -- Select all columns for the generated struct
WHERE user_id = $1
  AND consumable_type = $2; -- e.g., 'rose'

-- name: DecrementUserConsumable :one
-- Decrements the quantity of a consumable if available, returns the updated record
UPDATE user_consumables
SET quantity = quantity - 1,
    updated_at = NOW()
WHERE user_id = $1
  AND consumable_type = $2 -- e.g., 'rose'
  AND quantity > 0
RETURNING *;

-- name: UpsertUserConsumable :one
-- Adds quantity to a user's consumable balance (used when purchasing)
INSERT INTO user_consumables (user_id, consumable_type, quantity)
VALUES ($1, $2, $3) -- $3 is the quantity to add
ON CONFLICT (user_id, consumable_type) DO UPDATE
SET quantity = user_consumables.quantity + EXCLUDED.quantity,
    updated_at = NOW()
RETURNING *;

-- name: CountRecentStandardLikes :one
-- Counts standard likes sent by a user in the last 24 hours
SELECT COUNT(*) FROM likes
WHERE liker_user_id = $1
  AND interaction_type = 'standard' -- Only count standard likes
  AND created_at >= NOW() - INTERVAL '24 hours';


-- name: GetHomeFeed :many
-- Modified ORDER BY clause to potentially include spotlight later
WITH RequestingUser AS (
    SELECT
        u.id,
        u.latitude,
        u.longitude,
        u.gender,
        u.date_of_birth,
        u.spotlight_active_until -- Added for sorting checks
    FROM users u
    WHERE u.id = $1 -- requesting_user_id
), RequestingUserFilters AS (
    SELECT
        f.user_id,
        f.who_you_want_to_see,
        f.radius_km,
        f.active_today,
        f.age_min,
        f.age_max
    FROM filters f
    WHERE f.user_id = $1 -- requesting_user_id
)
SELECT
    -- Explicitly list columns from users to avoid ambiguity if joins added later
    target_user.id, target_user.created_at, target_user.name, target_user.last_name,
    target_user.phone_number, target_user.date_of_birth, target_user.latitude,
    target_user.longitude, target_user.gender, target_user.dating_intention, target_user.height,
    target_user.hometown, target_user.job_title, target_user.education,
    target_user.religious_beliefs, target_user.drinking_habit, target_user.smoking_habit,
    target_user.media_urls, target_user.verification_status, target_user.verification_pic,
    target_user.role, target_user.audio_prompt_question, target_user.audio_prompt_answer,
    target_user.spotlight_active_until, -- Include spotlight status
    -- Calculated distance
    haversine(ru.latitude, ru.longitude, target_user.latitude, target_user.longitude) AS distance_km
FROM users AS target_user
JOIN RequestingUser ru ON target_user.id != ru.id -- Don't show self
JOIN RequestingUserFilters rf ON ru.id = rf.user_id -- Ensure requesting user has filters (or defaults were just created)
LEFT JOIN filters AS target_user_filters ON target_user.id = target_user_filters.user_id -- Filters of the potential match
WHERE
    -- Location Check: target user must have location
    target_user.latitude IS NOT NULL AND target_user.longitude IS NOT NULL
    -- Radius Check: target user must be within radius OR radius filter is not set
    AND (rf.radius_km IS NULL OR haversine(ru.latitude, ru.longitude, target_user.latitude, target_user.longitude) <= rf.radius_km)

    -- Gender Preference Check (Requesting User's Preference)
    AND target_user.gender = rf.who_you_want_to_see

    -- Gender Preference Check (Target User's Preference - Reciprocal)
    -- Make reciprocal check optional or handle NULL target filters gracefully
    AND (target_user_filters.user_id IS NULL OR target_user_filters.who_you_want_to_see IS NULL OR target_user_filters.who_you_want_to_see = ru.gender)

    -- Age Check: target user's age must be within requesting user's filter range
    AND target_user.date_of_birth IS NOT NULL
    AND EXTRACT(YEAR FROM AGE(target_user.date_of_birth)) BETWEEN rf.age_min AND rf.age_max

    -- Active Today Check (only if filter is enabled)
    AND (NOT rf.active_today OR EXISTS (
            SELECT 1 FROM app_open_logs aol
            WHERE aol.user_id = target_user.id AND aol.opened_at >= NOW() - INTERVAL '24 hours'
        ))

    -- Dislike Checks (Neither user disliked the other)
    AND NOT EXISTS (SELECT 1 FROM dislikes d WHERE d.disliker_user_id = ru.id AND d.disliked_user_id = target_user.id)
    AND NOT EXISTS (SELECT 1 FROM dislikes d WHERE d.disliker_user_id = target_user.id AND d.disliked_user_id = ru.id)

    -- Like Check (Requesting user has NOT liked the target user)
    AND NOT EXISTS (SELECT 1 FROM likes l WHERE l.liker_user_id = ru.id AND l.liked_user_id = target_user.id) -- Unchanged

ORDER BY
    -- Spotlight Boost: Users with active spotlight appear first
    CASE WHEN target_user.spotlight_active_until > NOW() THEN 0 ELSE 1 END ASC,
    -- Then by distance
    distance_km ASC,
    -- Then by age difference
    ABS(EXTRACT(YEAR FROM AGE(ru.date_of_birth)) - EXTRACT(YEAR FROM AGE(target_user.date_of_birth))) ASC NULLS LAST
LIMIT $2; -- page_size (e.g., 15)


-- =============================================
-- END: Queries for Like/Dislike & Premium
-- =============================================

-- (Add this query to the end of your existing queries.sql file)

-- name: AddUserSubscription :one
-- Adds a new subscription record for a user.
-- Assumes basic insertion; complex logic (like extending existing subs) would need more checks.
INSERT INTO user_subscriptions (
    user_id, feature_type, expires_at, activated_at -- Use default for created_at
) VALUES (
    $1, $2, $3, NOW() -- activated_at is set to now
)
RETURNING *;
