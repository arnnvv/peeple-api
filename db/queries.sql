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

-- name: SetUserOnline :exec
UPDATE users
SET is_online = true
WHERE id = $1;

-- name: SetUserOffline :exec
UPDATE users
SET is_online = false,
    last_online = NOW()
WHERE id = $1;

-- name: GetUserLastOnline :one
SELECT last_online FROM users
WHERE id = $1 LIMIT 1;

-- name: GetUserOnlineStatus :one
SELECT is_online FROM users
WHERE id = $1 LIMIT 1;

-- name: GetPendingVerificationUsers :many
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
), AllPrompts AS (
    SELECT user_id, 'storyTime' as category, question::text, answer FROM story_time_prompts
    UNION ALL
    SELECT user_id, 'myType' as category, question::text, answer FROM my_type_prompts
    UNION ALL
    SELECT user_id, 'gettingPersonal' as category, question::text, answer FROM getting_personal_prompts
    UNION ALL
    SELECT user_id, 'dateVibes' as category, question::text, answer FROM date_vibes_prompts
), AggregatedPrompts AS (
    SELECT
        user_id,
        jsonb_agg(jsonb_build_object('category', category, 'question', question, 'answer', answer)) as prompts
    FROM AllPrompts
    GROUP BY user_id
)
SELECT
    target_user.*,
    COALESCE(ap.prompts, '[]'::jsonb) as prompts,
    haversine(ru.latitude, ru.longitude, target_user.latitude, target_user.longitude) AS distance_km
FROM users AS target_user
JOIN RequestingUser ru ON target_user.id != ru.id
JOIN RequestingUserFilters rf ON ru.id = rf.user_id
LEFT JOIN filters AS target_user_filters ON target_user.id = target_user_filters.user_id
LEFT JOIN AggregatedPrompts ap ON target_user.id = ap.user_id
WHERE
    target_user.latitude IS NOT NULL AND target_user.longitude IS NOT NULL
    AND ru.latitude IS NOT NULL AND ru.longitude IS NOT NULL
    AND ru.gender IS NOT NULL
    AND rf.who_you_want_to_see IS NOT NULL
    AND rf.age_min IS NOT NULL AND rf.age_max IS NOT NULL
    AND (rf.radius_km IS NULL OR haversine(ru.latitude, ru.longitude, target_user.latitude, target_user.longitude) <= rf.radius_km)
    AND target_user.gender = rf.who_you_want_to_see
    AND (target_user_filters.user_id IS NULL OR target_user_filters.who_you_want_to_see IS NULL OR target_user_filters.who_you_want_to_see = ru.gender)
    AND target_user.date_of_birth IS NOT NULL
    AND EXTRACT(YEAR FROM AGE(target_user.date_of_birth)) BETWEEN rf.age_min AND rf.age_max
    AND (NOT rf.active_today OR (
		target_user.last_online IS NOT NULL AND target_user.last_online >= NOW() - INTERVAL '24 hours'
    ))
    AND NOT EXISTS (SELECT 1 FROM dislikes d WHERE d.disliker_user_id = ru.id AND d.disliked_user_id = target_user.id)
    AND NOT EXISTS (SELECT 1 FROM dislikes d WHERE d.disliker_user_id = target_user.id AND d.disliked_user_id = ru.id)
    AND NOT EXISTS (SELECT 1 FROM likes l WHERE l.liker_user_id = ru.id AND l.liked_user_id = target_user.id)
ORDER BY
    CASE WHEN target_user.spotlight_active_until > NOW() THEN 0 ELSE 1 END ASC,
    distance_km ASC,
    CASE WHEN ru.date_of_birth IS NOT NULL THEN
      ABS(EXTRACT(YEAR FROM AGE(ru.date_of_birth)) - EXTRACT(YEAR FROM AGE(target_user.date_of_birth)))
    ELSE
      NULL
    END ASC NULLS LAST
LIMIT $2;

-- name: GetQuickFeed :many
SELECT
    target_user.*,
    haversine($1, $2, target_user.latitude, target_user.longitude) AS distance_km
FROM users AS target_user
WHERE
      target_user.id != $3
  AND target_user.latitude IS NOT NULL
  AND target_user.longitude IS NOT NULL
  AND target_user.gender = $4
  AND target_user.name IS NOT NULL AND target_user.name != ''
  AND target_user.date_of_birth IS NOT NULL
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
    l.id AS like_id,
    l.liker_user_id,
    l.comment,
    l.interaction_type,
    l.is_seen,
    l.created_at as liked_at,
    u.name,
    u.last_name,
    u.media_urls
