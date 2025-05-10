package server

import (
	"time"

	"github.com/dukunuu/hackathon_backend/db" // ADJUST THIS IMPORT PATH
	"github.com/google/uuid"
)

// UserResponseDTO is the data transfer object for user responses.
// swagger:model UserResponse
type UserResponseDTO struct {
	ID             uuid.UUID `json:"id" example:"a1b2c3d4-e5f6-7777-8888-99990000abcd"`
	FirstName      string    `json:"first_name" example:"John"`
	LastName       string    `json:"last_name" example:"Doe"`
	Phone          *string   `json:"phone,omitempty" example:"99119911"`
	IsVolunteering bool      `json:"is_volunteering" example:"false"`
	Email          string    `json:"email" example:"john.doe@example.com"`
	Role           string    `json:"role" example:"USER"` // Changed from interface{} to string
	ProfileUrl     *string   `json:"profile_url,omitempty" example:"http://example.com/profile.jpg"`
	CreatedAt      time.Time `json:"created_at" example:"2023-01-01T12:00:00Z"`
	UpdatedAt      time.Time `json:"updated_at" example:"2023-01-01T13:00:00Z"`
}

// ToUserResponseDTO converts a db.User to a UserResponseDTO.
func ToUserResponseDTO(user db.User) UserResponseDTO {
	var phone *string
	if user.Phone.Valid {
		phone = &user.Phone.String
	}

	var profileURL *string
	if user.ProfileUrl.Valid {
		profileURL = &user.ProfileUrl.String
	}

	var userID uuid.UUID
	if user.ID.Valid {
		userID = uuid.UUID(user.ID.Bytes)
	}

	var createdAt time.Time
	if user.CreatedAt.Valid {
		createdAt = user.CreatedAt.Time
	}

	var updatedAt time.Time
	if user.UpdatedAt.Valid {
		updatedAt = user.UpdatedAt.Time
	}

	// Handle Role conversion from interface{} to string
	var roleStr = string(user.Role)

	return UserResponseDTO{
		ID:             userID,
		FirstName:      user.FirstName,
		LastName:       user.LastName,
		Phone:          phone,
		IsVolunteering: user.IsVolunteering,
		Email:          user.Email,
		Role:           roleStr, // Assign the converted string
		ProfileUrl:     profileURL,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}
}

// ToUserResponseDTOs converts a slice of db.User to a slice of UserResponseDTO.
func ToUserResponseDTOs(users []db.User) []UserResponseDTO {
	dtos := make([]UserResponseDTO, len(users))
	for i, u := range users {
		dtos[i] = ToUserResponseDTO(u)
	}
	return dtos
}

// LoginResponsePayloadDTO defines the JSON response for successful login using DTO.
// swagger:model LoginResponse
type LoginResponsePayloadDTO struct {
	Token string          `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	User  UserResponseDTO `json:"user"`
}

// ToLoginResponsePayloadDTO creates a LoginResponsePayloadDTO.
func ToLoginResponsePayloadDTO(token string, user db.User) LoginResponsePayloadDTO {
	return LoginResponsePayloadDTO{
		Token: token,
		User:  ToUserResponseDTO(user),
	}
}

