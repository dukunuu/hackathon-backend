package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const UserIDKey contextKey = "userID"
const UserRoleKey contextKey = "userRole" // If you store role in JWT

type ErrorResponse struct {
	Error string `json:"error"`
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, ErrorResponse{Error: message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload any) {
	response, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "Failed to marshal JSON response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func verifyPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

type Claims struct {
	UserID pgtype.UUID `json:"user_id"`
	Email  string      `json:"email"`
	Role   any				 `json:"role,omitempty"` // Store role if needed
	jwt.RegisteredClaims
}

func generateJWT(userID pgtype.UUID, email string, role any, secret string, expiresAt time.Time) (string, error) {
	claims := &Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func parseJWT(tokenString string, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}

func toPgtypeText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

func parseUUIDFromParam(r *http.Request, paramName string) (pgtype.UUID, error) {
	idStr := chi.URLParam(r, paramName)
	if idStr == "" {
		return pgtype.UUID{}, fmt.Errorf("URL parameter '%s' is missing", paramName)
	}
	parsedUUID, err := uuid.Parse(idStr)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("invalid UUID format for parameter '%s': %w", paramName, err)
	}
	return pgtype.UUID{Bytes: parsedUUID, Valid: true}, nil
}

func getUserIDFromContext(ctx context.Context) (pgtype.UUID, error) {
	userIDVal := ctx.Value(UserIDKey)
	if userIDVal == nil {
		return pgtype.UUID{}, errors.New("user ID not found in context")
	}
	userID, ok := userIDVal.(pgtype.UUID)
	if !ok {
		return pgtype.UUID{}, errors.New("user ID in context is of invalid type")
	}
	if !userID.Valid {
		return pgtype.UUID{}, errors.New("user ID in context is invalid (not set)")
	}
	return userID, nil
}

