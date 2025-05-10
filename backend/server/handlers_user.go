package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/dukunuu/hackathon_backend/db" // ADJUST THIS IMPORT PATH
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const maxUploadSize = 5 * 1024 * 1024 // 5 MB
var allowedMimeTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
}

// CreateUserRequest defines the expected JSON body for creating a user.
// swagger:model CreateUserRequest
type CreateUserRequest struct {
	FirstName      string `json:"first_name" example:"John"`
	LastName       string `json:"last_name" example:"Doe"`
	Phone          string `json:"phone,omitempty" example:"123-456-7890"`
	IsVolunteering bool   `json:"is_volunteering" example:"false"`
	Email          string `json:"email" example:"john.doe@example.com"`
	Role           string `json:"role" example:"user"`
	ProfileUrl     string `json:"profile_url,omitempty" example:"http://example.com/profile.jpg"`
	Password       string `json:"password" example:"strongpassword123"`
}

// LoginRequestPayload defines the expected JSON body for login.
// swagger:model LoginRequestPayload
type LoginRequestPayload struct {
	Email    string `json:"email" example:"john.doe@example.com"`
	Password string `json:"password" example:"strongpassword123"`
}

// UpdateUserDetailsRequest defines the expected JSON body for updating user details.
// swagger:model UpdateUserDetailsRequest
type UpdateUserDetailsRequest struct {
	FirstName      string `json:"first_name" example:"John"`
	LastName       string `json:"last_name" example:"Doe"`
	Phone          string `json:"phone,omitempty" example:"123-456-7890"`
	IsVolunteering bool   `json:"is_volunteering" example:"true"`
	ProfileUrl     string `json:"profile_url,omitempty" example:"http://example.com/new_profile.jpg"`
}

// UpdateUserEmailRequest defines the expected JSON body for updating user email.
// swagger:model UpdateUserEmailRequest
type UpdateUserEmailRequest struct {
	Email string `json:"email" example:"new.email@example.com"`
}

// UpdateUserPasswordRequest defines the expected JSON body for updating user password.
// swagger:model UpdateUserPasswordRequest
type UpdateUserPasswordRequest struct {
	NewPassword string `json:"new_password" example:"newStrongPassword456"`
}

