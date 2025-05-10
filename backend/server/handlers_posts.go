// server/handlers_posts.go
package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/dukunuu/hackathon_backend/db" // ADJUST THIS IMPORT PATH
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func toPgtypeUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: id != uuid.Nil}
}

// handleGetUserPosts retrieves all posts for a specific user.
// @Summary Get posts by user ID
// @Description Retrieves all posts created by a specific user.
// @Tags Posts
// @Produce json
// @Param userId path string true "User ID" format(uuid)
// @Success 200 {array} PostResponseDTO "Successfully retrieved user's posts"
// @Failure 400 {object} ErrorResponse "Invalid user ID format"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Failed to retrieve user posts"
// @Security BearerAuth
// @Router /users/{userId}/posts [get]
func (s *Server) handleGetUserPosts(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.PathValue("userId")
	if userIDStr == "" {
		userIDStr = r.URL.Query().Get("userId")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid user ID format")
		return
	}

	posts, err := s.db.GetUserPosts(r.Context(), toPgtypeUUID(userID))
	if err != nil {
		slog.Error("Failed to get user posts", "error", err, "userID", userID)
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve user posts")
		return
	}

	var responseDTOs []PostResponseDTO
	for _, p := range posts {
		responseDTOs = append(responseDTOs, toPostResponseDTO(p))
	}
	respondWithJSON(w, http.StatusOK, responseDTOs)
}

