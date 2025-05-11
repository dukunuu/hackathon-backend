// server/handlers_posts.go
package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/dukunuu/hackathon_backend/db" // ADJUST THIS IMPORT PATH
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func toPgtypeUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: id != uuid.Nil}
}

const (
	maxPostImages        = 5
	maxPostImageFileSize = 5 * 1024 * 1024 // 5MB per image
	maxPostRequestSize = (maxPostImages * maxPostImageFileSize) + (1 * 1024 * 1024) // ~26MB
)


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

// handleCreatePost creates a new post, optionally with images and AI categorization.
// @Summary Create a new post (with optional images and AI categorization)
// @Description Creates a new post. The authenticated user will be the owner.
// @Description Post details are sent as a JSON string in the 'postData' form field.
// @Description Optionally, up to 5 images can be uploaded via the 'postImages' form field.
// @Description The post description will be used to automatically categorize the post using AI.
// @Tags Posts
// @Accept multipart/form-data
// @Produce json
// @Param postData formData string true "Post creation details as a JSON string. Example: '{\"title\":\"New Event\", \"description\":\"Event details here for AI categorization\", \"post_type\":\"event\"}'"
// @Param postImages formData file false "Optional post image files (max 5MB each, up to 5 files, types: jpeg, png, gif, webp)"
// @Success 201 {object} PostResponseDTO "Successfully created post"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 401 {object} ErrorResponse "Authentication required"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /posts [post] // Assuming this is defined in your main router setup
func (s *Server) handleCreatePost(w http.ResponseWriter, r *http.Request) {
	authUserID, err := getUserIDFromContext(r.Context())
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	if err := r.ParseMultipartForm(maxPostRequestSize); err != nil {
		slog.Error("Failed to parse multipart form for post creation", "error", err)
		respondWithError(
			w,
			http.StatusBadRequest,
			"Could not parse form: "+err.Error(),
		)
		return
	}

	postDataJSON := r.FormValue("postData")
	if postDataJSON == "" {
		respondWithError(w, http.StatusBadRequest, "Missing 'postData' form field.")
		return
	}

	var req CreatePostRequest
	if err := json.Unmarshal([]byte(postDataJSON), &req); err != nil {
		respondWithError(
			w,
			http.StatusBadRequest,
			"Invalid JSON in 'postData' field: "+err.Error(),
		)
		return
	}

	if req.Title == "" || req.Description == "" || req.PostType == "" {
		respondWithError(
			w,
			http.StatusBadRequest,
			"Missing required fields in 'postData': title, description, post_type",
		)
		return
	}

	// --- AI Categorization ---
	var chosenCategoryID pgtype.UUID // Assuming CategoryID in DB is UUID and nullable

	if s.aiModel != nil && req.Description != "" {
		slog.Info("Attempting AI categorization for new post", "user_id", authUserID)
		dbCategories, dbErr := s.db.GetCategories(r.Context()) // Ensure this method exists
		if dbErr != nil {
			slog.Error(
				"Failed to fetch categories for AI",
				"error",
				dbErr,
				"user_id",
				authUserID,
			)
			// Decide: continue without category, or error out?
			// For now, log and continue. Post will be created without AI category.
		} else if len(dbCategories) == 0 {
			slog.Warn(
				"No categories found in DB for AI categorization.",
				"user_id",
				authUserID,
			)
		} else {
			var categoryNames []string
			categoryMapByName := make(map[string]uuid.UUID) // Map lowercase name to ID
			promptCategoryList := "Available category names:\n"
			for _, cat := range dbCategories {
				// Assuming db.Category has Name (string) and ID (uuid.UUID)
				categoryNames = append(categoryNames, cat.Name)
				catId, err := uuid.Parse(cat.ID.String())
				if err != nil {
					respondWithError(w, http.StatusInternalServerError, "Failed to parse category id")
				}

				categoryMapByName[strings.ToLower(cat.Name)] = catId
				promptCategoryList += fmt.Sprintf("- %s\n", cat.Name)
			}

			// Construct a clear prompt for the AI
			// The system prompt is already part of s.aiModel.systemPrompt
			aiUserPrompt := fmt.Sprintf(
				"Analyze the following post description:\n\"%s\"\n\n%s\nBased on the description and the list of available category names, please identify the single most appropriate category. Respond with ONLY the name of the chosen category from the list. For example, if the best category is '%s', your response should be exactly '%s'.",
				req.Description,
				promptCategoryList,
				categoryNames[0], // Use an actual category name as an example
				categoryNames[0],
			)

			slog.Info(
				"Sending prompt to AI for categorization",
				"user_id",
				authUserID,
				"prompt_length",
				len(aiUserPrompt),
			)
			aiGeneratedCategoryName, aiErr := s.aiModel.GenerateResponse(
				aiUserPrompt,
			)
			if aiErr != nil {
				slog.Error(
					"AI categorization request failed",
					"error",
					aiErr,
					"user_id",
					authUserID,
				)
			} else {
				trimmedName := strings.TrimSpace(aiGeneratedCategoryName)
				slog.Info(
					"AI returned category name",
					"name",
					trimmedName,
					"user_id",
					authUserID,
				)

				// Find the ID for the AI-returned name (case-insensitive)
				catID, found := categoryMapByName[strings.ToLower(trimmedName)]
				if found {
					chosenCategoryID = pgtype.UUID{Bytes: catID, Valid: true}
					slog.Info(
						"Successfully matched AI category name to ID",
						"name",
						trimmedName,
						"id",
						catID.String(),
						"user_id",
						authUserID,
					)
				} else {
					slog.Warn(
						"AI returned category name not found in DB list or was ambiguous",
						"ai_name",
						trimmedName,
						"user_id",
						authUserID,
					)
				}
			}
		}
	} else {
		if s.aiModel == nil {
			slog.Warn(
				"AI model not configured, skipping categorization.",
				"user_id",
				authUserID,
			)
		}
		if req.Description == "" {
			slog.Warn(
				"Post description is empty, skipping AI categorization.",
				"user_id",
				authUserID,
			)
		}
	}
	// --- End AI Categorization ---

	uploadedImageURLs := []string{}
	formFiles := r.MultipartForm.File["postImages"]

	if len(formFiles) > maxPostImages {
		respondWithError(
			w,
			http.StatusBadRequest,
			fmt.Sprintf("Too many images. Maximum %d images allowed.", maxPostImages),
		)
		return
	}

	for _, handler := range formFiles {
		file, err := handler.Open()
		if err != nil {
			slog.Error(
				"Failed to open an uploaded post image file",
				"filename",
				handler.Filename,
				"error",
				err,
			)
			respondWithError(
				w,
				http.StatusInternalServerError,
				"Error processing uploaded file: "+handler.Filename,
			)
			return
		}

		if handler.Size > maxPostImageFileSize {
			file.Close()
			respondWithError(
				w,
				http.StatusBadRequest,
				fmt.Sprintf(
					"Post image '%s' too large. Maximum size is %dMB.",
					handler.Filename,
					maxPostImageFileSize/(1024*1024),
				),
			)
			return
		}

		buffer := make([]byte, 512)
		_, readErr := file.Read(buffer)
		if readErr != nil && readErr != io.EOF {
			file.Close()
			slog.Error(
				"Failed to read post image for MIME type detection",
				"filename",
				handler.Filename,
				"error",
				readErr,
			)
			respondWithError(
				w,
				http.StatusInternalServerError,
				"Could not read image for type detection: "+handler.Filename,
			)
			return
		}
		_, seekErr := file.Seek(0, io.SeekStart)
		if seekErr != nil {
			file.Close()
			slog.Error(
				"Failed to seek post image after MIME type detection",
				"filename",
				handler.Filename,
				"error",
				seekErr,
			)
			respondWithError(
				w,
				http.StatusInternalServerError,
				"Could not process image: "+handler.Filename,
			)
			return
		}

		contentType := http.DetectContentType(buffer)
		if !allowedMimeTypes[contentType] {
			file.Close()
			slog.Warn(
				"Invalid post image type uploaded",
				"contentType",
				contentType,
				"filename",
				handler.Filename,
			)
			respondWithError(
				w,
				http.StatusBadRequest,
				fmt.Sprintf(
					"Invalid image type for '%s': %s. Allowed types: jpeg, png, gif, webp.",
					handler.Filename,
					contentType,
				),
			)
			return
		}

		fileBytes, readAllErr := io.ReadAll(file)
		file.Close()
		if readAllErr != nil {
			slog.Error(
				"Failed to read post image into buffer",
				"filename",
				handler.Filename,
				"error",
				readAllErr,
			)
			respondWithError(
				w,
				http.StatusInternalServerError,
				"Could not read image content: "+handler.Filename,
			)
			return
		}

		tempEntityIDForPath := uuid.New()
		uploadedURL, _, uploadErr := s.filestore.UploadProfileImage(
			r.Context(),
			fileBytes,
			handler.Filename,
			contentType,
			tempEntityIDForPath,
		) // Assuming UploadProfileImage can be used or adapt to UploadPostImage
		if uploadErr != nil {
			slog.Error(
				"Failed to upload post image via filestore",
				"filename",
				handler.Filename,
				"error",
				uploadErr,
			)
			respondWithError(
				w,
				http.StatusInternalServerError,
				"Could not upload image '"+handler.Filename+"': "+uploadErr.Error(),
			)
			return
		}
		uploadedImageURLs = append(uploadedImageURLs, uploadedURL)
		slog.Info(
			"Successfully uploaded post image",
			"filename",
			handler.Filename,
			"url",
			uploadedURL,
		)
	}

	params := db.CreatePostParams{
		Title:             req.Title,
		Description:       req.Description,
		PostType:          db.PostType(req.PostType), // Ensure db.PostType is the correct type
		UserID:            authUserID,
		MaxVolunteers:     req.MaxVolunteers,
		CurrentVolunteers: 0,
		LocationLat:       toPgtypeFloat8(req.LocationLat),
		LocationLng:       toPgtypeFloat8(req.LocationLng),
		AddressText:       toPgtypeText(req.AddressText),
		CategoryID:        chosenCategoryID, // Set the AI-determined category ID
	}

	if req.Status == "" {
		params.Status = db.PostStatus("Хүлээгдэж байгаа") // Default status
	} else {
		params.Status = db.PostStatus(req.Status) // Ensure db.PostStatus is correct type
	}

	if req.Priority == "" {
		params.Priority = db.PostPriority("бага") // Default priority
	} else {
		params.Priority = db.PostPriority(req.Priority) // Ensure db.PostPriority is correct
	}

	if req.PreviewURL != "" {
		params.PreviewUrl = toPgtypeText(req.PreviewURL)
	} else if len(uploadedImageURLs) > 0 {
		params.PreviewUrl = toPgtypeText(uploadedImageURLs[0])
	}

	createdPost, err := s.db.CreatePost(r.Context(), params)
	if err != nil {
		slog.Error(
			"Failed to create post in DB",
			"error",
			err,
			"userID",
			authUserID.String(),
		)
		respondWithError(w, http.StatusInternalServerError, "Failed to create post: "+err.Error())
		return
	}

	finalLinkedImageURLs := []string{} // This will be part of the response DTO
	for _, imgURL := range uploadedImageURLs {
		_, dbErr := s.db.CreatePostImage(r.Context(), db.CreatePostImageParams{
			PostID:   createdPost.ID,
			ImageUrl: imgURL,
		})
		if dbErr != nil {
			slog.Error(
				"Failed to create post_image record in DB",
				"postID",
				createdPost.ID,
				"imageURL",
				imgURL,
				"error",
				dbErr,
			)
			// For simplicity, we log and continue; the image won't be linked.
			// For stricter handling, you might want to roll back or delete the post.
			// Or return an error for the whole request.
			respondWithError(
				w,
				http.StatusInternalServerError,
				"Failed to link image '"+imgURL+"' to post: "+dbErr.Error(),
			)
			return
		}
		finalLinkedImageURLs = append(finalLinkedImageURLs, imgURL)
	}

	responseDTO := toPostResponseDTO(createdPost) // Ensure this DTO can include image URLs
	// You might want to add finalLinkedImageURLs to responseDTO if it's designed for it
	// e.g., responseDTO.ImageURLs = finalLinkedImageURLs

	respondWithJSON(w, http.StatusCreated, responseDTO)
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
