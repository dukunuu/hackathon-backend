-- queries.sql

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
-- Useful for checking if email exists or fetching user details by email
SELECT * FROM users
WHERE email = $1;

-- name: LoginRequest :one
-- Specifically for authentication: fetches only necessary fields
-- The application should then verify the password_hash
SELECT id, email, password_hash, role FROM users
WHERE email = $1;

-- name: ListUsers :many
-- Selects all users. Consider adding pagination parameters (LIMIT, OFFSET)
-- for real-world applications if the user list can be large.
SELECT * FROM users
ORDER BY created_at DESC;

-- name: CreateUser :one
-- Inserts a new user and returns the newly created user record.
-- id, created_at, and updated_at have defaults and are not in the INSERT list.
INSERT INTO users (
    first_name,
    last_name,
    phone,
    is_volunteering,
    email,
    role,
    profile_url,
    password_hash
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: UpdateUserDetails :one
-- Updates general user details.
-- Excludes password_hash and role, which might have separate update logic.
UPDATE users
SET
    first_name = $2,
    last_name = $3,
    phone = $4,
    is_volunteering = $5,
    profile_url = $6
WHERE id = $1
RETURNING *;

-- name: UpdateUserEmail :one
-- Specific query to update user email.
-- Requires careful handling in application (e.g., re-verification).
UPDATE users
SET
    email = $2,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: UpdateUserPassword :one
-- Specific query to update user password hash.
UPDATE users
SET
    password_hash = $2,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: DeleteUser :exec
-- Deletes a user by ID.
-- :exec indicates it doesn't return rows.
DELETE FROM users
WHERE id = $1;
