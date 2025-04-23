-- FILE: db/queries.sql (Resolved)

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1 LIMIT 1;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1 LIMIT 1;

-- name: CreateUserWithEmail :one
-- Creates a new user with only their email address initially.
INSERT INTO users (
    email
) VALUES (
    $1
)
RETURNING *;

-- name: GetPendingVerificationUsers :many
-- Fetches users whose verification status is 'pending'.
SELECT * FROM users
WHERE verification_status = $1; -- $1 should be 'pending'

-- name: UpdateUserProfile :one
-- Updates the main profile details, EXCLUDING location and gender.
-- Parameter indexes adjusted after removing location/gender params.
UPDATE users SET
    name = $1,                -- param $1
    last_name = $2,           -- param $2
    date_of_birth = $3,       -- param $3
    dating_intention = $4,    -- param $4
    height = $5,              -- param $5
    hometown = $6,            -- param $6
    job_title = $7,           -- param $7
    education = $8,           -- param $8
    religious_beliefs = $9,   -- param $9
    drinking_habit = $10,     -- param $10
    smoking_habit = $11       -- param $11
WHERE id = $12                -- param $12
RETURNING *;

-- name: UpdateUserLocationGender :one
-- Updates only the user's latitude, longitude, and gender.
UPDATE users SET
    latitude = $1,
    longitude = $2,
    gender = $3
WHERE id = $4
RETURNING *;

-- name: UpdateUserRole :one
-- Updates the user's role (e.g., to 'admin' or 'user').
UPDATE users
SET role = $1
WHERE id = $2
RETURNING *;

-- name: ClearUserMediaURLs :exec
-- Removes all media URLs for a user.
UPDATE users
SET media_urls = '{}'
WHERE id = $1;

-- name: UpdateUserMediaURLs :exec
-- Sets the media URLs for a user, replacing any existing ones.
UPDATE users
SET media_urls = $1
WHERE id = $2;

-- name: UpdateUserVerificationStatus :one
-- Updates the verification status ('true', 'false', 'pending').
UPDATE users
SET verification_status = $1
WHERE id = $2
RETURNING *;

-- name: UpdateUserVerificationDetails :one
-- Updates the verification picture URL and status together.
UPDATE users
SET
    verification_pic = $1,
    verification_status = $2
WHERE id = $3
RETURNING *;

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
-- Retrieves the main feed based on user filters and location.
-- *** MODIFIED TO INCLUDE PROMPTS ***
WITH RequestingUser AS (
    SELECT
        u.id, u.latitude, u.longitude, u.gender, u.date_of_birth, u.spotlight_active_until
    FROM users u WHERE u.id = $1
), RequestingUserFilters AS (
    SELECT
        f.user_id, f.who_you_want_to_see, f.radius_km, f.active_today, f.age_min, f.age_max
    FROM filters f WHERE f.user_id = $1
), AllPrompts AS (
    -- Combine all prompts for all users into a single structure with category
    SELECT user_id, 'storyTime' as category, question::text, answer FROM story_time_prompts
    UNION ALL
    SELECT user_id, 'myType' as category, question::text, answer FROM my_type_prompts
    UNION ALL
    SELECT user_id, 'gettingPersonal' as category, question::text, answer FROM getting_personal_prompts
    UNION ALL
    SELECT user_id, 'dateVibes' as category, question::text, answer FROM date_vibes_prompts
), AggregatedPrompts AS (
    -- Aggregate combined prompts into a JSONB array for each user
    SELECT
        user_id,
        jsonb_agg(jsonb_build_object('category', category, 'question', question, 'answer', answer)) as prompts
    FROM AllPrompts
    GROUP BY user_id
)
SELECT
    target_user.*, -- Select all columns from the users table
    -- Aggregate prompts for the target user, default to empty array if none
    COALESCE(ap.prompts, '[]'::jsonb) as prompts,
    -- Calculate distance
    haversine(ru.latitude, ru.longitude, target_user.latitude, target_user.longitude) AS distance_km
FROM users AS target_user
JOIN RequestingUser ru ON target_user.id != ru.id
JOIN RequestingUserFilters rf ON ru.id = rf.user_id
LEFT JOIN filters AS target_user_filters ON target_user.id = target_user_filters.user_id
LEFT JOIN AggregatedPrompts ap ON target_user.id = ap.user_id -- Join with aggregated prompts
WHERE
    target_user.latitude IS NOT NULL AND target_user.longitude IS NOT NULL -- Ensure target has location
    AND ru.latitude IS NOT NULL AND ru.longitude IS NOT NULL             -- Ensure requesting user has location
    AND ru.gender IS NOT NULL                                             -- Ensure requesting user has gender set
    AND rf.who_you_want_to_see IS NOT NULL                                -- Ensure filter preference is set
    AND rf.age_min IS NOT NULL AND rf.age_max IS NOT NULL                 -- Ensure age filters are set
    AND (rf.radius_km IS NULL OR haversine(ru.latitude, ru.longitude, target_user.latitude, target_user.longitude) <= rf.radius_km)
    AND target_user.gender = rf.who_you_want_to_see -- Target gender matches filter
    AND (target_user_filters.user_id IS NULL OR target_user_filters.who_you_want_to_see IS NULL OR target_user_filters.who_you_want_to_see = ru.gender) -- Target preferences allow requesting user's gender
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
    CASE WHEN target_user.spotlight_active_until > NOW() THEN 0 ELSE 1 END ASC, -- Spotlight users first
    distance_km ASC,
    -- Use age difference calculation only if requesting user DOB is set
    CASE WHEN ru.date_of_birth IS NOT NULL THEN
      ABS(EXTRACT(YEAR FROM AGE(ru.date_of_birth)) - EXTRACT(YEAR FROM AGE(target_user.date_of_birth)))
    ELSE
      NULL -- Or some other default ordering if DOB is missing
    END ASC NULLS LAST
