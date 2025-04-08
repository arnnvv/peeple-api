-- =============================================
-- START: Original Schema Definitions
-- =============================================

-- Original ENUM Types
CREATE TYPE gender_enum AS ENUM (
    'man',
    'woman',
    'gay',
    'lesbian',
    'bisexual'
);
COMMENT ON TYPE gender_enum IS 'Enumerated type for representing gender identity and/or sexual orientation as specified.';

CREATE TYPE dating_intention AS ENUM (
    'lifePartner',
    'longTerm',
    'longTermOpenShort',
    'shortTermOpenLong',
    'shortTerm',
    'figuringOut'
);

CREATE TYPE religion AS ENUM (
    'agnostic',
    'atheist',
    'buddhist',
    'christian',
    'hindu',
    'jain',
    'jewish',
    'muslim',
    'zoroastrian',
    'sikh',
    'spiritual'
);

CREATE TYPE drinking_smoking_habits AS ENUM (
    'yes',
    'sometimes',
    'no'
);

CREATE TYPE story_time_prompt_type AS ENUM (
    'twoTruthsAndALie',
    'worstIdea',
    'biggestRisk',
    'biggestDateFail',
    'neverHaveIEver',
    'bestTravelStory',
    'weirdestGift',
    'mostSpontaneous',
    'oneThingNeverDoAgain'
);

CREATE TYPE my_type_prompt_type AS ENUM (
    'nonNegotiable',
    'hallmarkOfGoodRelationship',
    'lookingFor',
    'weirdlyAttractedTo',
    'allIAskIsThatYou',
    'wellGetAlongIf',
    'wantSomeoneWho',
    'greenFlags',
    'sameTypeOfWeird',
    'fallForYouIf',
    'bragAboutYou'
);

CREATE TYPE getting_personal_prompt_type AS ENUM (
    'oneThingYouShouldKnow',
    'loveLanguage',
    'dorkiestThing',
    'dontHateMeIf',
    'geekOutOn',
    'ifLovingThisIsWrong',
    'keyToMyHeart',
    'wontShutUpAbout',
    'shouldNotGoOutWithMeIf',
    'whatIfIToldYouThat'
);

CREATE TYPE date_vibes_prompt_type AS ENUM (
    'togetherWeCould',
    'firstRoundIsOnMeIf',
    'whatIOrderForTheTable',
    'bestSpotInTown',
    'bestWayToAskMeOut'
);

CREATE TYPE audio_prompt AS ENUM (
    'canWeTalkAbout', 'captionThisPhoto', 'caughtInTheAct', 'changeMyMindAbout',
    'chooseOurFirstDate', 'commentIfYouveBeenHere', 'cookWithMe', 'datingMeIsLike',
    'datingMeWillLookLike', 'doYouAgreeOrDisagreeThat', 'dontHateMeIfI', 'dontJudgeMe',
    'mondaysAmIRight', 'aBoundaryOfMineIs', 'aDailyEssential', 'aDreamHomeMustInclude',
    'aFavouriteMemoryOfMine', 'aFriendsReviewOfMe', 'aLifeGoalOfMine', 'aQuickRantAbout',
    'aRandomFactILoveIs', 'aSpecialTalentOfMine', 'aThoughtIRecentlyHadInTheShower',
    'allIAskIsThatYou', 'guessWhereThisPhotoWasTaken', 'helpMeIdentifyThisPhotoBomber',
    'hiFromMeAndMyPet', 'howIFightTheSundayScaries', 'howHistoryWillRememberMe',
    'howMyFriendsSeeMe', 'howToPronounceMyName', 'iBeatMyBluesBy', 'iBetYouCant',
    'iCanTeachYouHowTo', 'iFeelFamousWhen', 'iFeelMostSupportedWhen'
);

CREATE TYPE verification_status AS ENUM (
    'false',
    'true',
    'pending'
);

CREATE TYPE user_role AS ENUM (
    'user',
    'admin'
);

