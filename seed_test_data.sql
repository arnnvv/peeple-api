-- ====================================================================
-- Peeple API Test Data Seeding Script (with Premium & Content Likes)
-- ====================================================================

-- Start Transaction
BEGIN;

-- Clean existing test data (optional but recommended for repeatable testing)
-- CASCADE will follow foreign keys
TRUNCATE TABLE likes CASCADE;
TRUNCATE TABLE dislikes CASCADE;
TRUNCATE TABLE app_open_logs CASCADE;
TRUNCATE TABLE filters CASCADE;
TRUNCATE TABLE user_subscriptions CASCADE; -- Added previously
TRUNCATE TABLE user_consumables CASCADE;   -- Added previously
-- Delete specific user range or TRUNCATE users if acceptable for testing
DELETE FROM users WHERE id BETWEEN 1 AND 20;

-- Reset sequences (Important for predictable IDs like 1, 2, 3...)
-- Adjust sequence names if they differ ('public.' prefix might be needed sometimes)
SELECT setval(pg_get_serial_sequence('users', 'id'), COALESCE((SELECT MAX(id) FROM users), 0) + 1, false);
SELECT setval(pg_get_serial_sequence('app_open_logs', 'id'), COALESCE((SELECT MAX(id) FROM app_open_logs), 0) + 1, false);
SELECT setval(pg_get_serial_sequence('user_subscriptions', 'id'), COALESCE((SELECT MAX(id) FROM user_subscriptions), 0) + 1, false);
SELECT setval(pg_get_serial_sequence('likes', 'id'), COALESCE((SELECT MAX(id) FROM likes), 0) + 1, false); -- Reset new likes PK sequence


-- Insert Test Users
INSERT INTO users (id, name, phone_number, gender, date_of_birth, latitude, longitude, media_urls, created_at) VALUES
    (1, 'Alice', '1110000001', 'woman',   '1995-05-15', 40.7128, -74.0060, '{"img_alice1.jpg", "vid_alice1.mp4", "img_alice2.png"}', NOW() - INTERVAL '10 days'), -- Added 3 media items
    (2, 'Bob',   '1110000002', 'man',     '1993-08-20', 40.7135, -74.0050, '{"img_bob1.jpg"}', NOW() - INTERVAL '5 days'),
    (3, 'Charlie','1110000003', 'man',     '1997-01-10', 40.7580, -73.9855, '{"img_charlie1.jpg"}', NOW() - INTERVAL '8 days'),
    (4, 'David', '1110000004', 'man',     '1994-11-05', 34.0522, -118.2437,'{"img_david1.jpg"}', NOW() - INTERVAL '12 days'),
    (5, 'Eve',   '1110000005', 'woman',   '1996-03-25', 40.7120, -74.0070, '{"img_eve1.jpg", "img_eve2.jpg"}', NOW() - INTERVAL '3 days'), -- Added 2 media items
    (6, 'Frank', '1110000006', 'man',     '1987-07-12', 40.7140, -74.0045, '{"img_frank1.jpg"}', NOW() - INTERVAL '20 days'),
    (7, 'Grace', '1110000007', 'man',     '1996-09-30', 40.7130, -74.0065, '{"img_grace1.jpg"}', NOW() - INTERVAL '6 days'),
    (8, 'Hank',  '1110000008', 'man',     '1994-06-18', 40.7125, -74.0055, '{"img_hank1.jpg"}', NOW() - INTERVAL '15 days'),
    (9, 'Ivy',   '1110000009', 'man',     '1992-12-22', 40.7132, -74.0048, '{"img_ivy1.jpg"}', NOW() - INTERVAL '18 days'),
    (10,'Jack',  '1110000010', 'man',     '1991-02-01', 40.7138, -74.0052, '{"img_jack1.jpg"}', NOW() - INTERVAL '25 days'),
    (11,'Ken',   '1110000011', 'man',     '1996-04-14', 40.7129, -74.0062, '{"img_ken1.jpg"}', NOW() - INTERVAL '9 days'),
    (12,'Liam',  '1110000012', 'man',     '1995-01-05', 40.7127, -74.0061, '{"img_liam1.jpg"}', NOW() - INTERVAL '2 days'),
    (13,'Mia',   '1110000013', 'man',     '1999-10-10', 40.7133, -74.0058, '{}', NOW() - INTERVAL '4 days'), -- No media
    (14,'Noah',  '1110000014', 'man',     '1990-03-15', 40.7142, -74.0040, '{"img_noah1.jpg"}', NOW() - INTERVAL '30 days'),
    (15,'Olivia','1110000015', 'man',     '1993-07-20', 40.7122, -74.0068, '{"img_olivia1.jpg"}', NOW() - INTERVAL '1 day'),
    (16,'Peter', '1110000016', 'man',     '1994-08-25', 40.7100, -74.0100, '{"img_peter1.jpg"}', NOW() - INTERVAL '7 days'),
    (17,'Quinn', '1110000017', 'man',     '1995-09-01', 40.6892, -74.0445, '{"img_quinn1.jpg"}', NOW() - INTERVAL '11 days');

