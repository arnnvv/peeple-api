TRUNCATE TABLE 
    users, 
    story_time_prompts, 
    my_type_prompts, 
    getting_personal_prompts, 
    date_vibes_prompts, 
    filters, 
    app_open_logs, 
    dislikes, 
    likes, 
    user_subscriptions, 
    user_consumables, 
    chat_messages, 
    reports 
RESTART IDENTITY CASCADE;

BEGIN;

INSERT INTO users (
    name, last_name, email, date_of_birth, latitude, longitude, gender, 
    dating_intention, height, hometown, job_title, education, religious_beliefs, 
    drinking_habit, smoking_habit, media_urls, verification_status, role, 
    audio_prompt_question, audio_prompt_answer, spotlight_active_until
) VALUES
    ('Alice', 'Smith', 'alice.smith@email.com', '1997-03-15', 40.7128, -74.0060, 'woman', 'longTerm', 165, 'New York', 'Software Engineer', 'Masters Degree', 'agnostic', 'sometimes', 'no', ARRAY['media/alice1.jpg', 'media/alice2.jpg', 'media/alice3.png'], 'true', 'user', 'aRandomFactILoveIs', 'audio/alice_randomfact.mp3', NULL),
    ('Bob', 'Johnson', 'bob.j@email.com', '1996-07-22', 40.7580, -73.9855, 'man', 'lifePartner', 180, 'Brooklyn', 'Graphic Designer', 'Bachelors Degree', 'spiritual', 'yes', 'no', ARRAY['media/bob1.jpeg'], 'true', 'user', 'datingMeIsLike', 'audio/bob_datingmelike.mp3', NULL),
    ('Charlie', 'Williams', 'charlie.w@email.com', '1998-11-01', 40.7295, -73.9965, 'man', 'shortTermOpenLong', 175, 'Manhattan', 'Musician', 'Some College', 'atheist', 'yes', 'sometimes', ARRAY['media/charlie1.jpg', 'media/charlie2.gif'], 'false', 'user', NULL, NULL, NOW() + INTERVAL '30 minutes'),
    ('Diana', 'Brown', 'diana.b@email.com', '1996-05-10', 40.7484, -73.9857, 'woman', 'figuringOut', 170, 'Queens', 'Chef', 'Culinary School', 'christian', 'sometimes', 'no', ARRAY['media/diana1.jpg', 'media/diana2.jpg', 'media/diana3.jpg', 'media/diana4.jpg'], 'pending', 'user', 'cookWithMe', 'audio/diana_cook.mp3', NULL),
    ('Ethan', 'Davis', 'ethan.d@email.com', '1999-01-25', 40.7679, -73.9822, 'man', 'longTerm', 182, 'New York', 'Architect', 'Masters Degree', 'buddhist', 'no', 'no', ARRAY['media/ethan1.png', 'media/ethan2.png'], 'true', 'user', 'aDreamHomeMustInclude', 'audio/ethan_dreamhome.mp3', NULL),
    ('Fiona', 'Miller', 'fiona.m@email.com', '1999-09-09', 40.7050, -74.0090, 'woman', 'longTermOpenShort', 163, 'Jersey City', 'Student', 'High School', 'spiritual', 'no', 'no', ARRAY['media/fiona1.jpg'], 'false', 'user', NULL, NULL, NULL),
    ('George', 'Wilson', 'george.w@email.com', '1997-12-12', 40.7580, -73.9855, 'man', 'lifePartner', 178, 'Manhattan', 'Doctor', 'MD', 'jewish', 'sometimes', 'no', ARRAY['media/george1.jpg', 'media/george2.jpg'], 'true', 'user', 'aLifeGoalOfMine', 'audio/george_lifegoal.mp3', NULL),
    ('Hannah', 'Moore', 'hannah.m@email.com', '1998-06-30', 40.7420, -73.9875, 'woman', 'longTerm', 168, 'Brooklyn', 'Yoga Instructor', 'Bachelors Degree', 'hindu', 'no', 'no', ARRAY['media/hannah1.jpg', 'media/hannah2.jpg'], 'true', 'user', 'iBeatMyBluesBy', 'audio/hannah_blues.mp3', NULL),
    ('Ian', 'Taylor', 'ian.t@email.com', '1997-04-05', 40.7230, -73.9930, 'man', 'shortTerm', 190, 'Manhattan', 'Bartender', 'Some College', 'agnostic', 'yes', 'yes', ARRAY['media/ian1.jpg'], 'false', 'user', NULL, NULL, NULL),
    ('Admin', 'User', 'admin@app.com', '1996-01-01', 40.7128, -74.0060, 'woman', 'longTerm', 160, 'System', 'Administrator', 'PhD', 'atheist', 'no', 'no', ARRAY[]::TEXT[], 'true', 'admin', NULL, NULL, NULL);