// handleListPosts retrieves all posts with their images and volunteers.
// @Summary List all posts
// @Description Retrieves a list of all posts, including associated images and volunteers.
// @Tags Posts
// @Produce json
// @Success 200 {array} PostResponseDTO "Successfully retrieved list of posts"
// @Failure 500 {object} ErrorResponse "Failed to retrieve posts"
// @Router /posts [get]
func (s *Server) handleListPosts(w http.ResponseWriter, r *http.Request) {
	listPostsRows, err := s.db.ListPosts(r.Context())
	if err != nil {
		slog.Error("Failed to list posts", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve posts")
		return
	}

	var responseDTOs []PostResponseDTO
	for _, row := range listPostsRows {
		dto, err := toPostResponseDTOFromListPostsRow(row)
		if err != nil {
			slog.Error("Failed to process post row for listing", "postID", row.ID.Bytes, "error", err)
			continue
		}
		responseDTOs = append(responseDTOs, dto)
	}
	respondWithJSON(w, http.StatusOK, responseDTOs)
}

// handleCreatePost creates a new post.
// @Summary Create a new post
// @Description Creates a new post. The authenticated user will be the owner.
// @Tags Posts
// @Accept json
// @Produce json
// @Param postData body CreatePostRequest true "Post creation details"
// @Success 201 {object} PostResponseDTO "Successfully created post"
// @Failure 400 {object} ErrorResponse "Invalid request payload or missing required fields"
// @Failure 401 {object} ErrorResponse "Authentication required"
// @Failure 500 {object} ErrorResponse "Failed to create post"
// @Security BearerAuth
// @Router /posts [post]
func (s *Server) handleCreatePost(w http.ResponseWriter, r *http.Request) {
	authUserID, err := getUserIDFromContext(r.Context())
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req CreatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload: "+err.Error())
		return
	}
	defer r.Body.Close()

	if req.Title == "" || req.Description == "" || req.PostType == "" {
		respondWithError(w, http.StatusBadRequest, "Title, description, and post_type are required")
		return
	}

	params := db.CreatePostParams{
		Title:             req.Title,
		Description:       req.Description,
		Status:            db.PostStatus(req.Status),
		Priority:          db.PostPriority(req.Priority),
		PreviewUrl:        toPgtypeText(req.PreviewURL),
		PostType:          db.PostType(req.PostType),
		UserID:            authUserID,
		MaxVolunteers:     req.MaxVolunteers,
		CurrentVolunteers: 0,
		LocationLat:       toPgtypeFloat8(req.LocationLat),
		LocationLng:       toPgtypeFloat8(req.LocationLng),
		AddressText:       toPgtypeText(req.AddressText),
	}

	if string(req.Status) == "" { // Compare underlying string value
		params.Status = db.PostStatus("Хүлээгдэж байгаа")
	}
	if string(req.Priority) == "" { // Compare underlying string value
		params.Priority = db.PostPriority("бага")
	}

	createdPost, err := s.db.CreatePost(r.Context(), params)
	if err != nil {
		slog.Error("Failed to create post", "error", err, "userID", authUserID.Bytes)
		respondWithError(w, http.StatusInternalServerError, "Failed to create post: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, toPostResponseDTO(createdPost))
}

// handleGetPost retrieves a single post by its ID.
// @Summary Get post by ID
// @Description Retrieves details for a specific post, including images and volunteers.
// @Tags Posts
// @Produce json
// @Param postId path string true "Post ID" format(uuid)
// @Success 200 {object} PostResponseDTO "Successfully retrieved post"
// @Failure 400 {object} ErrorResponse "Invalid post ID format"
// @Failure 404 {object} ErrorResponse "Post not found"
// @Failure 500 {object} ErrorResponse "Failed to retrieve post"
// @Router /posts/{postId} [get]
func (s *Server) handleGetPost(w http.ResponseWriter, r *http.Request) {
	postIDstr := r.PathValue("postId")
	postID, err := uuid.Parse(postIDstr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid post ID format")
		return
	}

	postRow, err := s.db.GetPost(r.Context(), toPgtypeUUID(postID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Post not found")
			return
		}
		slog.Error("Failed to get post", "error", err, "postID", postID)
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve post")
		return
	}

	dto, err := toPostResponseDTOFromGetPostRow(postRow)
	if err != nil {
		slog.Error("Failed to process post row for get by ID", "postID", postID, "error", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to process post data")
		return
	}
	respondWithJSON(w, http.StatusOK, dto)
}

// handleDeletePost deletes a post by its ID.
// @Summary Delete post by ID
// @Description Deletes a specific post. Only the owner of the post can delete it.
// @Tags Posts
// @Produce json
// @Param postId path string true "Post ID" format(uuid)
// @Success 200 {object} map[string]string "message: Post deleted successfully"
// @Failure 400 {object} ErrorResponse "Invalid post ID format"
// @Failure 401 {object} ErrorResponse "Authentication required"
// @Failure 403 {object} ErrorResponse "Forbidden - not authorized to delete this post"
// @Failure 404 {object} ErrorResponse "Post not found"
// @Failure 500 {object} ErrorResponse "Failed to delete post"
// @Security BearerAuth
// @Router /posts/{postId} [delete]
func (s *Server) handleDeletePost(w http.ResponseWriter, r *http.Request) {
	authUserID, err := getUserIDFromContext(r.Context())
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	postIDstr := r.PathValue("postId")
	postID, err := uuid.Parse(postIDstr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid post ID format")
		return
	}

	post, err := s.db.GetPost(r.Context(), toPgtypeUUID(postID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Post not found")
			return
		}
		slog.Error("Failed to get post for delete authorization", "error", err, "postID", postID)
		respondWithError(w, http.StatusInternalServerError, "Could not verify post ownership")
		return
	}

	if post.UserID.Bytes != authUserID.Bytes {
		respondWithError(w, http.StatusForbidden, "You are not authorized to delete this post")
		return
	}

	err = s.db.DeletePost(r.Context(), toPgtypeUUID(postID))
	if err != nil {
		slog.Error("Failed to delete post", "error", err, "postID", postID)
		respondWithError(w, http.StatusInternalServerError, "Failed to delete post")
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Post deleted successfully"})
}

// handleUpdatePost updates an existing post.
// @Summary Update post by ID
// @Description Updates details of a specific post. Only the owner of the post can update it.
// @Tags Posts
// @Accept json
// @Produce json
// @Param postId path string true "Post ID" format(uuid)
// @Param postUpdateData body UpdatePostRequest true "Post update details"
// @Success 200 {object} PostResponseDTO "Successfully updated post"
// @Failure 400 {object} ErrorResponse "Invalid request payload or post ID format"
// @Failure 401 {object} ErrorResponse "Authentication required"
// @Failure 403 {object} ErrorResponse "Forbidden - not authorized to update this post"
// @Failure 404 {object} ErrorResponse "Post not found to update"
// @Failure 500 {object} ErrorResponse "Failed to update post"
// @Security BearerAuth
// @Router /posts/{postId} [put]
func (s *Server) handleUpdatePost(w http.ResponseWriter, r *http.Request) {
	authUserID, err := getUserIDFromContext(r.Context())
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	postIDstr := r.PathValue("postId")
	postID, err := uuid.Parse(postIDstr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid post ID format")
		return
	}

	existingPost, err := s.db.GetPost(r.Context(), toPgtypeUUID(postID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Post not found to update")
			return
		}
		slog.Error("Failed to get post for update authorization", "error", err, "postID", postID)
		respondWithError(w, http.StatusInternalServerError, "Could not verify post ownership for update")
		return
	}

	if existingPost.UserID.Bytes != authUserID.Bytes {
		respondWithError(w, http.StatusForbidden, "You are not authorized to update this post")
		return
	}

	var req UpdatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload: "+err.Error())
		return
	}
	defer r.Body.Close()

	params := db.UpdatePostParams{
		ID:                toPgtypeUUID(postID),
		Title:             req.Title,
		Description:       req.Description,
		Status:            db.PostStatus(req.Status),
		Priority:          db.PostPriority(req.Priority),
		PreviewUrl:        toPgtypeText(req.PreviewURL),
		PostType:          db.PostType(req.PostType),
		UserID:            authUserID,
		MaxVolunteers:     req.MaxVolunteers,
		CurrentVolunteers: req.CurrentVolunteers,
		CategoryID:        toPgtypeUUID(req.CategoryID),
		LocationLat:       toPgtypeFloat8(req.LocationLat),
		LocationLng:       toPgtypeFloat8(req.LocationLng),
		AddressText:       toPgtypeText(req.AddressText),
	}

	updatedPost, err := s.db.UpdatePost(r.Context(), params)
	if err != nil {
		slog.Error("Failed to update post", "error", err, "postID", postID)
		respondWithError(w, http.StatusInternalServerError, "Failed to update post: "+err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, toPostResponseDTO(updatedPost))
}

// handleListPostVolunteers lists all volunteer applications.
// @Summary List all post volunteers
// @Description Retrieves a list of all volunteer applications across all posts. (Consider admin-only access and pagination).
// @Tags Posts, Volunteers
// @Produce json
// @Success 200 {array} PostVolunteerDTO "Successfully retrieved list of post volunteers"
// @Failure 401 {object} ErrorResponse "Authentication required"
// @Failure 500 {object} ErrorResponse "Failed to retrieve post volunteers"
// @Security BearerAuth
// @Router /posts/volunteers [get]
func (s *Server) handleListPostVolunteers(w http.ResponseWriter, r *http.Request) {
	volunteers, err := s.db.ListPostVolunteers(r.Context())
	if err != nil {
		slog.Error("Failed to list post volunteers", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve post volunteers")
		return
	}

	var dtos []PostVolunteerDTO
	for _, v := range volunteers {
		dtos = append(dtos, PostVolunteerDTO{
			ID:        v.ID.Bytes,
			UserID:    v.UserID.Bytes,
			PostID:    v.PostID.Bytes,
			Status:    v.Status.String,
			Notes:     v.Notes.String,
			CreatedAt: v.CreatedAt.Time,
			UpdatedAt: v.UpdatedAt.Time,
		})
	}
	respondWithJSON(w, http.StatusOK, dtos)
}

// handleDeletePostVolunteer removes a volunteer from a post.
// @Summary Remove a volunteer from a post
// @Description Allows a post owner to remove a volunteer or a volunteer to remove their own application from a post.
// @Tags Posts, Volunteers
// @Produce json
// @Param userId path string true "Volunteer's User ID to remove" format(uuid)
// @Param postId query string true "Post ID from which to remove the volunteer" format(uuid)
// @Success 200 {object} map[string]string "message: Volunteer removed successfully"
// @Failure 400 {object} ErrorResponse "Invalid user ID or post ID format, or missing postId query parameter"
// @Failure 401 {object} ErrorResponse "Authentication required"
// @Failure 403 {object} ErrorResponse "Forbidden - not authorized to remove this volunteer"
// @Failure 404 {object} ErrorResponse "Post not found"
// @Failure 500 {object} ErrorResponse "Failed to delete post volunteer"
// @Security BearerAuth
// @Router /posts/volunteers/{userId} [delete]
func (s *Server) handleDeletePostVolunteer(w http.ResponseWriter, r *http.Request) {
	authUserID, err := getUserIDFromContext(r.Context())
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	volunteerIDStr := r.PathValue("userId")
	volunteerID, err := uuid.Parse(volunteerIDStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid volunteer user ID format")
		return
	}

	postIDStr := r.URL.Query().Get("postId")
	if postIDStr == "" {
		respondWithError(w, http.StatusBadRequest, "Missing 'postId' query parameter")
		return
	}
	postID, err := uuid.Parse(postIDStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid post ID format in query parameter")
		return
	}

	post, err := s.db.GetPost(r.Context(), toPgtypeUUID(postID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Post not found")
			return
		}
		slog.Error("Failed to get post for volunteer deletion authorization", "error", err, "postID", postID)
		respondWithError(w, http.StatusInternalServerError, "Could not verify post ownership")
		return
	}

	if post.UserID.Bytes != authUserID.Bytes {
		if volunteerID != authUserID.Bytes {
			respondWithError(w, http.StatusForbidden, "You are not authorized to remove this volunteer or application")
			return
		}
	}

	params := db.DeletePostVolunteerParams{
		PostID: toPgtypeUUID(postID),
		UserID: toPgtypeUUID(volunteerID),
	}

	err = s.db.DeletePostVolunteer(r.Context(), params)
	if err != nil {
		slog.Error("Failed to delete post volunteer", "error", err, "postID", postID, "volunteerID", volunteerID)
		respondWithError(w, http.StatusInternalServerError, "Failed to delete post volunteer")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Volunteer removed successfully"})
}

// handleApproveVolunteer approves a volunteer for a post.
// @Summary Approve a volunteer for a post
// @Description Allows the owner of a post to approve a pending volunteer application.
// @Tags Posts, Volunteers
// @Accept json
// @Produce json
// @Param approvalData body ApproveRejectVolunteerRequest true "Details of the post and volunteer to approve"
// @Success 200 {object} PostVolunteerDTO "Successfully approved volunteer"
// @Failure 400 {object} ErrorResponse "Invalid request payload or missing IDs"
// @Failure 401 {object} ErrorResponse "Authentication required"
// @Failure 403 {object} ErrorResponse "Forbidden - not authorized to approve volunteers for this post"
// @Failure 404 {object} ErrorResponse "Post or volunteer application not found"
// @Failure 500 {object} ErrorResponse "Failed to approve volunteer"
// @Security BearerAuth
// @Router /approve_volunteer [post]
func (s *Server) handleApproveVolunteer(w http.ResponseWriter, r *http.Request) {
	authUserID, err := getUserIDFromContext(r.Context())
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req ApproveRejectVolunteerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload: "+err.Error())
		return
	}
	defer r.Body.Close()

	if req.PostID == uuid.Nil || req.VolunteerUserID == uuid.Nil {
		respondWithError(w, http.StatusBadRequest, "PostID and VolunteerUserID are required")
		return
	}

	post, err := s.db.GetPost(r.Context(), toPgtypeUUID(req.PostID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Post not found")
			return
		}
		slog.Error("Failed to get post for volunteer approval", "error", err, "postID", req.PostID)
		respondWithError(w, http.StatusInternalServerError, "Could not verify post ownership")
		return
	}

	if post.UserID.Bytes != authUserID.Bytes {
		respondWithError(w, http.StatusForbidden, "You are not authorized to approve volunteers for this post")
		return
	}

	params := db.ApproveVolunteerParams{
		PostID: toPgtypeUUID(req.PostID),
		UserID: toPgtypeUUID(req.VolunteerUserID),
	}

	approvedVolunteer, err := s.db.ApproveVolunteer(r.Context(), params)
	if err != nil {
		slog.Error("Failed to approve volunteer", "error", err, "postID", req.PostID, "volunteerID", req.VolunteerUserID)
		respondWithError(w, http.StatusInternalServerError, "Failed to approve volunteer: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, PostVolunteerDTO{
		ID:        approvedVolunteer.ID.Bytes,
		UserID:    approvedVolunteer.UserID.Bytes,
		PostID:    approvedVolunteer.PostID.Bytes,
		Status:    approvedVolunteer.Status.String,
		Notes:     approvedVolunteer.Notes.String,
		CreatedAt: approvedVolunteer.CreatedAt.Time,
		UpdatedAt: approvedVolunteer.UpdatedAt.Time,
	})
}

// handleRejectVolunteer rejects a volunteer for a post.
// @Summary Reject a volunteer for a post
// @Description Allows the owner of a post to reject a pending volunteer application.
// @Tags Posts, Volunteers
// @Accept json
// @Produce json
// @Param rejectionData body ApproveRejectVolunteerRequest true "Details of the post and volunteer to reject"
// @Success 200 {object} PostVolunteerDTO "Successfully rejected volunteer"
// @Failure 400 {object} ErrorResponse "Invalid request payload or missing IDs"
// @Failure 401 {object} ErrorResponse "Authentication required"
// @Failure 403 {object} ErrorResponse "Forbidden - not authorized to reject volunteers for this post"
// @Failure 404 {object} ErrorResponse "Post or volunteer application not found"
// @Failure 500 {object} ErrorResponse "Failed to reject volunteer"
// @Security BearerAuth
// @Router /reject_volunteer [post]
func (s *Server) handleRejectVolunteer(w http.ResponseWriter, r *http.Request) {
	authUserID, err := getUserIDFromContext(r.Context())
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req ApproveRejectVolunteerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload: "+err.Error())
		return
	}
	defer r.Body.Close()

	if req.PostID == uuid.Nil || req.VolunteerUserID == uuid.Nil {
		respondWithError(w, http.StatusBadRequest, "PostID and VolunteerUserID are required")
		return
	}

	post, err := s.db.GetPost(r.Context(), toPgtypeUUID(req.PostID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Post not found")
			return
		}
		slog.Error("Failed to get post for volunteer rejection", "error", err, "postID", req.PostID)
		respondWithError(w, http.StatusInternalServerError, "Could not verify post ownership")
		return
	}

	if post.UserID.Bytes != authUserID.Bytes {
		respondWithError(w, http.StatusForbidden, "You are not authorized to reject volunteers for this post")
		return
	}

	params := db.RejectVolunteerParams{
		PostID: toPgtypeUUID(req.PostID),
		UserID: toPgtypeUUID(req.VolunteerUserID),
	}

	rejectedVolunteer, err := s.db.RejectVolunteer(r.Context(), params)
	if err != nil {
		slog.Error("Failed to reject volunteer", "error", err, "postID", req.PostID, "volunteerID", req.VolunteerUserID)
		respondWithError(w, http.StatusInternalServerError, "Failed to reject volunteer: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, PostVolunteerDTO{
		ID:        rejectedVolunteer.ID.Bytes,
		UserID:    rejectedVolunteer.UserID.Bytes,
		PostID:    rejectedVolunteer.PostID.Bytes,
		Status:    rejectedVolunteer.Status.String,
		Notes:     rejectedVolunteer.Notes.String,
		CreatedAt: rejectedVolunteer.CreatedAt.Time,
		UpdatedAt: rejectedVolunteer.UpdatedAt.Time,
	})
}

// handleGetUserStats retrieves statistics for a user.
// @Summary Get user statistics
// @Description Retrieves statistics for a specific user, including post count, volunteer count, and approved post count.
// @Tags Users, Stats
// @Produce json
// @Param userId path string true "User ID" format(uuid)
// @Success 200 {object} db.GetUserStatsRow "Successfully retrieved user statistics"
// @Failure 400 {object} ErrorResponse "Invalid user ID format"
// @Failure 401 {object} ErrorResponse "Authentication required"
// @Failure 500 {object} ErrorResponse "Failed to retrieve user stats"
// @Security BearerAuth
// @Router /users/{userId}/stats [get]
func (s *Server) handleGetUserStats(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.PathValue("userId")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid user ID format")
		return
	}

	stats, err := s.db.GetUserStats(r.Context(), toPgtypeUUID(userID))
	if err != nil {
		slog.Error("Failed to get user stats", "error", err, "userID", userID)
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve user stats")
		return
	}
	respondWithJSON(w, http.StatusOK, stats)
}

func toPgtypeFloat8(val float64) pgtype.Float8 {
	return pgtype.Float8{Float64: val, Valid: true}
}

// handleGetCategoryName retrieves name for a category.
// @Summary Get category name
// @Description Retrieves category name.
// @Tags Categories
// @Produce json
// @Param categoryId path string true "Category ID" format(uuid)
// @Success 200 {object} string "Successfully retrieved category name."
// @Failure 400 {object} ErrorResponse "Invalid category ID format"
// @Failure 500 {object} ErrorResponse "Failed to retrieve category name"
// @Router /categories/{categoryId} [get]
func (s *Server) handleGetCategoryName(w http.ResponseWriter, r *http.Request) {
	categoryIdStr := r.PathValue("categoryId")
	categoryId, err := uuid.Parse(categoryIdStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid category ID format")
		return
	}

	name, err := s.db.GetCategoryName(r.Context(), toPgtypeUUID(categoryId))
	if err != nil {
		slog.Error("Failed to get user stats", "error", err, "userID", categoryId)
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve user stats")
		return
	}

	respondWithJSON(w, http.StatusOK, name)
}

// handleGetCategories retrieves categories.
// @Summary Get categories
// @Description Retrieves categories.
// @Tags Categories
// @Produce json
// @Success 200 {object} []CategoryDTO "Successfully retrieved categories"
// @Failure 500 {object} ErrorResponse "Failed to retrieve categories"
// @Router /categories [get]
func (s *Server) handleGetCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := s.db.GetCategories(r.Context())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve user stats")
		return
	}

	res, err := toCategoryDTOFromGetCategories(categories)

	if err != nil {
		slog.Error("Failed to get categories", "error", err)
	}

	respondWithJSON(w, http.StatusOK, res)
}
