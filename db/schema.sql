CREATE TYPE gender_enum AS ENUM (
    'man',
    'woman'
);

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
    'twoTruthsAndALie', 'worstIdea', 'biggestRisk', 'biggestDateFail',
    'neverHaveIEver', 'bestTravelStory', 'weirdestGift', 'mostSpontaneous',
    'oneThingNeverDoAgain'
);

CREATE TYPE my_type_prompt_type AS ENUM (
    'nonNegotiable', 'hallmarkOfGoodRelationship', 'lookingFor', 'weirdlyAttractedTo',
    'allIAskIsThatYou', 'wellGetAlongIf', 'wantSomeoneWho', 'greenFlags',
    'sameTypeOfWeird', 'fallForYouIf', 'bragAboutYou'
);

CREATE TYPE getting_personal_prompt_type AS ENUM (
    'oneThingYouShouldKnow', 'loveLanguage', 'dorkiestThing', 'dontHateMeIf',
    'geekOutOn', 'ifLovingThisIsWrong', 'keyToMyHeart', 'wontShutUpAbout',
    'shouldNotGoOutWithMeIf', 'whatIfIToldYouThat'
);

CREATE TYPE date_vibes_prompt_type AS ENUM (
    'togetherWeCould', 'firstRoundIsOnMeIf', 'whatIOrderForTheTable',
    'bestSpotInTown', 'bestWayToAskMeOut'
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

CREATE TYPE premium_feature_type AS ENUM (
    'unlimited_likes',
    'travel_mode',
    'rose',
    'spotlight'
);

CREATE TYPE like_interaction_type AS ENUM ('standard', 'rose');

CREATE TYPE content_like_type AS ENUM (
    'media',
    'prompt_story',
    'prompt_mytype',
    'prompt_gettingpersonal',
    'prompt_datevibes',
    'audio_prompt',
    'profile'
);

CREATE TYPE report_reason AS ENUM (
    'notInterested',
    'fakeProfile',
    'inappropriate',
    'minor',
    'spam'
);

CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    name TEXT,
    last_name TEXT,
    email TEXT UNIQUE NOT NULL,
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
    audio_prompt_answer TEXT,
    spotlight_active_until TIMESTAMPTZ NULL,
    last_online TIMESTAMPTZ
);
CREATE INDEX idx_users_spotlight_active ON users (spotlight_active_until) WHERE spotlight_active_until IS NOT NULL;
CREATE INDEX idx_users_email ON users (email);
CREATE INDEX idx_users_last_online ON users (last_online DESC NULLS LAST);

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

CREATE TRIGGER set_timestamp
BEFORE UPDATE ON filters
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE app_open_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    opened_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_app_open_logs_user_time ON app_open_logs (user_id, opened_at DESC);

CREATE OR REPLACE FUNCTION haversine(lat1 float, lon1 float, lat2 float, lon2 float)
RETURNS float AS $$
DECLARE
    radius float := 6371; delta_lat float; delta_lon float; a float; c float; d float;
BEGIN
    delta_lat := RADIANS(lat2 - lat1); delta_lon := RADIANS(lon2 - lon1);
    a := SIN(delta_lat / 2) * SIN(delta_lat / 2) + COS(RADIANS(lat1)) * COS(RADIANS(lat2)) * SIN(delta_lon / 2) * SIN(delta_lon / 2);
    c := 2 * ASIN(SQRT(a)); d := radius * c;
    RETURN d;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

CREATE TABLE dislikes (
    disliker_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    disliked_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (disliker_user_id, disliked_user_id)
);
CREATE INDEX idx_dislikes_disliked_user ON dislikes (disliked_user_id);

CREATE TABLE likes (
    id SERIAL PRIMARY KEY,
    liker_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    liked_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content_type content_like_type NOT NULL DEFAULT 'media',
    content_identifier TEXT NOT NULL DEFAULT '0',
    comment TEXT CHECK (length(comment) <= 140),
    interaction_type like_interaction_type NOT NULL DEFAULT 'standard',
    is_seen BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_like_item UNIQUE (liker_user_id, liked_user_id, content_type, content_identifier)
);
CREATE INDEX idx_likes_liked_user ON likes (liked_user_id);
CREATE INDEX idx_likes_liker_user ON likes (liker_user_id);

CREATE TABLE user_subscriptions (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    feature_type premium_feature_type NOT NULL,
    activated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (feature_type IN ('unlimited_likes', 'travel_mode'))
);
CREATE INDEX idx_user_subscriptions_user_expires ON user_subscriptions (user_id, feature_type, expires_at);

CREATE TABLE user_consumables (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    consumable_type premium_feature_type NOT NULL,
    quantity INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, consumable_type),
    CHECK (consumable_type IN ('rose', 'spotlight')),
    CHECK (quantity >= 0)
);

CREATE TABLE chat_messages (
    id BIGSERIAL PRIMARY KEY,
    sender_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipient_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    message_text TEXT,
    media_url TEXT,
    media_type TEXT,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    is_read BOOLEAN NOT NULL DEFAULT false,
    CONSTRAINT chk_sender_recipient_different CHECK (sender_user_id <> recipient_user_id),
    CONSTRAINT chk_message_content CHECK (
    (
      message_text IS NOT NULL
      AND media_url IS NULL
      AND media_type IS NULL
      AND char_length(message_text) BETWEEN 1 AND 500
    )
    OR
    (
      message_text IS NULL
      AND media_url IS NOT NULL
      AND media_type IS NOT NULL
    )
  )
);

CREATE INDEX idx_chat_messages_conversation ON chat_messages (sender_user_id, recipient_user_id, sent_at DESC);
CREATE INDEX idx_chat_messages_recipient_time ON chat_messages (recipient_user_id, sent_at DESC);
CREATE INDEX idx_chat_messages_recipient_unread ON chat_messages (recipient_user_id, is_read) WHERE is_read = false;

CREATE TABLE reports (
    id BIGSERIAL PRIMARY KEY,
    reporter_user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reported_user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reason report_reason NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT reporter_cannot_be_reported CHECK (reporter_user_id <> reported_user_id)
);

CREATE INDEX idx_reports_reporter_user_id ON reports(reporter_user_id);
CREATE INDEX idx_reports_reported_user_id ON reports(reported_user_id);

CREATE TABLE message_reactions (
    id BIGSERIAL PRIMARY KEY,
    message_id BIGINT NOT NULL REFERENCES chat_messages(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    emoji VARCHAR(13) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_message_user UNIQUE (message_id, user_id)
);

CREATE INDEX idx_message_reactions_message_id ON message_reactions (message_id);
CREATE INDEX idx_message_reactions_user_id ON message_reactions (user_id);

CREATE TRIGGER set_reaction_timestamp
BEFORE UPDATE ON message_reactions
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();
