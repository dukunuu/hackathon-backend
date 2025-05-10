CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TYPE user_role AS ENUM ('ADMIN', 'USER');

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    phone CHAR(8) UNIQUE,
    is_volunteering BOOLEAN NOT NULL DEFAULT FALSE,
    email VARCHAR(255) NOT NULL UNIQUE,
    role user_role NOT NULL DEFAULT 'USER',
    profile_url TEXT,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT check_phone_format CHECK (phone IS NULL OR phone ~ '^[0-9]{8}$')
);

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
   NEW.updated_at = NOW();
   RETURN NEW;
END;
$$ language 'plpgsql';

DROP TRIGGER IF EXISTS trigger_users_updated_at ON users;

CREATE TRIGGER trigger_users_updated_at
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_phone ON users(phone);
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);

COMMENT ON COLUMN users.id IS 'Primary key, UUID';
COMMENT ON COLUMN users.first_name IS 'User''s first name';
COMMENT ON COLUMN users.last_name IS 'User''s last name';
COMMENT ON COLUMN users.phone IS 'User''s 8-digit phone number (optional, unique if provided)';
COMMENT ON COLUMN users.is_volunteering IS 'Flag indicating if the user is volunteering (default: false)';
COMMENT ON COLUMN users.email IS 'User''s email address (unique, mandatory)';
COMMENT ON COLUMN users.role IS 'User''s role in the system (admin or user, default: user)';
COMMENT ON COLUMN users.profile_url IS 'URL to the user''s profile picture or page (optional)';
COMMENT ON COLUMN users.password_hash IS 'Hashed password for the user (mandatory)';
COMMENT ON COLUMN users.created_at IS 'Timestamp of when the user record was created';
COMMENT ON COLUMN users.updated_at IS 'Timestamp of when the user record was last updated';