INSERT INTO story_time_prompts (user_id, question, answer) VALUES
    (1, 'twoTruthsAndALie', 'I speak 3 languages. I''ve run the NYC marathon. I hate pizza. (The lie is hating pizza!)'),
    (2, 'biggestRisk', 'Quitting my stable job to pursue graphic design full-time.'),
    (3, 'bestTravelStory', 'Getting stuck on the subway overnight during a blizzard - surprisingly fun crowd!'),
    (4, 'neverHaveIEver', 'Never have I ever been to the top of the Empire State Building.'),
    (7, 'biggestDateFail', 'Spilled coffee all over myself meeting someone at a cafe in Midtown.');

INSERT INTO my_type_prompts (user_id, question, answer) VALUES
    (1, 'lookingFor', 'Someone kind, funny, and adventurous who enjoys exploring the city.'),
    (2, 'hallmarkOfGoodRelationship', 'Open communication and mutual respect, even when you disagree.'),
    (5, 'wantSomeoneWho', 'Appreciates good design, quiet nights in, and exploring different boroughs.'),
    (6, 'nonNegotiable', 'Must love dogs (or at least tolerate my obsession).'),
    (8, 'greenFlags', 'Being genuinely curious about others and practicing active listening.');

INSERT INTO getting_personal_prompts (user_id, question, answer) VALUES
    (1, 'oneThingYouShouldKnow', 'I can be a bit introverted at first, but I warm up quickly!'),
    (3, 'geekOutOn', 'Vintage synthesizers and finding hidden music venues in Brooklyn.'),
    (4, 'loveLanguage', 'Quality time and acts of service. Let''s grab dinner somewhere new!'),
    (7, 'wontShutUpAbout', 'The latest medical research I read... sorry in advance!'),
    (8, 'ifLovingThisIsWrong', 'Spending a whole Saturday reading in Prospect Park.');

INSERT INTO date_vibes_prompts (user_id, question, answer) VALUES
    (2, 'togetherWeCould', 'Explore art galleries in Chelsea, find hidden taco trucks in Bushwick, and debate the best bagel spot.'),
    (4, 'bestSpotInTown', 'A cozy little Italian place in the West Village with amazing pasta.'),
    (5, 'firstRoundIsOnMeIf', 'You can beat me at a game of chess in Washington Square Park.'),
    (9, 'bestWayToAskMeOut', 'Just be direct and suggest a specific plan! Bonus points for a cool cocktail bar in the LES.');

INSERT INTO filters (user_id, who_you_want_to_see, radius_km, active_today, age_min, age_max) VALUES
    (1, 'man', 20, true, 25, 30),
    (2, 'woman', 30, false, 24, 29),
    (3, 'woman', 15, true, 24, 28),
    (4, 'man', 25, true, 26, 30),
    (5, 'woman', 40, false, 24, 29),
    (6, 'man', 10, true, 23, 28),
    (7, 'woman', 20, false, 25, 30),
    (8, 'man', 15, true, 25, 30),
    (9, 'woman', 10, true, 24, 29);

