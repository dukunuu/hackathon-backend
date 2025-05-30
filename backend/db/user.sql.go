// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.29.0
// source: user.sql

package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

const createUser = `-- name: CreateUser :one
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
RETURNING id, first_name, last_name, phone, is_volunteering, email, role, profile_url, password_hash, created_at, updated_at
`

type CreateUserParams struct {
	FirstName      string
	LastName       string
	Phone          pgtype.Text
	IsVolunteering bool
	Email          string
	Role           UserRole
	ProfileUrl     pgtype.Text
	PasswordHash   string
}

// Inserts a new user and returns the newly created user record.
// id, created_at, and updated_at have defaults and are not in the INSERT list.
func (q *Queries) CreateUser(ctx context.Context, arg CreateUserParams) (User, error) {
	row := q.db.QueryRow(ctx, createUser,
		arg.FirstName,
		arg.LastName,
		arg.Phone,
		arg.IsVolunteering,
		arg.Email,
		arg.Role,
		arg.ProfileUrl,
		arg.PasswordHash,
	)
	var i User
	err := row.Scan(
		&i.ID,
		&i.FirstName,
		&i.LastName,
		&i.Phone,
		&i.IsVolunteering,
		&i.Email,
		&i.Role,
		&i.ProfileUrl,
		&i.PasswordHash,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const deleteUser = `-- name: DeleteUser :exec
DELETE FROM users
WHERE id = $1
`

// Deletes a user by ID.
// :exec indicates it doesn't return rows.
func (q *Queries) DeleteUser(ctx context.Context, id pgtype.UUID) error {
	_, err := q.db.Exec(ctx, deleteUser, id)
	return err
}

const getUserByEmail = `-- name: GetUserByEmail :one
SELECT id, first_name, last_name, phone, is_volunteering, email, role, profile_url, password_hash, created_at, updated_at FROM users
WHERE email = $1
`

// Useful for checking if email exists or fetching user details by email
func (q *Queries) GetUserByEmail(ctx context.Context, email string) (User, error) {
	row := q.db.QueryRow(ctx, getUserByEmail, email)
	var i User
	err := row.Scan(
		&i.ID,
		&i.FirstName,
		&i.LastName,
		&i.Phone,
		&i.IsVolunteering,
		&i.Email,
		&i.Role,
		&i.ProfileUrl,
		&i.PasswordHash,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const getUserByID = `-- name: GetUserByID :one

SELECT id, first_name, last_name, phone, is_volunteering, email, role, profile_url, password_hash, created_at, updated_at FROM users
WHERE id = $1
`

// queries.sql
func (q *Queries) GetUserByID(ctx context.Context, id pgtype.UUID) (User, error) {
	row := q.db.QueryRow(ctx, getUserByID, id)
	var i User
	err := row.Scan(
		&i.ID,
		&i.FirstName,
		&i.LastName,
		&i.Phone,
		&i.IsVolunteering,
		&i.Email,
		&i.Role,
		&i.ProfileUrl,
		&i.PasswordHash,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const listUsers = `-- name: ListUsers :many
SELECT id, first_name, last_name, phone, is_volunteering, email, role, profile_url, password_hash, created_at, updated_at FROM users
ORDER BY created_at DESC
`

// Selects all users. Consider adding pagination parameters (LIMIT, OFFSET)
// for real-world applications if the user list can be large.
func (q *Queries) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := q.db.Query(ctx, listUsers)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []User
	for rows.Next() {
		var i User
		if err := rows.Scan(
			&i.ID,
			&i.FirstName,
			&i.LastName,
			&i.Phone,
			&i.IsVolunteering,
			&i.Email,
			&i.Role,
			&i.ProfileUrl,
			&i.PasswordHash,
			&i.CreatedAt,
			&i.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const loginRequest = `-- name: LoginRequest :one
SELECT id, email, password_hash, role FROM users
WHERE email = $1
`

type LoginRequestRow struct {
	ID           pgtype.UUID
	Email        string
	PasswordHash string
	Role         UserRole
}

// Specifically for authentication: fetches only necessary fields
// The application should then verify the password_hash
func (q *Queries) LoginRequest(ctx context.Context, email string) (LoginRequestRow, error) {
	row := q.db.QueryRow(ctx, loginRequest, email)
	var i LoginRequestRow
	err := row.Scan(
		&i.ID,
		&i.Email,
		&i.PasswordHash,
		&i.Role,
	)
	return i, err
}

const updateUserDetails = `-- name: UpdateUserDetails :one
UPDATE users
SET
    first_name = $2,
    last_name = $3,
    phone = $4,
    is_volunteering = $5,
    profile_url = $6
WHERE id = $1
RETURNING id, first_name, last_name, phone, is_volunteering, email, role, profile_url, password_hash, created_at, updated_at
`

type UpdateUserDetailsParams struct {
	ID             pgtype.UUID
	FirstName      string
	LastName       string
	Phone          pgtype.Text
	IsVolunteering bool
	ProfileUrl     pgtype.Text
}

// Updates general user details.
// Excludes password_hash and role, which might have separate update logic.
func (q *Queries) UpdateUserDetails(ctx context.Context, arg UpdateUserDetailsParams) (User, error) {
	row := q.db.QueryRow(ctx, updateUserDetails,
		arg.ID,
		arg.FirstName,
		arg.LastName,
		arg.Phone,
		arg.IsVolunteering,
		arg.ProfileUrl,
	)
	var i User
	err := row.Scan(
		&i.ID,
		&i.FirstName,
		&i.LastName,
		&i.Phone,
		&i.IsVolunteering,
		&i.Email,
		&i.Role,
		&i.ProfileUrl,
		&i.PasswordHash,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const updateUserEmail = `-- name: UpdateUserEmail :one
UPDATE users
SET
    email = $2,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING id, first_name, last_name, phone, is_volunteering, email, role, profile_url, password_hash, created_at, updated_at
`

type UpdateUserEmailParams struct {
	ID    pgtype.UUID
	Email string
}

// Specific query to update user email.
// Requires careful handling in application (e.g., re-verification).
func (q *Queries) UpdateUserEmail(ctx context.Context, arg UpdateUserEmailParams) (User, error) {
	row := q.db.QueryRow(ctx, updateUserEmail, arg.ID, arg.Email)
	var i User
	err := row.Scan(
		&i.ID,
		&i.FirstName,
		&i.LastName,
		&i.Phone,
		&i.IsVolunteering,
		&i.Email,
		&i.Role,
		&i.ProfileUrl,
		&i.PasswordHash,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const updateUserPassword = `-- name: UpdateUserPassword :one
UPDATE users
SET
    password_hash = $2,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING id, first_name, last_name, phone, is_volunteering, email, role, profile_url, password_hash, created_at, updated_at
`

type UpdateUserPasswordParams struct {
	ID           pgtype.UUID
	PasswordHash string
}

// Specific query to update user password hash.
func (q *Queries) UpdateUserPassword(ctx context.Context, arg UpdateUserPasswordParams) (User, error) {
	row := q.db.QueryRow(ctx, updateUserPassword, arg.ID, arg.PasswordHash)
	var i User
	err := row.Scan(
		&i.ID,
		&i.FirstName,
		&i.LastName,
		&i.Phone,
		&i.IsVolunteering,
		&i.Email,
		&i.Role,
		&i.ProfileUrl,
		&i.PasswordHash,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}