-- Original TABLE Definitions
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    name TEXT,
    last_name TEXT,
    phone_number TEXT NOT NULL UNIQUE,
    date_of_birth DATE,
    latitude DOUBLE PRECISION,
    longitude DOUBLE PRECISION,
    gender gender_enum,
    dating_intention dating_intention,
    height DOUBLE PRECISION,
    hometown TEXT,
    job_title TEXT,
    education TEXT,
    religious_beliefs religion,
    drinking_habit drinking_smoking_habits,
    smoking_habit drinking_smoking_habits,
    media_urls TEXT[],
    verification_status verification_status NOT NULL DEFAULT 'false',
    verification_pic TEXT,
    role user_role NOT NULL DEFAULT 'user',
    audio_prompt_question audio_prompt,
    audio_prompt_answer TEXT
);

CREATE TABLE otps (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    otp_code VARCHAR(6) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '2 minute')
);

CREATE TABLE story_time_prompts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    question story_time_prompt_type NOT NULL,
    answer TEXT NOT NULL,
    CONSTRAINT uq_user_story_time_prompt UNIQUE (user_id, question)
);

CREATE TABLE my_type_prompts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    question my_type_prompt_type NOT NULL,
    answer TEXT NOT NULL,
    CONSTRAINT uq_user_my_type_prompt UNIQUE (user_id, question)
);

CREATE TABLE getting_personal_prompts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    question getting_personal_prompt_type NOT NULL,
    answer TEXT NOT NULL,
    CONSTRAINT uq_user_getting_personal_prompt UNIQUE (user_id, question)
);

CREATE TABLE date_vibes_prompts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    question date_vibes_prompt_type NOT NULL,
    answer TEXT NOT NULL,
    CONSTRAINT uq_user_date_vibes_prompts UNIQUE (user_id, question)
);

-- Original Function and Trigger Definitions
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
   NEW.updated_at = NOW();
   RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TABLE filters (
    user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    who_you_want_to_see gender_enum,
    radius_km INTEGER CHECK (radius_km > 0 AND radius_km <= 500),
    active_today BOOLEAN NOT NULL DEFAULT false,
    age_min INTEGER CHECK (age_min >= 18),
    age_max INTEGER CHECK (age_max >= 18),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT age_check CHECK (age_max >= age_min)
);
COMMENT ON COLUMN filters.who_you_want_to_see IS 'Which gender the user wants to see in their feed.';
COMMENT ON COLUMN filters.radius_km IS 'Maximum distance in kilometers for potential matches.';
COMMENT ON COLUMN filters.active_today IS 'Filter for users active within the last 24 hours.';
COMMENT ON COLUMN filters.age_min IS 'Minimum age preference.';
COMMENT ON COLUMN filters.age_max IS 'Maximum age preference.';

CREATE TRIGGER set_timestamp
BEFORE UPDATE ON filters
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE app_open_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    opened_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
COMMENT ON TABLE app_open_logs IS 'Logs each time a user is considered to have opened the app (triggered by a specific API call).';
COMMENT ON COLUMN app_open_logs.id IS 'Unique identifier for the log entry.';
COMMENT ON COLUMN app_open_logs.user_id IS 'The ID of the user who opened the app.';
COMMENT ON COLUMN app_open_logs.opened_at IS 'The timestamp when the app open event was recorded.';
CREATE INDEX idx_app_open_logs_user_time ON app_open_logs (user_id, opened_at DESC);


CREATE OR REPLACE FUNCTION haversine(lat1 float, lon1 float, lat2 float, lon2 float)
RETURNS float AS $$
DECLARE
    radius float := 6371; -- Earth radius in kilometers
    delta_lat float; delta_lon float; a float; c float; d float;
BEGIN
    delta_lat := RADIANS(lat2 - lat1); delta_lon := RADIANS(lon2 - lon1);
    a := SIN(delta_lat / 2) * SIN(delta_lat / 2) + COS(RADIANS(lat1)) * COS(RADIANS(lat2)) * SIN(delta_lon / 2) * SIN(delta_lon / 2);
    c := 2 * ASIN(SQRT(a)); d := radius * c;
    RETURN d;
