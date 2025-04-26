--
-- PostgreSQL database dump
--

-- Dumped from database version 17.4
-- Dumped by pg_dump version 17.4 (Homebrew)

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

--
-- Data for Name: users; Type: TABLE DATA; Schema: public; Owner: neondb_owner
--

COPY public.users (id, created_at, name, last_name, email, date_of_birth, latitude, longitude, gender, dating_intention, height, hometown, job_title, education, religious_beliefs, drinking_habit, smoking_habit, media_urls, verification_status, verification_pic, role, audio_prompt_question, audio_prompt_answer, spotlight_active_until, last_online) FROM stdin;
17	2025-04-21 15:02:53.690116+00	kushal		ayushiitroorkie@gmail.com	2006-05-05	29.8665951	77.8999037	man	longTermOpenShort	66	Lucknow	Athlete	Nift	buddhist	no	no	{https://peeple1.s3.ap-south-1.amazonaws.com/uploads/17/2025-04-21/IMG_20250421_185657_071.jpg,https://peeple1.s3.ap-south-1.amazonaws.com/uploads/17/2025-04-21/IMG_20250421_185656_900.jpg,https://peeple1.s3.ap-south-1.amazonaws.com/uploads/17/2025-04-21/IMG_20250421_185656_897.jpg}	false	\N	user	cookWithMe	https://peeple1.s3.ap-south-1.amazonaws.com/users/17/audio/cookwithme/1745247970793197186-voice_prompt_1745247961233.m4a	\N	\N
18	2025-04-21 15:06:45.935222+00	mansi		peeple.help@gmail.com	2004-11-05	29.8665951	77.8999037	woman	shortTerm	64	Mohali	Doctor	SMS Jaipur	sikh	sometimes	no	{https://peeple1.s3.ap-south-1.amazonaws.com/uploads/18/2025-04-21/IMG_20250421_185656_822.jpg,https://peeple1.s3.ap-south-1.amazonaws.com/uploads/18/2025-04-21/IMG_20250421_185657_345.jpg,https://peeple1.s3.ap-south-1.amazonaws.com/uploads/18/2025-04-21/IMG_20250421_185656_987.jpg}	false	\N	user	dontHateMeIfI	https://peeple1.s3.ap-south-1.amazonaws.com/users/18/audio/donthatemeifi/1745248198476168003-voice_prompt_1745248186537.m4a	\N	2025-04-24 10:37:21.481426+00
12	2025-04-20 13:53:03.112973+00	shruti		sg64776@gmail.com	2005-05-05	29.8666161	77.8998275	woman	longTerm	62	Udaipur	Footballer	IIT Madras	zoroastrian	no	yes	{https://peeple1.s3.ap-south-1.amazonaws.com/uploads/12/2025-04-21/IMG_20250421_185657_342.jpg,https://peeple1.s3.ap-south-1.amazonaws.com/uploads/12/2025-04-21/IMG_20250421_185656_748.jpg,https://peeple1.s3.ap-south-1.amazonaws.com/uploads/12/2025-04-21/IMG_20250421_190407_527.jpg}	false	\N	user	chooseOurFirstDate	https://peeple1.s3.ap-south-1.amazonaws.com/users/12/audio/chooseourfirstdate/1745157284840473795-voice_prompt_1745157272575.m4a	\N	2025-04-26 02:57:01.384749+00
13	2025-04-20 13:55:53.306309+00	Ayush		ayush_g@ar.iitr.ac.in	2004-05-05	29.8666145	77.8999064	man	longTermOpenShort	61	Roorkee	Carpenter	DTU	jain	yes	no	{https://peeple1.s3.ap-south-1.amazonaws.com/uploads/13/2025-04-21/IMG_20250421_185709_657.jpg,https://peeple1.s3.ap-south-1.amazonaws.com/uploads/13/2025-04-21/IMG_20250421_185708_857.jpg,https://peeple1.s3.ap-south-1.amazonaws.com/uploads/13/2025-04-21/IMG_20250421_185657_008.jpg}	false	\N	user	changeMyMindAbout	https://peeple1.s3.ap-south-1.amazonaws.com/users/13/audio/changemymindabout/1745157853104918363-voice_prompt_1745157819780.m4a	\N	2025-04-26 11:35:34.672609+00
\.


--
-- Data for Name: app_open_logs; Type: TABLE DATA; Schema: public; Owner: neondb_owner
--

COPY public.app_open_logs (id, user_id, opened_at) FROM stdin;
\.


--
-- Data for Name: chat_messages; Type: TABLE DATA; Schema: public; Owner: neondb_owner
--

COPY public.chat_messages (id, sender_user_id, recipient_user_id, message_text, media_url, media_type, sent_at, is_read, reply_to_message_id) FROM stdin;
3	12	13	Please read this message, User 13!	\N	\N	2025-04-26 03:01:33.634777+00	t	\N
4	13	12	Hi	\N	\N	2025-04-26 09:57:40.955427+00	f	\N
5	13	12	\N	https://peeple1.s3.ap-south-1.amazonaws.com/chat-media/13/1745661502360111099-file_00000000c82061f7b2614d029a7413e8_conversation_id=680b934b-5a84-800e-a61d-5ec49165556a&message_id=0f95c5ad-dd1a-4789-a931-3bf930170dfa.jpg	image/jpeg	2025-04-26 09:58:26.730357+00	f	\N
6	13	12	\N	https://peeple1.s3.ap-south-1.amazonaws.com/chat-media/13/1745661534303934947-Screenshot_2025-04-23-19-10-44-788_com.example.dtx.jpg	image/jpeg	2025-04-26 09:58:58.06495+00	f	\N
7	13	12	\N	https://peeple1.s3.ap-south-1.amazonaws.com/chat-media/13/1745661560165254435-Screenshot_2025-04-25-16-03-59-230_com.example.dtx.jpg	image/jpeg	2025-04-26 09:59:22.274641+00	f	\N
8	13	12	\N	https://peeple1.s3.ap-south-1.amazonaws.com/chat-media/13/1745661588080593119-Screenshot_2025-04-23-19-11-12-859_com.example.dtx.jpg	image/jpeg	2025-04-26 09:59:50.288581+00	f	\N
9	13	12	Heya	\N	\N	2025-04-26 10:18:25.403167+00	f	\N
10	13	12	\N	https://peeple1.s3.ap-south-1.amazonaws.com/chat-media/13/1745663361518663205-IMG-20250422-WA0016.jpg	image/jpeg	2025-04-26 10:29:34.636528+00	f	\N
11	13	12	\N	https://peeple1.s3.ap-south-1.amazonaws.com/chat-media/13/1745663411811704251-Screenshot_2025-04-23-15-34-53-936_com.android.chrome.jpg	image/jpeg	2025-04-26 10:30:13.789378+00	f	\N
12	13	12	\N	https://peeple1.s3.ap-south-1.amazonaws.com/chat-media/13/1745664685905088379-Screenshot_2025-04-24-10-29-52-905_com.google.android.gm.jpg	image/jpeg	2025-04-26 10:51:30.545393+00	f	\N
13	13	12	Hellow	\N	\N	2025-04-26 11:00:04.943082+00	f	\N
14	13	12	\N	https://peeple1.s3.ap-south-1.amazonaws.com/chat-media/13/1745665214222346901-Screenshot_2025-04-23-19-12-23-800_com.example.dtx.jpg	image/jpeg	2025-04-26 11:00:17.22939+00	f	\N
\.


--
-- Data for Name: date_vibes_prompts; Type: TABLE DATA; Schema: public; Owner: neondb_owner
--

COPY public.date_vibes_prompts (id, user_id, question, answer) FROM stdin;
\.


--
-- Data for Name: dislikes; Type: TABLE DATA; Schema: public; Owner: neondb_owner
--

COPY public.dislikes (disliker_user_id, disliked_user_id, created_at) FROM stdin;
18	17	2025-04-25 16:32:31.903552+00
\.


--
-- Data for Name: filters; Type: TABLE DATA; Schema: public; Owner: neondb_owner
--

COPY public.filters (user_id, who_you_want_to_see, radius_km, active_today, age_min, age_max, created_at, updated_at) FROM stdin;
12	man	500	f	18	55	2025-04-20 13:53:12.715144+00	2025-04-20 13:53:12.715144+00
13	woman	500	f	18	55	2025-04-20 13:56:13.271705+00	2025-04-21 12:32:44.30093+00
17	woman	500	f	18	55	2025-04-21 15:04:16.634138+00	2025-04-21 15:04:16.634138+00
18	man	500	f	18	55	2025-04-21 15:07:31.488501+00	2025-04-21 15:07:31.488501+00
\.


--
-- Data for Name: getting_personal_prompts; Type: TABLE DATA; Schema: public; Owner: neondb_owner
--

COPY public.getting_personal_prompts (id, user_id, question, answer) FROM stdin;
6	18	loveLanguage	Surgery
\.


--
-- Data for Name: likes; Type: TABLE DATA; Schema: public; Owner: neondb_owner
--

COPY public.likes (id, liker_user_id, liked_user_id, content_type, content_identifier, comment, interaction_type, is_seen, created_at) FROM stdin;
1	18	13	media	0	\N	standard	f	2025-04-25 16:35:18.095516+00
2	13	18	media	0	Hi	standard	f	2025-04-25 16:35:41.91093+00
3	12	13	media	0	Great picture!	standard	f	2025-04-25 16:53:16.490217+00
4	13	12	media	0	Great picture!	standard	f	2025-04-25 16:54:10.078909+00
\.


--
-- Data for Name: message_reactions; Type: TABLE DATA; Schema: public; Owner: neondb_owner
--

COPY public.message_reactions (id, message_id, user_id, emoji, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: my_type_prompts; Type: TABLE DATA; Schema: public; Owner: neondb_owner
--

COPY public.my_type_prompts (id, user_id, question, answer) FROM stdin;
\.


--
-- Data for Name: reports; Type: TABLE DATA; Schema: public; Owner: neondb_owner
--

COPY public.reports (id, reporter_user_id, reported_user_id, reason, created_at) FROM stdin;
\.


--
-- Data for Name: story_time_prompts; Type: TABLE DATA; Schema: public; Owner: neondb_owner
--

COPY public.story_time_prompts (id, user_id, question, answer) FROM stdin;
9	13	biggestRisk	Ayush you are the best.
10	13	neverHaveIEver	Replied to this question
11	12	biggestRisk	Hohohoho
12	17	biggestRisk	Making this profile
13	18	bestTravelStory	Newzealand
14	18	oneThingNeverDoAgain	MBBS
\.


--
-- Data for Name: user_consumables; Type: TABLE DATA; Schema: public; Owner: neondb_owner
--

COPY public.user_consumables (user_id, consumable_type, quantity, updated_at) FROM stdin;
12	rose	1	2025-04-20 13:53:03.419949+00
13	rose	1	2025-04-20 13:55:53.919255+00
17	rose	1	2025-04-21 15:02:54.304622+00
18	rose	1	2025-04-21 15:06:46.550048+00
\.


--
-- Data for Name: user_subscriptions; Type: TABLE DATA; Schema: public; Owner: neondb_owner
--

COPY public.user_subscriptions (id, user_id, feature_type, activated_at, expires_at, created_at) FROM stdin;
\.


--
-- Name: app_open_logs_id_seq; Type: SEQUENCE SET; Schema: public; Owner: neondb_owner
--

SELECT pg_catalog.setval('public.app_open_logs_id_seq', 1, false);


--
-- Name: chat_messages_id_seq; Type: SEQUENCE SET; Schema: public; Owner: neondb_owner
--

SELECT pg_catalog.setval('public.chat_messages_id_seq', 14, true);


--
-- Name: date_vibes_prompts_id_seq; Type: SEQUENCE SET; Schema: public; Owner: neondb_owner
--

SELECT pg_catalog.setval('public.date_vibes_prompts_id_seq', 1, false);


--
-- Name: getting_personal_prompts_id_seq; Type: SEQUENCE SET; Schema: public; Owner: neondb_owner
--

SELECT pg_catalog.setval('public.getting_personal_prompts_id_seq', 1, false);


--
-- Name: likes_id_seq; Type: SEQUENCE SET; Schema: public; Owner: neondb_owner
--

SELECT pg_catalog.setval('public.likes_id_seq', 4, true);


--
-- Name: message_reactions_id_seq; Type: SEQUENCE SET; Schema: public; Owner: neondb_owner
--

SELECT pg_catalog.setval('public.message_reactions_id_seq', 4, true);


--
-- Name: my_type_prompts_id_seq; Type: SEQUENCE SET; Schema: public; Owner: neondb_owner
--

SELECT pg_catalog.setval('public.my_type_prompts_id_seq', 1, false);


--
-- Name: reports_id_seq; Type: SEQUENCE SET; Schema: public; Owner: neondb_owner
--

SELECT pg_catalog.setval('public.reports_id_seq', 1, false);


--
-- Name: story_time_prompts_id_seq; Type: SEQUENCE SET; Schema: public; Owner: neondb_owner
--

SELECT pg_catalog.setval('public.story_time_prompts_id_seq', 1, false);


--
-- Name: user_subscriptions_id_seq; Type: SEQUENCE SET; Schema: public; Owner: neondb_owner
--

SELECT pg_catalog.setval('public.user_subscriptions_id_seq', 1, false);


--
-- Name: users_id_seq; Type: SEQUENCE SET; Schema: public; Owner: neondb_owner
--

SELECT pg_catalog.setval('public.users_id_seq', 1, false);


--
-- PostgreSQL database dump complete
--