// handleCreateUser creates a new user, optionally with a profile picture.
// @Summary Create a new user (with optional profile image)
// @Description Registers a new user. User details are sent as a JSON string in the 'userData' form field.
// @Description Optionally, a profile image can be uploaded via the 'profileImage' form field.
// @Tags Users
// @Accept multipart/form-data
// @Produce json
// @Param userData formData string true "User registration details as a JSON string. Example: '{\"first_name\":\"John\", \"last_name\":\"Doe\", \"email\":\"john.doe@example.com\", \"password\":\"secure123\", \"role\":\"user\"}'"
// @Param profileImage formData file false "Optional profile image file (max 5MB, types: jpeg, png, gif, webp)"
// @Success 201 {object} UserResponseDTO "Successfully created user"
// @Failure 400 {object} ErrorResponse "Invalid request (e.g., missing 'userData', invalid JSON, invalid image, missing required fields in userData)"
// @Failure 409 {object} ErrorResponse "User with this email already exists"
// @Failure 500 {object} ErrorResponse "Internal server error (e.g., failed to hash password, upload image, or create user)"
// @Router /users/register [post]
func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxUploadSize + 1*1024*1024); err != nil { // e.g., file size + 1MB for other fields
		slog.Error("Failed to parse multipart form", "error", err)
		respondWithError(w, http.StatusBadRequest, "Could not parse form: "+err.Error())
		return
	}

	// 1. Get and parse the 'userData' JSON string field
	userDataJSON := r.FormValue("userData")
	if userDataJSON == "" {
		respondWithError(w, http.StatusBadRequest, "Missing 'userData' form field.")
		return
	}

	var req CreateUserRequest
	if err := json.Unmarshal([]byte(userDataJSON), &req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON in 'userData' field: "+err.Error())
		return
	}

	// Validate required fields from the parsed JSON
	if req.Email == "" || req.Password == "" || req.FirstName == "" || req.LastName == "" || req.Role == "" {
		respondWithError(w, http.StatusBadRequest, "Missing required fields in 'userData': email, password, first_name, last_name, role")
		return
	}

	// 2. Hash password
	hashedPassword, err := hashPassword(req.Password)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	// 3. Handle optional profile image upload
	var uploadedProfileURL string
	file, handler, err := r.FormFile("profileImage")
	if err != nil && !errors.Is(err, http.ErrMissingFile) {
		// An error other than "missing file" occurred
		slog.Error("Error retrieving profileImage from form", "error", err)
		respondWithError(w, http.StatusBadRequest, "Error processing profileImage: "+err.Error())
		return
	}

	if file != nil { // A file was provided
		defer file.Close()

		if handler.Size > maxUploadSize {
			respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Profile image too large. Maximum size is %dMB.", maxUploadSize/(1024*1024)))
			return
		}

		buffer := make([]byte, 512)
		_, readErr := file.Read(buffer)
		if readErr != nil && readErr != io.EOF {
			slog.Error("Failed to read profileImage for MIME type detection", "error", readErr)
			respondWithError(w, http.StatusInternalServerError, "Could not read profileImage for type detection.")
			return
		}
		_, seekErr := file.Seek(0, io.SeekStart)
		if seekErr != nil {
			slog.Error("Failed to seek profileImage after MIME type detection", "error", seekErr)
			respondWithError(w, http.StatusInternalServerError, "Could not process profileImage.")
			return
		}

		contentType := http.DetectContentType(buffer)
		if !allowedMimeTypes[contentType] {
			slog.Warn("Invalid profileImage type uploaded", "contentType", contentType, "filename", handler.Filename)
			respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Invalid profileImage type: %s. Allowed types: jpeg, png, gif, webp.", contentType))
			return
		}

		fileBytes, readAllErr := io.ReadAll(file)
		if readAllErr != nil {
			slog.Error("Failed to read profileImage into buffer", "error", readAllErr)
			respondWithError(w, http.StatusInternalServerError, "Could not read profileImage content.")
			return
		}

		tempUserIDForPath := uuid.New() // Generate a UUID to use in the path for now
		uploadedURL, _, uploadErr := s.filestore.UploadProfileImage(r.Context(), fileBytes, handler.Filename, contentType, tempUserIDForPath)
		if uploadErr != nil {
			slog.Error("Failed to upload profile image via filestore during user creation", "error", uploadErr)
			respondWithError(w, http.StatusInternalServerError, "Could not upload profile image: "+uploadErr.Error())
			return
		}
		uploadedProfileURL = uploadedURL
		slog.Info("Successfully uploaded profile image during user creation", "tempUserIDForPath", tempUserIDForPath, "url", uploadedProfileURL)
	}

	finalProfileURL := uploadedProfileURL
	if finalProfileURL == "" { // No image uploaded, check if a URL was provided in JSON
		finalProfileURL = req.ProfileUrl
	}

	var roleValueForDB = req.Role

	params := db.CreateUserParams{
		FirstName:      req.FirstName,
		LastName:       req.LastName,
		Phone:          toPgtypeText(req.Phone),
		IsVolunteering: req.IsVolunteering,
		Email:          req.Email,
		Role:           db.UserRole(roleValueForDB),
		ProfileUrl:     toPgtypeText(finalProfileURL),
		PasswordHash:   hashedPassword,
	}

	// 5. Create user in database
	user, err := s.db.CreateUser(r.Context(), params)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // Unique violation
			respondWithError(w, http.StatusConflict, "User with this email already exists")
			return
		}
		slog.Error("Failed to create user in DB", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to create user: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, ToUserResponseDTO(user))
}