FROM likes l
JOIN users u ON l.liker_user_id = u.id
WHERE l.liked_user_id = $1
  AND NOT EXISTS (
      SELECT 1
      FROM likes l2
      WHERE l2.liker_user_id = l.liked_user_id
        AND l2.liked_user_id = l.liker_user_id
  )
ORDER BY
    (l.interaction_type = 'rose') DESC,
    l.is_seen ASC,
    l.created_at DESC;

-- name: GetLikeDetails :one
SELECT
    comment,
    interaction_type
FROM likes
WHERE liker_user_id = $1
AND liked_user_id = $2
LIMIT 1;

-- name: CreateChatMessage :one
INSERT INTO chat_messages (
    sender_user_id,
    recipient_user_id,
    message_text,
    media_url,
    media_type,
    reply_to_message_id
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: GetConversationMessages :many
WITH MessageReactionsAgg AS (
    SELECT
        message_id,
        jsonb_object_agg(emoji, count) FILTER (WHERE emoji IS NOT NULL) AS reactions_summary_json
     FROM (
        SELECT message_id, emoji, COUNT(user_id) as count
        FROM message_reactions
        WHERE message_id IN (
            SELECT id FROM chat_messages
            WHERE (sender_user_id = $1 AND recipient_user_id = $2)
               OR (sender_user_id = $2 AND recipient_user_id = $1)
        )
        GROUP BY message_id, emoji
     ) AS grouped_reactions
    GROUP BY message_id
)
SELECT
    cm.id, cm.sender_user_id, cm.recipient_user_id, cm.message_text, cm.media_url, cm.media_type, cm.sent_at, cm.is_read,
    COALESCE(mra.reactions_summary_json, '{}'::jsonb) AS reactions_data,
    cm.reply_to_message_id,
    replied_msg.sender_user_id AS replied_message_sender_id,
    COALESCE(substring(replied_msg.message_text for 50)::TEXT, '') AS replied_message_text_snippet,
    replied_msg.media_type AS replied_message_media_type
FROM chat_messages cm
LEFT JOIN MessageReactionsAgg mra ON cm.id = mra.message_id
LEFT JOIN chat_messages replied_msg ON cm.reply_to_message_id = replied_msg.id
WHERE (cm.sender_user_id = $1 AND cm.recipient_user_id = $2)
   OR (cm.sender_user_id = $2 AND cm.recipient_user_id = $1)
ORDER BY cm.sent_at ASC;

-- name: GetUserReactionsForMessages :many
SELECT message_id, emoji
FROM message_reactions
WHERE user_id = $1 AND message_id = ANY($2::bigint[]);

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

-- name: GetUnseenLikeCount :one
SELECT COUNT(*)
FROM likes l
WHERE l.liked_user_id = $1 -- The user receiving the likes
  AND l.is_seen = false
  AND NOT EXISTS ( -- Ensure it's not a mutual like
      SELECT 1
      FROM likes l2
      WHERE l2.liker_user_id = l.liked_user_id -- The recipient liked the liker back
        AND l2.liked_user_id = l.liker_user_id
  );

-- name: MarkLikesAsSeenUntil :execresult
UPDATE likes
SET is_seen = true
WHERE liked_user_id = $1
  AND id <= $2
  AND is_seen = false;

-- name: CheckLikeExistsForRecipient :one
SELECT EXISTS (
    SELECT 1
    FROM likes
    WHERE id = $1
      AND liked_user_id = $2
);

-- name: UpsertMessageReaction :one
INSERT INTO message_reactions (message_id, user_id, emoji)
VALUES ($1, $2, $3)
ON CONFLICT (message_id, user_id) DO UPDATE SET
    emoji = EXCLUDED.emoji,
    updated_at = NOW()
RETURNING *;

-- name: DeleteMessageReactionByUser :execresult
DELETE FROM message_reactions
WHERE message_id = $1
  AND user_id = $2;

-- name: GetSingleReactionByUser :one
SELECT id, message_id, user_id, emoji, created_at, updated_at
FROM message_reactions
WHERE message_id = $1 AND user_id = $2
LIMIT 1;

-- name: GetMessageSenderRecipient :one
SELECT sender_user_id, recipient_user_id
FROM chat_messages
WHERE id = $1
LIMIT 1;

-- name: MarkChatAsReadOnUnmatch :execresult
UPDATE chat_messages
SET is_read = true
WHERE is_read = false
  AND (
       (recipient_user_id = $1 AND sender_user_id = $2)
    OR (recipient_user_id = $2 AND sender_user_id = $1)
  );

-- name: GetMatchesWithLastEvent :many
SELECT
    target_user.id AS matched_user_id,
    target_user.name AS matched_user_name,
    target_user.last_name AS matched_user_last_name,
    target_user.media_urls AS matched_user_media_urls,
    target_user.is_online AS matched_user_is_online,
    target_user.last_online AS matched_user_last_online,
    last_event.event_at AS last_event_timestamp,
    COALESCE(last_event.event_user_id, 0) AS last_event_user_id,
    COALESCE(last_event.event_type, '') AS last_event_type,
    COALESCE(last_event.event_content, '') AS last_event_content,
    COALESCE(last_event.event_extra, '') AS last_event_extra,
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
    users target_user
    ON l1.liked_user_id = target_user.id
JOIN
    likes l2
    ON l1.liked_user_id = l2.liker_user_id
   AND l1.liker_user_id = l2.liked_user_id
LEFT JOIN LATERAL (
    (
        SELECT
            cm.sent_at AS event_at,
            cm.sender_user_id AS event_user_id,
            CASE
                WHEN cm.message_text IS NOT NULL THEN 'text'
                ELSE 'media'
            END AS event_type,
            COALESCE(cm.message_text, cm.media_type) AS event_content,
            cm.media_url AS event_extra
        FROM chat_messages cm
        WHERE
            (cm.sender_user_id = l1.liker_user_id AND cm.recipient_user_id = l1.liked_user_id)
            OR (cm.sender_user_id = l1.liked_user_id AND cm.recipient_user_id = l1.liker_user_id)

        UNION ALL

        SELECT
            mr.updated_at AS event_at,
            mr.user_id AS event_user_id,
            'reaction' AS event_type,
            mr.emoji AS event_content,
            NULL AS event_extra
        FROM message_reactions mr
        JOIN chat_messages cm_react
            ON mr.message_id = cm_react.id
        WHERE
            (mr.user_id = l1.liker_user_id OR mr.user_id = l1.liked_user_id)
            AND (
                (cm_react.sender_user_id = l1.liker_user_id AND cm_react.recipient_user_id = l1.liked_user_id)
                OR (cm_react.sender_user_id = l1.liked_user_id AND cm_react.recipient_user_id = l1.liker_user_id)
            )
    )
    ORDER BY event_at DESC
    LIMIT 1
) AS last_event ON true
WHERE
    l1.liker_user_id = $1
ORDER BY
    COALESCE(last_event.event_at, '1970-01-01'::timestamptz) DESC,
    target_user.id;

-- name: GetMatchIDs :many
SELECT
  l1.liked_user_id
FROM
  likes l1
JOIN
  likes l2
  ON l1.liker_user_id = l2.liked_user_id
  AND l1.liked_user_id = l2.liker_user_id
WHERE
  l1.liker_user_id = $1;

-- name: UpdateLastOnline :exec
UPDATE users
SET last_online = NOW()
WHERE id = $1;

-- *** ADDED QUERIES FOR LIKE/MATCH NOTIFICATIONS ***

-- name: GetBasicUserInfo :one
-- Fetches minimal user info needed for a "new like" notification payload.
SELECT
    id,
    name,
    last_name,
    media_urls
FROM users
WHERE id = $1 LIMIT 1;

-- name: GetBasicMatchInfo :one
-- Fetches minimal user info needed for a "new match" notification payload.
SELECT
    id,
    name,
    last_name,
    media_urls,
    is_online,
    last_online
FROM users
WHERE id = $1 LIMIT 1;

-- ========================================
--      ANALYTICS QUERIES (CORRECTED)
-- ========================================

-- name: LogUserProfileImpression :exec
-- Logs when a user's profile is shown to another user.
INSERT INTO user_profile_impressions (
    viewer_user_id, shown_user_id, source
) VALUES (
    $1, $2, $3 -- Positional is fine for simple inserts if preferred, but named below for clarity
);

-- name: LogLikeProfileView :exec
-- Logs when a user views a liker's profile from the 'Likes You' screen.
INSERT INTO like_profile_views (
    viewer_user_id, liker_user_id, like_id
) VALUES (
    $1, $2, $3
);

-- name: CountProfileImpressions :one
-- Counts how many times a user's profile was shown within a date range.
SELECT COUNT(*)
FROM user_profile_impressions
WHERE shown_user_id = @shown_user_id -- Use named param
  AND (@start_date::timestamptz IS NULL OR impression_timestamp >= @start_date) -- Start date (inclusive)
  AND (@end_date::timestamptz IS NULL OR impression_timestamp <= @end_date); -- End date (inclusive)


-- name: GetApproximateProfileViewTimeSeconds :one
-- Calculates the approximate average profile view time in seconds, based on interaction intervals.
-- Note: This is an approximation and excludes views without interactions.
WITH UserInteractions AS (
    -- Combine likes and dislikes where the target user was interacted with
    SELECT
        l.liker_user_id AS viewer_user_id,
        l.liked_user_id AS interacted_user_id,
        l.created_at
    FROM likes l
    WHERE l.liked_user_id = @target_user_id -- Use named param for the user whose profile view time we want
      AND (@start_date::timestamptz IS NULL OR l.created_at >= @start_date)
      AND (@end_date::timestamptz IS NULL OR l.created_at <= @end_date)
    UNION ALL
    SELECT
        d.disliker_user_id AS viewer_user_id,
        d.disliked_user_id AS interacted_user_id,
        d.created_at
    FROM dislikes d
    WHERE d.disliked_user_id = @target_user_id -- Use named param here too
      AND (@start_date::timestamptz IS NULL OR d.created_at >= @start_date)
      AND (@end_date::timestamptz IS NULL OR d.created_at <= @end_date)
),
InteractionIntervals AS (
    SELECT
        created_at,
        LAG(created_at, 1) OVER (PARTITION BY viewer_user_id ORDER BY created_at ASC) as prev_created_at
    FROM UserInteractions
)
SELECT
    COALESCE(AVG(
        LEAST( -- Apply 60-second cap
            EXTRACT(EPOCH FROM (created_at - prev_created_at)), -- Duration in seconds
            60.0
        )
    ), 0.0)::float -- Return 0.0 if no intervals found
FROM InteractionIntervals
WHERE prev_created_at IS NOT NULL; -- Only consider intervals where a previous interaction exists


-- name: CountDislikesSent :one
-- Counts dislikes sent by the user within a date range.
SELECT COUNT(*)
FROM dislikes
WHERE disliker_user_id = @disliker_user_id -- Use named param
  AND (@start_date::timestamptz IS NULL OR created_at >= @start_date)
  AND (@end_date::timestamptz IS NULL OR created_at <= @end_date);

-- name: CountDislikesReceived :one
-- Counts dislikes received by the user within a date range.
SELECT COUNT(*)
FROM dislikes
WHERE disliked_user_id = @disliked_user_id -- Use named param
  AND (@start_date::timestamptz IS NULL OR created_at >= @start_date)
  AND (@end_date::timestamptz IS NULL OR created_at <= @end_date);

-- name: CountProfilesOpenedFromLike :one
-- Counts how many times profiles liked by the user were opened from the 'Likes You' screen.
SELECT COUNT(*)
FROM like_profile_views
WHERE liker_user_id = @liker_user_id -- Use named param
  AND (@start_date::timestamptz IS NULL OR view_timestamp >= @start_date)
  AND (@end_date::timestamptz IS NULL OR view_timestamp <= @end_date);


-- name: CountImpressionsDuringSpotlight :one
-- Counts profile impressions specifically from spotlight source within a date range.
SELECT COUNT(*)
FROM user_profile_impressions
WHERE shown_user_id = @shown_user_id -- Use named param
  AND source = 'spotlight'
  AND (@start_date::timestamptz IS NULL OR impression_timestamp >= @start_date)
  AND (@end_date::timestamptz IS NULL OR impression_timestamp <= @end_date);

-- name: GetUserSpotlightActivationTimes :many
-- Fetches the activation time (approximated by updated_at) and expiry time for spotlight consumables.
SELECT
    updated_at as potentially_activated_at,
    u.spotlight_active_until as expires_at
FROM user_consumables uc
JOIN users u ON uc.user_id = u.id
WHERE uc.user_id = @user_id -- Use named param
  AND uc.consumable_type = 'spotlight'
  AND u.spotlight_active_until IS NOT NULL
  AND (
       (@start_date::timestamptz IS NULL OR u.spotlight_active_until >= @start_date)
       AND
       (@end_date::timestamptz IS NULL OR uc.updated_at <= @end_date)
      )
ORDER BY u.spotlight_active_until DESC;


-- ========================================
--      PHOTO VIEW DURATION QUERIES
-- ========================================

-- name: LogPhotoViewDuration :exec
-- Logs a single instance of a photo being viewed for a specific duration.
INSERT INTO photo_view_durations (
    viewer_user_id, viewed_user_id, photo_index, duration_ms
) VALUES (
    $1, $2, $3, $4
);

-- name: GetPhotoAverageViewDurations :many
-- Calculates the average view duration in milliseconds for each photo index
-- of a specific user, optionally filtered by a date range.
SELECT
    photo_index,
    COALESCE(AVG(duration_ms), 0)::float AS average_duration_ms -- Return 0 if no views for that index
FROM photo_view_durations
WHERE
    viewed_user_id = @viewed_user_id -- Use named param for the user whose photos were viewed
    AND (@start_date::timestamptz IS NULL OR view_timestamp >= @start_date)
    AND (@end_date::timestamptz IS NULL OR view_timestamp <= @end_date)
GROUP BY photo_index
ORDER BY photo_index;
