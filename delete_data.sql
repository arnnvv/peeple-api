-- Truncate script for clearing all data while preserving schema
-- First, disable foreign key checks during truncation
BEGIN;

-- Truncate tables with foreign keys first (in reverse dependency order)
TRUNCATE TABLE user_consumables CASCADE;
TRUNCATE TABLE user_subscriptions CASCADE;
TRUNCATE TABLE likes CASCADE;
TRUNCATE TABLE dislikes CASCADE;
TRUNCATE TABLE app_open_logs CASCADE;
TRUNCATE TABLE filters CASCADE;
TRUNCATE TABLE date_vibes_prompts CASCADE;
TRUNCATE TABLE getting_personal_prompts CASCADE;
TRUNCATE TABLE my_type_prompts CASCADE;
TRUNCATE TABLE story_time_prompts CASCADE;
TRUNCATE TABLE users CASCADE;

-- Commit the transaction
COMMIT;

-- Verify counts
SELECT 'users' as table_name, COUNT(*) as row_count FROM users
UNION ALL
SELECT 'story_time_prompts', COUNT(*) FROM story_time_prompts
UNION ALL
SELECT 'my_type_prompts', COUNT(*) FROM my_type_prompts
UNION ALL
SELECT 'getting_personal_prompts', COUNT(*) FROM getting_personal_prompts  
UNION ALL
SELECT 'date_vibes_prompts', COUNT(*) FROM date_vibes_prompts
UNION ALL
SELECT 'filters', COUNT(*) FROM filters
UNION ALL
SELECT 'app_open_logs', COUNT(*) FROM app_open_logs
UNION ALL
SELECT 'dislikes', COUNT(*) FROM dislikes
UNION ALL
SELECT 'likes', COUNT(*) FROM likes
UNION ALL
SELECT 'user_subscriptions', COUNT(*) FROM user_subscriptions
UNION ALL
SELECT 'user_consumables', COUNT(*) FROM user_consumables;