// handleLogin authenticates a user and returns a JWT.
// @Summary Login a user
// @Description Authenticates a user with email and password, returns a JWT token and user details.
// @Tags Authentication
// @Accept json
// @Produce json
// @Param credentials body LoginRequestPayload true "User login credentials"
// @Success 200 {object} LoginResponsePayloadDTO "Successfully logged in"
// @Failure 400 {object} ErrorResponse "Invalid request payload or missing fields"
// @Failure 401 {object} ErrorResponse "Invalid email or password"
// @Failure 500 {object} ErrorResponse "Login failed or failed to generate token"
// @Router /users/login [post]
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequestPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	if req.Email == "" || req.Password == "" {
		respondWithError(w, http.StatusBadRequest, "Email and password are required")
		return
	}

	loginRow, err := s.db.LoginRequest(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			respondWithError(w, http.StatusUnauthorized, "Invalid email or password")
			return
		}
		slog.Error("Login failed during LoginRequest", "error", err, "email", req.Email)
		respondWithError(w, http.StatusInternalServerError, "Login failed: "+err.Error())
		return
	}

	if err := verifyPassword(loginRow.PasswordHash, req.Password); err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	fullUser, err := s.db.GetUserByID(r.Context(), loginRow.ID)
	if err != nil {
		slog.Error("Failed to fetch full user details post-login", "error", err, "userID", loginRow.ID)
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve user details after login")
		return
	}

	expirationTime := time.Now().Add(24 * time.Hour)
	tokenString, err := generateJWT(loginRow.ID, loginRow.Email, loginRow.Role, s.jwtSecret, expirationTime)
	if err != nil {
		slog.Error("Failed to generate token", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}
	respondWithJSON(w, http.StatusOK, ToLoginResponsePayloadDTO(tokenString, fullUser))
}

// handleGetUserByID fetches a user by their ID.
// @Summary Get user by ID
// @Description Retrieves user details for a given user ID.
// @Tags Users
// @Produce json
// @Param userID path string true "User ID" format(uuid)
// @Success 200 {object} UserResponseDTO "Successfully retrieved user"
// @Failure 400 {object} ErrorResponse "Invalid user ID format"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 404 {object} ErrorResponse "User not found"
// @Failure 500 {object} ErrorResponse "Failed to get user"
// @Security BearerAuth
// @Router /users/{userID} [get]
func (s *Server) handleGetUserByID(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUUIDFromParam(r, "userID")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	user, err := s.db.GetUserByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "User not found")
			return
		}
		slog.Error("Failed to get user by ID", "error", err, "userID", userID)
		respondWithError(w, http.StatusInternalServerError, "Failed to get user: "+err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, ToUserResponseDTO(user))
}

// handleGetCurrentUser fetches the currently authenticated user's details.
// @Summary Get current user
// @Description Retrieves details for the currently authenticated user.
// @Tags Users
// @Produce json
// @Success 200 {object} UserResponseDTO "Successfully retrieved user"
// @Failure 401 {object} ErrorResponse "Authentication required or user not found"
// @Failure 500 {object} ErrorResponse "Failed to get user"
// @Security BearerAuth
// @Router /users/me [get]
func (s *Server) handleGetCurrentUser(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromContext(r.Context())
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required: "+err.Error())
		return
	}

	user, err := s.db.GetUserByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			slog.Warn("Authenticated user not found in DB", "userID", userID, "error", err)
			respondWithError(w, http.StatusNotFound, "User not found")
			return
		}
		slog.Error("Failed to get current user", "error", err, "userID", userID)
		respondWithError(w, http.StatusInternalServerError, "Failed to get user: "+err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, ToUserResponseDTO(user))
}