-- Insert Filters
INSERT INTO filters (user_id, who_you_want_to_see, radius_km, active_today, age_min, age_max) VALUES
    (1, 'man', 20, true, 28, 36), (2, 'woman', 50, false, 28, 38), (3, 'woman', NULL, true, 25, 35),
    (4, 'woman', 100, true, 25, 40), (6, 'woman', 30, false, 30, 45), (7, 'man', 30, true, 25, 35),
    (8, 'woman', 40, true, 25, 35), (9, 'woman', 50, false, 28, 38), (10,'woman', 60, true, 29, 39),
    (11,'woman', 25, false, 25, 33), (12,'woman', 15, true, 26, 34), (13,'woman', NULL, true, 22, 30),
    (14,'woman', 20, true, 30, 40), (15,'woman', 25, true, 27, 35), (17,'woman', 50, true, 26, 36);

-- Insert App Open Logs
INSERT INTO app_open_logs (user_id, opened_at) VALUES
    (2, NOW()), (3, NOW() - INTERVAL '5 hour'), (4, NOW()), (6, NOW() - INTERVAL '10 minute'),
    (7, NOW()), (8, NOW() - INTERVAL '1 hour'), (9, NOW()), (10, NOW() - INTERVAL '2 hour'),
    (11, NOW() - INTERVAL '3 day'), (12, NOW() - INTERVAL '5 minute'), (13, NOW()),
    (14, NOW() - INTERVAL '1 day'), (15, NOW() - INTERVAL '30 minute'), (16, NOW()), (17, NOW() - INTERVAL '6 hour');

-- Setup Premium Features
INSERT INTO user_subscriptions (user_id, feature_type, expires_at) VALUES
    (2, 'unlimited_likes', NOW() + INTERVAL '7 days');
INSERT INTO user_consumables (user_id, consumable_type, quantity) VALUES
    (3, 'rose', 5);

-- *** UPDATED Seed Like Data ***
-- Alice (ID 1) liked Bob's (ID 2) first photo (index 0) with a comment
INSERT INTO likes (liker_user_id, liked_user_id, content_type, content_identifier, comment, interaction_type) VALUES
    (1, 2, 'media', '0', 'Great pic!', 'standard'); -- Identifier is "0" for the first media item

-- Insert Dislikes involving Alice (ID 1)
INSERT INTO dislikes (disliker_user_id, disliked_user_id) VALUES
    (1, 9),   -- Alice disliked Ivy
    (10, 1);  -- Jack disliked Alice

-- Commit Transaction
COMMIT;

-- Optional: Query to verify data after running
-- SELECT u.id, u.name, u.media_urls FROM users u WHERE u.id BETWEEN 1 AND 20 ORDER BY u.id;
-- SELECT * FROM likes;
-- SELECT * FROM dislikes;
-- SELECT * FROM user_subscriptions;
-- SELECT * FROM user_consumables;
