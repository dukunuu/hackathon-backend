DROP TRIGGER IF EXISTS trigger_users_updated_at ON users;

DROP FUNCTION IF EXISTS update_updated_at_column();
DROP INDEX IF EXISTS idx_users_role;
DROP INDEX IF EXISTS idx_users_phone;
DROP INDEX IF EXISTS idx_users_email; -- This was explicitly created in the 'up'
DROP TABLE IF EXISTS users;
DROP TYPE IF EXISTS user_role;
