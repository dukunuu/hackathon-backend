package server

import (
	"context"
	"net/http"
	"strings"
)

func (s *Server) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			respondWithError(w, http.StatusUnauthorized, "Authorization header required")
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			respondWithError(w, http.StatusUnauthorized, "Invalid Authorization header format")
			return
		}
		tokenString := parts[1]

		claims, err := parseJWT(tokenString, s.jwtSecret)
		if err != nil {
			respondWithError(w, http.StatusUnauthorized, "Invalid token: "+err.Error())
			return
		}

		// Add user information to context
		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		if claims.Role != nil {
			ctx = context.WithValue(ctx, UserRoleKey, claims.Role)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