// handleListUsers lists all users.
// @Summary List all users
// @Description Retrieves a list of all users. (Note: Consider pagination for large datasets and admin-only access)
// @Tags Users
// @Produce json
// @Success 200 {array} UserResponseDTO "Successfully retrieved list of users"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Failed to list users"
// @Security BearerAuth
// @Router /users [get]
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.db.ListUsers(r.Context())
	if err != nil {
		slog.Error("Failed to list users", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to list users: "+err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, ToUserResponseDTOs(users))
}

// handleDeleteUser deletes a user by their ID.
// @Summary Delete user by ID
// @Description Deletes a user account. Authenticated user can delete their own account. (Admin functionality can be added).
// @Tags Users
// @Produce json
// @Param userID path string true "User ID of the account to delete" format(uuid)
// @Success 200 {object} map[string]string "message: User deleted successfully"
// @Failure 400 {object} ErrorResponse "Invalid user ID format"
// @Failure 401 {object} ErrorResponse "Authentication required"
// @Failure 403 {object} ErrorResponse "Forbidden - cannot delete another user's account"
// @Failure 404 {object} ErrorResponse "User not found to delete"
// @Failure 500 {object} ErrorResponse "Failed to delete user"
// @Security BearerAuth
// @Router /users/{userID} [delete]
func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	userIDToDelete, err := parseUUIDFromParam(r, "userID")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	authUserID, err := getUserIDFromContext(r.Context())
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var authUUIDBytes = authUserID.Bytes
	var targetUUIDBytes = userIDToDelete.Bytes

	if authUUIDBytes != targetUUIDBytes {
		respondWithError(w, http.StatusForbidden, "You can only delete your own account or you lack admin privileges.")
		return
	}

	_, err = s.db.GetUserByID(r.Context(), userIDToDelete)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "User not found to delete")
			return
		}
		slog.Error("Error checking user before delete", "error", err, "userIDToDelete", userIDToDelete)
		respondWithError(w, http.StatusInternalServerError, "Error checking user before deletion: "+err.Error())
		return
	}

	err = s.db.DeleteUser(r.Context(), userIDToDelete)
	if err != nil {
		slog.Error("Failed to delete user", "error", err, "userIDToDelete", userIDToDelete)
		respondWithError(w, http.StatusInternalServerError, "Failed to delete user: "+err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]string{"message": "User deleted successfully"})
}

// handleUpdateUserDetails updates details for the authenticated user.
// @Summary Update current user's details
// @Description Updates the first name, last name, phone, volunteering status, and profile URL for the authenticated user.
// @Tags Users
// @Accept json
// @Produce json
// @Param userDetails body UpdateUserDetailsRequest true "User details to update"
// @Success 200 {object} UserResponseDTO "Successfully updated user details"
// @Failure 400 {object} ErrorResponse "Invalid request payload"
// @Failure 401 {object} ErrorResponse "Authentication required"
// @Failure 404 {object} ErrorResponse "User not found to update"
// @Failure 500 {object} ErrorResponse "Failed to update user details"
// @Security BearerAuth
// @Router /users/me/details [put]
func (s *Server) handleUpdateUserDetails(w http.ResponseWriter, r *http.Request) {
	targetUserID, err := getUserIDFromContext(r.Context())
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req UpdateUserDetailsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	params := db.UpdateUserDetailsParams{
		ID:             targetUserID,
		FirstName:      req.FirstName,
		LastName:       req.LastName,
		Phone:          toPgtypeText(req.Phone),
		IsVolunteering: req.IsVolunteering,
		ProfileUrl:     toPgtypeText(req.ProfileUrl),
	}

	updatedUser, err := s.db.UpdateUserDetails(r.Context(), params)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "User not found to update")
			return
		}
		slog.Error("Failed to update user details", "error", err, "userID", targetUserID)
		respondWithError(w, http.StatusInternalServerError, "Failed to update user details: "+err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, ToUserResponseDTO(updatedUser))
}