END;
$$ LANGUAGE plpgsql IMMUTABLE;
COMMENT ON FUNCTION haversine(float, float, float, float) IS 'Calculates the great-circle distance between two points (latitude/longitude) in kilometers using the Haversine formula.';

CREATE TABLE dislikes (
    disliker_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    disliked_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (disliker_user_id, disliked_user_id)
);
COMMENT ON TABLE dislikes IS 'Stores records of users disliking other users.';
COMMENT ON COLUMN dislikes.disliker_user_id IS 'The ID of the user performing the dislike action.';
COMMENT ON COLUMN dislikes.disliked_user_id IS 'The ID of the user being disliked.';
COMMENT ON COLUMN dislikes.created_at IS 'Timestamp when the dislike occurred.';
CREATE INDEX idx_dislikes_disliked_user ON dislikes (disliked_user_id);

CREATE TABLE likes (
    liker_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    liked_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (liker_user_id, liked_user_id)
);
COMMENT ON TABLE likes IS 'Stores records of users liking other users.';
COMMENT ON COLUMN likes.liker_user_id IS 'The ID of the user performing the like action.';
COMMENT ON COLUMN likes.liked_user_id IS 'The ID of the user being liked.';
COMMENT ON COLUMN likes.created_at IS 'Timestamp when the like occurred.';
CREATE INDEX idx_likes_liked_user ON likes (liked_user_id);

-- =============================================
-- END: Original Schema Definitions
-- =============================================


-- =============================================
-- START: Additions for Premium Features
-- =============================================

-- Enum for distinct premium feature types
CREATE TYPE premium_feature_type AS ENUM (
    'unlimited_likes',
    'travel_mode',
    'rose',
    'spotlight'
);
COMMENT ON TYPE premium_feature_type IS 'Defines the types of premium features available.';

-- Enum for like interaction types
CREATE TYPE like_interaction_type AS ENUM ('standard', 'rose');
COMMENT ON TYPE like_interaction_type IS 'Distinguishes standard likes from premium interactions like Roses.';

-- Table for active time-based subscriptions
CREATE TABLE user_subscriptions (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    feature_type premium_feature_type NOT NULL,
    activated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Ensure this table only stores time-based features
    CHECK (feature_type IN ('unlimited_likes', 'travel_mode'))
);
CREATE INDEX idx_user_subscriptions_user_expires ON user_subscriptions (user_id, feature_type, expires_at);
COMMENT ON TABLE user_subscriptions IS 'Tracks active time-based premium features for users.';
COMMENT ON COLUMN user_subscriptions.expires_at IS 'Timestamp when the subscription benefit ends.';

-- Table for consumable item balances
CREATE TABLE user_consumables (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    consumable_type premium_feature_type NOT NULL,
    quantity INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), -- Track last modification

    PRIMARY KEY (user_id, consumable_type), -- One row per user per consumable type
    -- Ensure this table only stores quantity-based features
    CHECK (consumable_type IN ('rose', 'spotlight')),
    CHECK (quantity >= 0) -- Ensure balance doesn't go negative
);
COMMENT ON TABLE user_consumables IS 'Tracks the balance of quantity-based premium items (Roses, Spotlights) for users.';
COMMENT ON COLUMN user_consumables.quantity IS 'The number of remaining items the user possesses.';

-- Modify the existing 'likes' table **AFTER** it's created
ALTER TABLE likes
ADD COLUMN interaction_type like_interaction_type NOT NULL DEFAULT 'standard';

-- Add an index for the new column
CREATE INDEX idx_likes_interaction_type ON likes (interaction_type);
COMMENT ON COLUMN likes.interaction_type IS 'Distinguishes standard likes from premium interactions like Roses.';

-- Modify the existing 'users' table **AFTER** it's created
ALTER TABLE users
ADD COLUMN spotlight_active_until TIMESTAMPTZ NULL;

CREATE INDEX idx_users_spotlight_active ON users (spotlight_active_until) WHERE spotlight_active_until IS NOT NULL;
COMMENT ON COLUMN users.spotlight_active_until IS 'Timestamp until which the user''s profile is boosted by Spotlight.';

-- =============================================
-- END: Additions for Premium Features
-- =============================================
