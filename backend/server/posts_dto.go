// server/handlers_posts_dtos.go
package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/dukunuu/hackathon_backend/db" // ADJUST THIS IMPORT PATH
	"github.com/google/uuid"
)

// CreatePostRequest defines the JSON body for creating a new post.
// swagger:model CreatePostRequest
type CreatePostRequest struct {
	Title         string          `json:"title" example:"Need help cleaning the park"`
	Description   string          `json:"description" example:"The local park needs volunteers for a cleanup drive."`
	Status        string   				`json:"status,omitempty" example:"Хүлээгдэж байгаа"`
	Priority      string				  `json:"priority,omitempty" example:"дунд"`
	PreviewURL    string          `json:"preview_url,omitempty" example:"http://example.com/image.jpg"`
	PostType      string		      `json:"post_type" example:"хандив"`
	MaxVolunteers int32           `json:"max_volunteers,omitempty" example:"10"`
	CategoryID    uuid.UUID       `json:"category_id,omitempty" format:"uuid"`
	LocationLat   float64         `json:"location_lat,omitempty" example:"47.9187"`
	LocationLng   float64         `json:"location_lng,omitempty" example:"106.9170"`
	AddressText   string          `json:"address_text,omitempty" example:"Peace Avenue, Ulaanbaatar"`
}

// UpdatePostRequest defines the JSON body for updating an existing post.
// swagger:model UpdatePostRequest
type UpdatePostRequest struct {
	Title             string          `json:"title" example:"Urgent: Park Cleanup Drive"`
	Description       string          `json:"description" example:"Updated details: The local park needs volunteers urgently."`
	Status            string		      `json:"status" example:"Шийдвэрлэгдэж байгаа"`
	Priority          string   			  `json:"priority" example:"өндөр"`
	PreviewURL        string          `json:"preview_url,omitempty" example:"http://example.com/new_image.jpg"`
	PostType          string		      `json:"post_type" example:"хандив"`
	MaxVolunteers     int32           `json:"max_volunteers" example:"15"`
	CurrentVolunteers int32           `json:"current_volunteers" example:"5"`
	CategoryID        uuid.UUID       `json:"category_id,omitempty" format:"uuid"`
	LocationLat       float64         `json:"location_lat,omitempty" example:"47.9200"`
	LocationLng       float64         `json:"location_lng,omitempty" example:"106.9250"`
	AddressText       string          `json:"address_text,omitempty" example:"Sukhbaatar Square, Ulaanbaatar"`
}

