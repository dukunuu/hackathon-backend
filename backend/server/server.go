package server

import (
	"log"
	"log/slog"
	"net/http"

	"github.com/dukunuu/hackathon_backend/db"
	"github.com/dukunuu/hackathon_backend/file"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

type Server struct {
	db        *db.Queries
	filestore file.FileStore
	addr      string
	jwtSecret string
}

func Init(addr, jwtSecret string, database *db.Queries, filestore file.FileStore) *Server {
	return &Server{
		db:        database,
		addr:      addr,
		jwtSecret: jwtSecret,
		filestore: filestore,
	}
}
func (s *Server) Start() {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger) // Chi's built-in logger
	r.Use(middleware.Recoverer)

	r.Get("/docs/*", httpSwagger.WrapHandler)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		respondWithJSON(w, http.StatusOK, map[string]string{"message": "Welcome to the API"})
	})
	r.Post("/api/v1/users/register", s.handleCreateUser)
	r.Post("/api/v1/users/login", s.handleLogin)

	r.Group(func(rauth chi.Router) {
		rauth.Use(s.AuthMiddleware)

		rauth.Get("/api/v1/users/me", s.handleGetCurrentUser)
		rauth.Put("/api/v1/users/me/details", s.handleUpdateUserDetails) // User updates their own details
		rauth.Put("/api/v1/users/me/email", s.handleUpdateUserEmail)
		rauth.Put("/api/v1/users/me/password", s.handleUpdateUserPassword)
		// rauth.Delete("/api/v1/users/me", s.handleDeleteSelf) // If you want a specific route for self-delete

		// Routes that might require specific roles (e.g., admin) or operate on other users by ID
		rauth.Get("/api/v1/users", s.handleListUsers) // Potentially admin only
		rauth.Get("/api/v1/users/{userID}", s.handleGetUserByID)
		rauth.Delete("/api/v1/users/{userID}", s.handleDeleteUser) // User can delete self, or admin can delete others

		// Example: Get user by email (might be admin only)
		rauth.Get("/api/v1/users/by-email", s.handleGetUserByEmail) // e.g., /api/v1/users/by-email?email=test@example.com
	})

	slog.Info("Server starting", "address", s.addr)
	if err := http.ListenAndServe(s.addr, r); err != nil {
		slog.Error("Failed to start server", "error", err)
		log.Fatalf("Failed to start server: %v", err)
	}
}
