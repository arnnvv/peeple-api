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
    'canWeTalkAbout',
    'captionThisPhoto',
    'caughtInTheAct',
    'changeMyMindAbout',
    'chooseOurFirstDate',
    'commentIfYouveBeenHere',
    'cookWithMe',
    'datingMeIsLike',
    'datingMeWillLookLike',
    'doYouAgreeOrDisagreeThat',
    'dontHateMeIfI',
    'dontJudgeMe',
    'mondaysAmIRight',
    'aBoundaryOfMineIs',
    'aDailyEssential',
    'aDreamHomeMustInclude',
    'aFavouriteMemoryOfMine',
    'aFriendsReviewOfMe',
    'aLifeGoalOfMine',
    'aQuickRantAbout',
    'aRandomFactILoveIs',
    'aSpecialTalentOfMine',
    'aThoughtIRecentlyHadInTheShower',
    'allIAskIsThatYou',
    'guessWhereThisPhotoWasTaken',
    'helpMeIdentifyThisPhotoBomber',
    'hiFromMeAndMyPet',
    'howIFightTheSundayScaries',
    'howHistoryWillRememberMe',
    'howMyFriendsSeeMe',
    'howToPronounceMyName',
    'iBeatMyBluesBy',
    'iBetYouCant',
    'iCanTeachYouHowTo',
    'iFeelFamousWhen',
    'iFeelMostSupportedWhen'
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
    user_id INTEGER NOT NULL,
    otp_code VARCHAR(6) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '2 minute'),
    CONSTRAINT fk_otps_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE story_time_prompts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    question story_time_prompt_type NOT NULL,
    answer TEXT NOT NULL,
    CONSTRAINT fk_story_time_prompt_user
        FOREIGN KEY (user_id)
        REFERENCES users(id)
        ON DELETE CASCADE,
    CONSTRAINT uq_user_story_time_prompt UNIQUE (user_id, question)
);

CREATE TABLE my_type_prompts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    question my_type_prompt_type NOT NULL,
    answer TEXT NOT NULL,
    CONSTRAINT fk_my_type_prompt_user
        FOREIGN KEY (user_id)
        REFERENCES users(id)
        ON DELETE CASCADE,
    CONSTRAINT uq_user_my_type_prompt UNIQUE (user_id, question)
);

CREATE TABLE getting_personal_prompts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    question getting_personal_prompt_type NOT NULL,
    answer TEXT NOT NULL,
    CONSTRAINT fk_getting_personal_prompt_user
        FOREIGN KEY (user_id)
        REFERENCES users(id)
        ON DELETE CASCADE,
    CONSTRAINT uq_user_getting_personal_prompt UNIQUE (user_id, question)
);

CREATE TABLE date_vibes_prompts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    question date_vibes_prompt_type NOT NULL,
    answer TEXT NOT NULL,
    CONSTRAINT fk_date_vibes_prompt_user
        FOREIGN KEY (user_id)
        REFERENCES users(id)
        ON DELETE CASCADE,
    CONSTRAINT uq_user_date_vibes_prompts UNIQUE (user_id, question)
);

-- Function to update the updated_at column
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
   NEW.updated_at = NOW();
   RETURN NEW;
END;
$$ language 'plpgsql';

-- Filters table
CREATE TABLE filters (
    user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    who_you_want_to_see gender_enum, -- Can be 'man' or 'woman' from existing enum
    radius_km INTEGER CHECK (radius_km > 0 AND radius_km <= 500), -- Example constraint: 1-500 km
    active_today BOOLEAN NOT NULL DEFAULT false,
    age_min INTEGER CHECK (age_min >= 18),
    age_max INTEGER CHECK (age_max >= 18),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT age_check CHECK (age_max >= age_min) -- Ensure max age is not less than min age
);

COMMENT ON COLUMN filters.who_you_want_to_see IS 'Which gender the user wants to see in their feed.';
COMMENT ON COLUMN filters.radius_km IS 'Maximum distance in kilometers for potential matches.';
COMMENT ON COLUMN filters.active_today IS 'Filter for users active within the last 24 hours.';
COMMENT ON COLUMN filters.age_min IS 'Minimum age preference.';
COMMENT ON COLUMN filters.age_max IS 'Maximum age preference.';


-- Trigger to automatically update updated_at on table update
CREATE TRIGGER set_timestamp
BEFORE UPDATE ON filters
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();