// handleUpdateUserEmail updates the email for the authenticated user.
// @Summary Update current user's email
// @Description Updates the email address for the authenticated user. May require re-verification in a real application.
// @Tags Users
// @Accept json
// @Produce json
// @Param emailUpdate body UpdateUserEmailRequest true "New email address"
// @Success 200 {object} UserResponseDTO "Successfully updated user email"
// @Failure 400 {object} ErrorResponse "Invalid request payload or email empty"
// @Failure 401 {object} ErrorResponse "Authentication required"
// @Failure 404 {object} ErrorResponse "User not found to update email"
// @Failure 409 {object} ErrorResponse "This email is already in use"
// @Failure 500 {object} ErrorResponse "Failed to update user email"
// @Security BearerAuth
// @Router /users/me/email [put]
func (s *Server) handleUpdateUserEmail(w http.ResponseWriter, r *http.Request) {
	targetUserID, err := getUserIDFromContext(r.Context())
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req UpdateUserEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	if req.Email == "" {
		respondWithError(w, http.StatusBadRequest, "Email cannot be empty")
		return
	}

	params := db.UpdateUserEmailParams{
		ID:    targetUserID,
		Email: req.Email,
	}

	updatedUser, err := s.db.UpdateUserEmail(r.Context(), params)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			respondWithError(w, http.StatusConflict, "This email is already in use.")
			return
		}
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "User not found to update email")
			return
		}
		slog.Error("Failed to update user email", "error", err, "userID", targetUserID)
		respondWithError(w, http.StatusInternalServerError, "Failed to update user email: "+err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, ToUserResponseDTO(updatedUser))
}

// handleUpdateUserPassword updates the password for the authenticated user.
// @Summary Update current user's password
// @Description Updates the password for the authenticated user.
// @Tags Users
// @Accept json
// @Produce json
// @Param passwordUpdate body UpdateUserPasswordRequest true "New password"
// @Success 200 {object} map[string]string "message: Password updated successfully"
// @Failure 400 {object} ErrorResponse "Invalid request payload or new password empty"
// @Failure 401 {object} ErrorResponse "Authentication required"
// @Failure 404 {object} ErrorResponse "User not found to update password"
// @Failure 500 {object} ErrorResponse "Failed to hash new password or update user password"
// @Security BearerAuth
// @Router /users/me/password [put]
func (s *Server) handleUpdateUserPassword(w http.ResponseWriter, r *http.Request) {
	targetUserID, err := getUserIDFromContext(r.Context())
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req UpdateUserPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	if req.NewPassword == "" {
		respondWithError(w, http.StatusBadRequest, "New password cannot be empty")
		return
	}

	newHashedPassword, err := hashPassword(req.NewPassword)
	if err != nil {
		slog.Error("Failed to hash new password", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to hash new password")
		return
	}

	params := db.UpdateUserPasswordParams{
		ID:           targetUserID,
		PasswordHash: newHashedPassword,
	}

	_, err = s.db.UpdateUserPassword(r.Context(), params)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "User not found to update password")
			return
		}
		slog.Error("Failed to update user password", "error", err, "userID", targetUserID)
		respondWithError(w, http.StatusInternalServerError, "Failed to update user password: "+err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Password updated successfully"})
}

// handleGetUserByEmail fetches a user by their email address.
// @Summary Get user by email
// @Description Retrieves user details for a given email address. (Consider admin-only access).
// @Tags Users
// @Produce json
// @Param email query string true "Email address of the user" example("jane.doe@example.com")
// @Success 200 {object} UserResponseDTO "Successfully retrieved user"
// @Failure 400 {object} ErrorResponse "Email query parameter is required"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 404 {object} ErrorResponse "User with this email not found"
// @Failure 500 {object} ErrorResponse "Failed to get user by email"
// @Security BearerAuth
// @Router /users/by-email [get]
func (s *Server) handleGetUserByEmail(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" {
		respondWithError(w, http.StatusBadRequest, "Email query parameter is required")
		return
	}

	user, err := s.db.GetUserByEmail(r.Context(), email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "User with this email not found")
			return
		}
		slog.Error("Failed to get user by email", "error", err, "email", email)
		respondWithError(w, http.StatusInternalServerError, "Failed to get user by email: "+err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, ToUserResponseDTO(user))
}