INSERT INTO app_open_logs (user_id, opened_at) VALUES
    (1, NOW() - INTERVAL '1 hour'),
    (2, NOW() - INTERVAL '2 days'),
    (1, NOW() - INTERVAL '5 minutes'),
    (3, NOW() - INTERVAL '10 minutes'),
    (4, NOW() - INTERVAL '3 hours'),
    (5, NOW() - INTERVAL '1 day'),
    (6, NOW() - INTERVAL '15 minutes'),
    (7, NOW() - INTERVAL '6 hours'),
    (8, NOW() - INTERVAL '30 minutes'),
    (9, NOW() - INTERVAL '4 hours');

INSERT INTO dislikes (disliker_user_id, disliked_user_id) VALUES
    (1, 9),
    (3, 5),
    (6, 7);

INSERT INTO likes (liker_user_id, liked_user_id, content_type, content_identifier, comment, interaction_type) VALUES
    (1, 2, 'profile', '0', 'Great profile!', 'standard'),
    (4, 5, 'media', 'media/ethan1.png', 'Cool architecture shot!', 'standard'),
    (8, 7, 'prompt_gettingpersonal', 'wontShutUpAbout', 'Haha, relatable!', 'standard'),
    (1, 7, 'profile', '0', 'You seem really kind!', 'standard'),
    (7, 1, 'media', 'media/alice2.jpg', NULL, 'standard'),
    (2, 8, 'prompt_gettingpersonal', 'ifLovingThisIsWrong', 'My kind of Saturday!', 'standard'),
    (8, 2, 'profile', '0', NULL, 'rose'),
    (3, 4, 'audio_prompt', 'cookWithMe', 'Sounds delicious!', 'standard'),
    (4, 3, 'prompt_gettingpersonal', 'geekOutOn', 'Love those synths!', 'standard');

INSERT INTO user_subscriptions (user_id, feature_type, activated_at, expires_at) VALUES
    (2, 'unlimited_likes', NOW() - INTERVAL '10 days', NOW() + INTERVAL '20 days'),
    (5, 'travel_mode', NOW() - INTERVAL '2 days', NOW() + INTERVAL '5 days');

INSERT INTO user_consumables (user_id, consumable_type, quantity, updated_at) VALUES
    (1, 'rose', 3, NOW()),
    (4, 'spotlight', 1, NOW()),
    (8, 'rose', 1, NOW());

INSERT INTO chat_messages (sender_user_id, recipient_user_id, message_text, sent_at, is_read) VALUES
    (1, 7, 'Hi George! Thanks for the like :)', NOW() - INTERVAL '1 day', true),
    (7, 1, 'Hey Alice! Likewise. Your pictures are great.', NOW() - INTERVAL '23 hours', true),
    (1, 7, 'Thanks! So, you''re a doctor? That must be intense working here.', NOW() - INTERVAL '22 hours', false),
    (2, 8, 'Hey Hannah! A rose? Wow, thank you!', NOW() - INTERVAL '2 days', true),
    (8, 2, 'Hi Bob! Your profile made me smile :)', NOW() - INTERVAL '1 day 23 hours', true),
    (2, 8, 'Glad to hear it! Love your answer about Saturdays haha. Busy weekend?', NOW() - INTERVAL '1 day 22 hours', true),
    (8, 2, 'Not too bad! Mostly yoga and relaxing in BK. You?', NOW() - INTERVAL '1 day 21 hours', false),
    (3, 4, 'Hey Diana! Your audio prompt about cooking sounded amazing.', NOW() - INTERVAL '5 hours', true),
    (4, 3, 'Hi Charlie! Thanks! Vintage synths sound pretty cool too.', NOW() - INTERVAL '4 hours', false);

INSERT INTO reports (reporter_user_id, reported_user_id, reason, created_at) VALUES
    (1, 9, 'inappropriate', NOW() - INTERVAL '3 days'),
    (5, 3, 'spam', NOW() - INTERVAL '1 week');

COMMIT;

SELECT 'Seed data insertion complete (Age/Location Adjusted).' AS status;