// swagger:model CategoryDTO
type CategoryDTO struct{
	ID        uuid.UUID   `json:"id" format:"uuid"`
	Name    	string		  `json:"name"`
	Description    	string		  `json:"description"`
	Endpoint    string   	`json:"endpoint"`
	CanVolunteer bool			`json:"can_volunteer"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// PostVolunteerDTO represents a volunteer for a post in responses.
// swagger:model PostVolunteerDTO
type PostVolunteerDTO struct {
	ID        uuid.UUID   `json:"id" format:"uuid"`
	UserID    uuid.UUID   `json:"user_id" format:"uuid"`
	PostID    uuid.UUID   `json:"post_id" format:"uuid"`
	Status    string      `json:"status" example:"pending"`
	Notes     string		  `json:"notes"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

func toCategoryDTO(cat db.Category) (CategoryDTO, error) {
	categoryId, err := uuid.Parse(cat.ID.String())
	if err != nil {
		slog.Error("Failed to parse category ID")
		return CategoryDTO{}, fmt.Errorf("Failed to parse category ID")
	}
	return CategoryDTO{
		ID: categoryId,
		Name: cat.Name,
		Description: cat.Description.String,
		Endpoint: cat.Endpoint,
		CanVolunteer: cat.CanVolunteer.Bool,
		CreatedAt: cat.CreatedAt.Time,
		UpdatedAt: cat.UpdatedAt.Time,
	}, nil	
}

func toCategoryDTOFromGetCategories(cat []db.Category) ([]CategoryDTO, error) {
	var res []CategoryDTO
	for _, val := range cat {
		val, err := toCategoryDTO(val)
		if err != nil {
			slog.Error("Failed to parse category")
			return nil, fmt.Errorf("Failed to parse category: %s", val.Name)
		}
		res = append(res, val)	
	}

	return res, nil
}

// PostResponseDTO is used for GET /posts and GET /posts/{postId}
// swagger:model PostResponseDTO
type PostResponseDTO struct {
	ID                uuid.UUID          `json:"id" format:"uuid"`
	Title             string             `json:"title"`
	Description       string             `json:"description"`
	Status            string      `json:"status"`
	Priority          string    `json:"priority"`
	PreviewURL        string             `json:"preview_url,omitempty"`
	PostType          string        `json:"post_type"`
	UserID            uuid.UUID          `json:"user_id" format:"uuid"`
	MaxVolunteers     int32              `json:"max_volunteers"`
	CurrentVolunteers int32              `json:"current_volunteers"`
	CategoryID        uuid.UUID        `json:"category_id,omitempty"`
	LocationLat       float32      `json:"location_lat,omitempty"`
	LocationLng       float32      `json:"location_lng,omitempty"`
	AddressText       string        `json:"address_text,omitempty"`
	CreatedAt         time.Time          `json:"created_at"`
	UpdatedAt         time.Time          `json:"updated_at"`
	Images            []string           `json:"images"`
	Volunteers        []PostVolunteerDTO `json:"volunteers"`
}

// ApproveRejectVolunteerRequest defines the JSON body for approving or rejecting a volunteer.
// swagger:model ApproveRejectVolunteerRequest
type ApproveRejectVolunteerRequest struct {
	PostID          uuid.UUID `json:"post_id" format:"uuid" example:"a1b2c3d4-e5f6-7777-8888-99990000aaaa"`
	VolunteerUserID uuid.UUID `json:"volunteer_user_id" format:"uuid" example:"a1b2c3d4-e5f6-7777-8888-99990000bbbb"`
}

func toPostResponseDTO(p db.Post) PostResponseDTO {
	categoryId,err := uuid.Parse(p.CategoryID.String())
	if err != nil {
		slog.Default().Error("")
		return PostResponseDTO{}
	}
	return PostResponseDTO{
		ID:                p.ID.Bytes,
		Title:             p.Title,
		Description:       p.Description,
		Status:            string(p.Status),
		Priority:          string(p.Priority),
		PreviewURL:        p.PreviewUrl.String,
		PostType:          string(p.PostType),
		UserID:            p.UserID.Bytes,
		MaxVolunteers:     p.MaxVolunteers,
		CurrentVolunteers: p.CurrentVolunteers,
		CategoryID:        categoryId,
		LocationLat:       float32(p.LocationLat.Float64),
		LocationLng:       float32(p.LocationLng.Float64),
		AddressText:       p.AddressText.String,
		CreatedAt:         p.CreatedAt.Time,
		UpdatedAt:         p.UpdatedAt.Time,
		Images:            []string{},
		Volunteers:        []PostVolunteerDTO{},
	}
}

func toPostResponseDTOFromGetPostRow(row db.GetPostRow) (PostResponseDTO, error) {
	var images []string
	if row.Images != nil && len(row.Images) > 0 {
		if err := json.Unmarshal(row.Images, &images); err != nil {
			return PostResponseDTO{}, fmt.Errorf("failed to unmarshal images: %w", err)
		}
	}

	categoryId,err := uuid.Parse(row.CategoryID.String())
	if err != nil {
		slog.Default().Error("")
		return PostResponseDTO{}, err
	}

	var volunteers []PostVolunteerDTO
	if row.Volunteers != nil && len(row.Volunteers) > 0 {
		var dbVolunteers []db.PostVolunteer
		if err := json.Unmarshal(row.Volunteers, &dbVolunteers); err != nil {
			return PostResponseDTO{}, fmt.Errorf("failed to unmarshal volunteers: %w", err)
		}
		for _, v := range dbVolunteers {
			volunteers = append(volunteers, PostVolunteerDTO{
				ID:        v.ID.Bytes,
				UserID:    v.UserID.Bytes,
				PostID:    v.PostID.Bytes,
				Status:    v.Status.String,
				Notes:     v.Notes.String,
				CreatedAt: v.CreatedAt.Time,
				UpdatedAt: v.UpdatedAt.Time,
			})
		}
	}

	return PostResponseDTO{
		ID:                row.ID.Bytes,
		Title:             row.Title,
		Description:       row.Description,
		Status:            string(row.Status),
		Priority:          string(row.Priority),
		PreviewURL:        row.PreviewUrl.String,
		PostType:          string(row.PostType),
		UserID:            row.UserID.Bytes,
		MaxVolunteers:     row.MaxVolunteers,
		CurrentVolunteers: row.CurrentVolunteers,
		CategoryID:        categoryId,
		LocationLat:       float32(row.LocationLat.Float64),
		LocationLng:       float32(row.LocationLng.Float64),
		AddressText:       row.AddressText.String,
		CreatedAt:         row.CreatedAt.Time,
		UpdatedAt:         row.UpdatedAt.Time,
		Images:            images,
		Volunteers:        volunteers,
	}, nil
}

func toPostResponseDTOFromListPostsRow(row db.ListPostsRow) (PostResponseDTO, error) {
	var images []string
	if row.Images != nil && len(row.Images) > 0 {
		if err := json.Unmarshal(row.Images, &images); err != nil {
			return PostResponseDTO{}, fmt.Errorf("failed to unmarshal images: %w", err)
		}
	}

	categoryId,err := uuid.Parse(row.CategoryID.String())
	if err != nil && len(categoryId) == 0 {
		slog.Default().Error("")
		return PostResponseDTO{}, err
	}

	var volunteers []PostVolunteerDTO
	if row.Volunteers != nil && len(row.Volunteers) > 0 {
		var dbVolunteers []db.PostVolunteer
		if err := json.Unmarshal(row.Volunteers, &dbVolunteers); err != nil {
			return PostResponseDTO{}, fmt.Errorf("failed to unmarshal volunteers: %w", err)
		}
		for _, v := range dbVolunteers {
			volunteers = append(volunteers, PostVolunteerDTO{
				ID:        v.ID.Bytes,
				UserID:    v.UserID.Bytes,
				PostID:    v.PostID.Bytes,
				Status:    v.Status.String,
				Notes:     v.Notes.String,
				CreatedAt: v.CreatedAt.Time,
				UpdatedAt: v.UpdatedAt.Time,
			})
		}
	}

	return PostResponseDTO{
		ID:                row.ID.Bytes,
		Title:             row.Title,
		Description:       row.Description,
		Status:            string(row.Status),
		Priority:          string(row.Priority),
		PreviewURL:        row.PreviewUrl.String,
		PostType:          string(row.PostType),
		UserID:            row.UserID.Bytes,
		MaxVolunteers:     row.MaxVolunteers,
		CurrentVolunteers: row.CurrentVolunteers,
		CategoryID:        categoryId,
		LocationLat:       float32(row.LocationLat.Float64),
		LocationLng:       float32(row.LocationLng.Float64),
		AddressText:       row.AddressText.String,
		CreatedAt:         row.CreatedAt.Time,
		UpdatedAt:         row.UpdatedAt.Time,
		Images:            images,
		Volunteers:        volunteers,
	}, nil
}