LIMIT $2; -- Param $2 is the feed batch size (e.g., 15)
-- name: GetQuickFeed :many
-- Retrieves a small feed based purely on proximity and opposite gender.
-- Parameters: $1=latitude, $2=longitude (of requesting user), $3=requesting_user_id, $4=gender_to_show, $5=limit
SELECT
    target_user.*,
    haversine($1, $2, target_user.latitude, target_user.longitude) AS distance_km
FROM users AS target_user
WHERE
      target_user.id != $3 -- Exclude self
  AND target_user.latitude IS NOT NULL
  AND target_user.longitude IS NOT NULL
  AND target_user.gender = $4 -- Filter by the OPPOSITE gender passed as param
  AND target_user.name IS NOT NULL AND target_user.name != '' -- Basic profile check
  AND target_user.date_of_birth IS NOT NULL                  -- Basic profile check
ORDER BY
    distance_km ASC
LIMIT $5;

-- name: AddUserSubscription :one
INSERT INTO user_subscriptions (
    user_id, feature_type, expires_at, activated_at
) VALUES (
    $1, $2, $3, NOW()
)
RETURNING *;

-- name: AddContentLike :one
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
ON CONFLICT (liker_user_id, liked_user_id, content_type, content_identifier)
DO NOTHING
RETURNING *;

-- name: GetLikersForUser :many
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
SELECT
    comment,
    interaction_type
FROM likes
WHERE liker_user_id = $1 -- The user who sent the like
AND liked_user_id = $2   -- The user who received the like (current user)
LIMIT 1;


-- Chat Queries (from partner branch) --
-- name: CreateChatMessage :one
INSERT INTO chat_messages (
    sender_user_id,
    recipient_user_id,
    message_text,
    media_url,
    media_type
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING *;

-- name: GetConversationMessages :many
SELECT * FROM chat_messages
WHERE (sender_user_id = $1 AND recipient_user_id = $2)
   OR (sender_user_id = $2 AND recipient_user_id = $1)
ORDER BY sent_at ASC;

-- name: MarkMessagesAsReadUntil :execresult
UPDATE chat_messages
SET is_read = true
WHERE recipient_user_id = $1
  AND sender_user_id = $2
  AND id <= $3
  AND is_read = false;

-- name: CheckMutualLikeExists :one
SELECT EXISTS (SELECT 1 FROM likes l1 WHERE l1.liker_user_id = $1 AND l1.liked_user_id = $2)
   AND EXISTS (SELECT 1 FROM likes l2 WHERE l2.liker_user_id = $2 AND l2.liked_user_id = $1);

-- name: DeleteLikesBetweenUsers :exec
DELETE FROM likes
WHERE (liker_user_id = $1 AND liked_user_id = $2)
   OR (liker_user_id = $2 AND liked_user_id = $1);

-- name: CreateReport :one
INSERT INTO reports (
    reporter_user_id,
    reported_user_id,
    reason
) VALUES (
    $1, $2, $3
)
RETURNING id, reporter_user_id, reported_user_id, reason, created_at;

-- name: GetMatchesWithLastMessage :many
SELECT
    target_user.id AS matched_user_id,
    target_user.name AS matched_user_name,
    target_user.last_name AS matched_user_last_name,
    target_user.media_urls AS matched_user_media_urls,
    COALESCE(last_msg.message_text, '') AS last_message_text,
    last_msg.media_type AS last_message_media_type,
    last_msg.media_url AS last_message_media_url,
    last_msg.sent_at AS last_message_sent_at,
    COALESCE(last_msg.sender_user_id, 0) AS last_message_sender_id,
    (
        SELECT COUNT(*)
        FROM chat_messages cm_unread
        WHERE cm_unread.recipient_user_id = l1.liker_user_id
          AND cm_unread.sender_user_id = l1.liked_user_id
          AND cm_unread.is_read = false
    ) AS unread_message_count
FROM
    likes l1
JOIN
    users target_user ON l1.liked_user_id = target_user.id
JOIN
    likes l2 ON l1.liked_user_id = l2.liker_user_id AND l1.liker_user_id = l2.liked_user_id
LEFT JOIN LATERAL (
    SELECT
        cm.message_text,
        cm.media_type,
        cm.media_url,
        cm.sent_at,
        cm.sender_user_id
    FROM
        chat_messages cm
    WHERE
        (cm.sender_user_id = l1.liker_user_id AND cm.recipient_user_id = l1.liked_user_id)
        OR (cm.sender_user_id = l1.liked_user_id AND cm.recipient_user_id = l1.liker_user_id)
    ORDER BY
        cm.sent_at DESC
    LIMIT 1
) last_msg ON true
WHERE
    l1.liker_user_id = $1
ORDER BY
    last_msg.sent_at DESC NULLS LAST,
    target_user.id;

-- name: CheckLikeExists :one
SELECT EXISTS (
    SELECT 1 FROM likes
    WHERE liker_user_id = $1 AND liked_user_id = $2
);

-- name: GetTotalUnreadCount :one
SELECT COUNT(*)
FROM chat_messages
WHERE recipient_user_id = $1
  AND is_read = false;
