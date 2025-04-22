SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

COPY public.users (id, created_at, name, last_name, email, date_of_birth, latitude, longitude, gender, dating_intention, height, hometown, job_title, education, religious_beliefs, drinking_habit, smoking_habit, media_urls, verification_status, verification_pic, role, audio_prompt_question, audio_prompt_answer, spotlight_active_until) FROM stdin;
1	2025-04-20 12:57:09.440764+00	Alice	Smith	alice.smith@email.com	1997-03-15	29.8665972	77.90812129028555	woman	longTerm	165	New York	Software Engineer	Masters Degree	agnostic	sometimes	no	{media/alice1.jpg,media/alice2.jpg,media/alice3.png}	true	\N	user	aRandomFactILoveIs	audio/alice_randomfact.mp3	\N
2	2025-04-20 12:57:09.440764+00	Bob	Johnson	bob.j@email.com	1996-07-22	29.8665972	77.91031954136328	man	lifePartner	180	Brooklyn	Graphic Designer	Bachelors Degree	spiritual	yes	no	{media/bob1.jpeg}	true	\N	user	datingMeIsLike	audio/bob_datingmelike.mp3	\N
3	2025-04-20 12:57:09.440764+00	Charlie	Williams	charlie.w@email.com	1998-11-01	29.8665972	77.90843488411778	man	shortTermOpenLong	175	Manhattan	Musician	Some College	atheist	yes	sometimes	{media/charlie1.jpg,media/charlie2.gif}	false	\N	user	\N	\N	2025-04-20 13:27:09.440764+00
4	2025-04-20 12:57:09.440764+00	Diana	Brown	diana.b@email.com	1996-05-10	29.8665972	77.87465570216798	woman	figuringOut	170	Queens	Chef	Culinary School	christian	sometimes	no	{media/diana1.jpg,media/diana2.jpg,media/diana3.jpg,media/diana4.jpg}	pending	\N	user	cookWithMe	audio/diana_cook.mp3	\N
5	2025-04-20 12:57:09.440764+00	Ethan	Davis	ethan.d@email.com	1999-01-25	29.8665972	77.9084356562672	man	longTerm	182	New York	Architect	Masters Degree	buddhist	no	no	{media/ethan1.png,media/ethan2.png}	true	\N	user	aDreamHomeMustInclude	audio/ethan_dreamhome.mp3	\N
6	2025-04-20 12:57:09.440764+00	Fiona	Miller	fiona.m@email.com	1999-09-09	29.8665972	77.91422324479382	woman	longTermOpenShort	163	Jersey City	Student	High School	spiritual	no	no	{media/fiona1.jpg}	false	\N	user	\N	\N	\N
7	2025-04-20 12:57:09.440764+00	George	Wilson	george.w@email.com	1997-12-12	29.8665972	77.89179056409012	man	lifePartner	178	Manhattan	Doctor	MD	jewish	sometimes	no	{media/george1.jpg,media/george2.jpg}	true	\N	user	aLifeGoalOfMine	audio/george_lifegoal.mp3	\N
8	2025-04-20 12:57:09.440764+00	Hannah	Moore	hannah.m@email.com	1998-06-30	29.8665972	77.88632681104704	woman	longTerm	168	Brooklyn	Yoga Instructor	Bachelors Degree	hindu	no	no	{media/hannah1.jpg,media/hannah2.jpg}	true	\N	user	iBeatMyBluesBy	audio/hannah_blues.mp3	\N
9	2025-04-20 12:57:09.440764+00	Ian	Taylor	ian.t@email.com	1997-04-05	29.8665972	77.8884369593791	man	shortTerm	190	Manhattan	Bartender	Some College	agnostic	yes	yes	{media/ian1.jpg}	false	\N	user	\N	\N	\N
10	2025-04-20 12:57:09.440764+00	Admin	User	admin@app.com	1996-01-01	29.8665972	77.87675098568556	woman	longTerm	160	System	Administrator	PhD	atheist	no	no	{}	true	\N	admin	\N	\N	\N
13	2025-04-20 13:55:53.306309+00	Ayush		ayush_g@ar.iitr.ac.in	2004-05-05	29.8666145	77.8999064	man	longTermOpenShort	61	Roorkee	Carpenter	DTU	jain	yes	no	{https://peeple1.s3.ap-south-1.amazonaws.com/uploads/13/2025-04-21/IMG_20250421_185709_657.jpg,https://peeple1.s3.ap-south-1.amazonaws.com/uploads/13/2025-04-21/IMG_20250421_185708_857.jpg,https://peeple1.s3.ap-south-1.amazonaws.com/uploads/13/2025-04-21/IMG_20250421_185657_008.jpg}	false	\N	user	changeMyMindAbout	https://peeple1.s3.ap-south-1.amazonaws.com/users/13/audio/changemymindabout/1745157853104918363-voice_prompt_1745157819780.m4a	\N
12	2025-04-20 13:53:03.112973+00	shruti		sg64776@gmail.com	2005-05-05	29.8666161	77.8998275	woman	longTerm	62	Udaipur	Footballer	IIT Madras	zoroastrian	no	yes	{https://peeple1.s3.ap-south-1.amazonaws.com/uploads/12/2025-04-21/IMG_20250421_185657_342.jpg,https://peeple1.s3.ap-south-1.amazonaws.com/uploads/12/2025-04-21/IMG_20250421_185656_748.jpg,https://peeple1.s3.ap-south-1.amazonaws.com/uploads/12/2025-04-21/IMG_20250421_190407_527.jpg}	false	\N	user	chooseOurFirstDate	https://peeple1.s3.ap-south-1.amazonaws.com/users/12/audio/chooseourfirstdate/1745157284840473795-voice_prompt_1745157272575.m4a	\N
18	2025-04-21 15:06:45.935222+00	mansi		peeple.help@gmail.com	2004-11-05	29.8665951	77.8999037	woman	shortTerm	64	Mohali	Doctor	SMS Jaipur	sikh	sometimes	no	{https://peeple1.s3.ap-south-1.amazonaws.com/uploads/18/2025-04-21/IMG_20250421_185656_822.jpg,https://peeple1.s3.ap-south-1.amazonaws.com/uploads/18/2025-04-21/IMG_20250421_185657_345.jpg,https://peeple1.s3.ap-south-1.amazonaws.com/uploads/18/2025-04-21/IMG_20250421_185656_987.jpg}	false	\N	user	dontHateMeIfI	https://peeple1.s3.ap-south-1.amazonaws.com/users/18/audio/donthatemeifi/1745248198476168003-voice_prompt_1745248186537.m4a	\N
17	2025-04-21 15:02:53.690116+00	kushal		ayushiitroorkie@gmail.com	2006-05-05	29.8665951	77.8999037	man	longTermOpenShort	66	Lucknow	Athlete	Nift	buddhist	no	no	{https://peeple1.s3.ap-south-1.amazonaws.com/uploads/17/2025-04-21/IMG_20250421_185657_071.jpg,https://peeple1.s3.ap-south-1.amazonaws.com/uploads/17/2025-04-21/IMG_20250421_185656_900.jpg,https://peeple1.s3.ap-south-1.amazonaws.com/uploads/17/2025-04-21/IMG_20250421_185656_897.jpg}	false	\N	user	cookWithMe	https://peeple1.s3.ap-south-1.amazonaws.com/users/17/audio/cookwithme/1745247970793197186-voice_prompt_1745247961233.m4a	\N
\.

COPY public.app_open_logs (id, user_id, opened_at) FROM stdin;
1	1	2025-04-20 11:57:09.440764+00
2	2	2025-04-18 12:57:09.440764+00
3	1	2025-04-20 12:52:09.440764+00
4	3	2025-04-20 12:47:09.440764+00
5	4	2025-04-20 09:57:09.440764+00
6	5	2025-04-19 12:57:09.440764+00
7	6	2025-04-20 12:42:09.440764+00
8	7	2025-04-20 06:57:09.440764+00
9	8	2025-04-20 12:27:09.440764+00
10	9	2025-04-20 08:57:09.440764+00
\.

COPY public.chat_messages (id, sender_user_id, recipient_user_id, message_text, media_url, media_type, sent_at, is_read) FROM stdin;
\.

COPY public.date_vibes_prompts (id, user_id, question, answer) FROM stdin;
1	2	togetherWeCould	Explore art galleries in Chelsea, find hidden taco trucks in Bushwick, and debate the best bagel spot.
2	4	bestSpotInTown	A cozy little Italian place in the West Village with amazing pasta.
3	5	firstRoundIsOnMeIf	You can beat me at a game of chess in Washington Square Park.
4	9	bestWayToAskMeOut	Just be direct and suggest a specific plan! Bonus points for a cool cocktail bar in the LES.
\.

COPY public.dislikes (disliker_user_id, disliked_user_id, created_at) FROM stdin;
\.

COPY public.filters (user_id, who_you_want_to_see, radius_km, active_today, age_min, age_max, created_at, updated_at) FROM stdin;
1	man	20	t	25	30	2025-04-20 12:57:09.440764+00	2025-04-20 12:57:09.440764+00
2	woman	30	f	24	29	2025-04-20 12:57:09.440764+00	2025-04-20 12:57:09.440764+00
3	woman	15	t	24	28	2025-04-20 12:57:09.440764+00	2025-04-20 12:57:09.440764+00
4	man	25	t	26	30	2025-04-20 12:57:09.440764+00	2025-04-20 12:57:09.440764+00
5	woman	40	f	24	29	2025-04-20 12:57:09.440764+00	2025-04-20 12:57:09.440764+00
6	man	10	t	23	28	2025-04-20 12:57:09.440764+00	2025-04-20 12:57:09.440764+00
7	woman	20	f	25	30	2025-04-20 12:57:09.440764+00	2025-04-20 12:57:09.440764+00
8	man	15	t	25	30	2025-04-20 12:57:09.440764+00	2025-04-20 12:57:09.440764+00
9	woman	10	t	24	29	2025-04-20 12:57:09.440764+00	2025-04-20 12:57:09.440764+00
12	man	500	f	18	55	2025-04-20 13:53:12.715144+00	2025-04-20 13:53:12.715144+00
13	woman	500	f	18	55	2025-04-20 13:56:13.271705+00	2025-04-21 12:32:44.30093+00
17	woman	500	f	18	55	2025-04-21 15:04:16.634138+00	2025-04-21 15:04:16.634138+00
18	man	500	f	18	55	2025-04-21 15:07:31.488501+00	2025-04-21 15:07:31.488501+00
\.

COPY public.getting_personal_prompts (id, user_id, question, answer) FROM stdin;
1	1	oneThingYouShouldKnow	I can be a bit introverted at first, but I warm up quickly!
2	3	geekOutOn	Vintage synthesizers and finding hidden music venues in Brooklyn.
3	4	loveLanguage	Quality time and acts of service. Let's grab dinner somewhere new!
4	7	wontShutUpAbout	The latest medical research I read... sorry in advance!
5	8	ifLovingThisIsWrong	Spending a whole Saturday reading in Prospect Park.
6	18	loveLanguage	Surgery
\.

COPY public.likes (id, liker_user_id, liked_user_id, content_type, content_identifier, comment, interaction_type, created_at) FROM stdin;
\.

COPY public.my_type_prompts (id, user_id, question, answer) FROM stdin;
1	1	lookingFor	Someone kind, funny, and adventurous who enjoys exploring the city.
2	2	hallmarkOfGoodRelationship	Open communication and mutual respect, even when you disagree.
3	5	wantSomeoneWho	Appreciates good design, quiet nights in, and exploring different boroughs.
4	6	nonNegotiable	Must love dogs (or at least tolerate my obsession).
5	8	greenFlags	Being genuinely curious about others and practicing active listening.
\.

COPY public.reports (id, reporter_user_id, reported_user_id, reason, created_at) FROM stdin;
1	1	9	inappropriate	2025-04-17 12:57:09.440764+00
2	5	3	spam	2025-04-13 12:57:09.440764+00
\.

COPY public.story_time_prompts (id, user_id, question, answer) FROM stdin;
1	1	twoTruthsAndALie	I speak 3 languages. I've run the NYC marathon. I hate pizza. (The lie is hating pizza!)
2	2	biggestRisk	Quitting my stable job to pursue graphic design full-time.
3	3	bestTravelStory	Getting stuck on the subway overnight during a blizzard - surprisingly fun crowd!
4	4	neverHaveIEver	Never have I ever been to the top of the Empire State Building.
5	7	biggestDateFail	Spilled coffee all over myself meeting someone at a cafe in Midtown.
9	13	biggestRisk	Ayush you are the best.
10	13	neverHaveIEver	Replied to this question
11	12	biggestRisk	Hohohoho
12	17	biggestRisk	Making this profile
13	18	bestTravelStory	Newzealand
14	18	oneThingNeverDoAgain	MBBS
\.

COPY public.user_consumables (user_id, consumable_type, quantity, updated_at) FROM stdin;
1	rose	3	2025-04-20 12:57:09.440764+00
4	spotlight	1	2025-04-20 12:57:09.440764+00
8	rose	1	2025-04-20 12:57:09.440764+00
12	rose	1	2025-04-20 13:53:03.419949+00
13	rose	1	2025-04-20 13:55:53.919255+00
17	rose	1	2025-04-21 15:02:54.304622+00
18	rose	1	2025-04-21 15:06:46.550048+00
\.

COPY public.user_subscriptions (id, user_id, feature_type, activated_at, expires_at, created_at) FROM stdin;
1	2	unlimited_likes	2025-04-10 12:57:09.440764+00	2025-05-10 12:57:09.440764+00	2025-04-20 12:57:09.440764+00
2	5	travel_mode	2025-04-18 12:57:09.440764+00	2025-04-25 12:57:09.440764+00	2025-04-20 12:57:09.440764+00
\.

SELECT pg_catalog.setval('public.app_open_logs_id_seq', 1, false);

SELECT pg_catalog.setval('public.chat_messages_id_seq', 1, false);

SELECT pg_catalog.setval('public.date_vibes_prompts_id_seq', 1, false);

SELECT pg_catalog.setval('public.getting_personal_prompts_id_seq', 1, false);

SELECT pg_catalog.setval('public.likes_id_seq', 1, false);

SELECT pg_catalog.setval('public.my_type_prompts_id_seq', 1, false);

SELECT pg_catalog.setval('public.reports_id_seq', 1, false);

SELECT pg_catalog.setval('public.story_time_prompts_id_seq', 1, false);

SELECT pg_catalog.setval('public.user_subscriptions_id_seq', 1, false);

SELECT pg_catalog.setval('public.users_id_seq', 1, false);
