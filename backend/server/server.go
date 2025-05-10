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

	r.Get("/api/v1/posts", s.handleListPosts)
	r.Get("/api/v1/category/{categoryId}", s.handleGetCategoryName)
	r.Get("/api/v1/categories", s.handleGetCategories)

r.Group(func(rauth chi.Router) {
		rauth.Use(s.AuthMiddleware)
		rauth.Get("/api/v1/users/me", s.handleGetCurrentUser)
		rauth.Put("/api/v1/users/me/details", s.handleUpdateUserDetails)
		rauth.Put("/api/v1/users/me/email", s.handleUpdateUserEmail)
		rauth.Put("/api/v1/users/me/password", s.handleUpdateUserPassword)

		rauth.Get("/api/v1/users", s.handleListUsers)
		rauth.Get("/api/v1/users/{userID}", s.handleGetUserByID)
		rauth.Delete("/api/v1/users/{userID}", s.handleDeleteUser)
		rauth.Get("/api/v1/users/by-email", s.handleGetUserByEmail)

		rauth.Get("/api/v1/users/{userId}/posts", s.handleGetUserPosts)
		rauth.Post("/api/v1/posts", s.handleCreatePost)

		rauth.Get("/api/v1/posts/{postId}", s.handleGetPost)
		rauth.Put("/api/v1/posts/{postId}", s.handleUpdatePost)
		rauth.Delete("/api/v1/posts/{postId}", s.handleDeletePost)

		rauth.Get("/api/v1/posts/volunteers", s.handleListPostVolunteers)

		rauth.Delete("/api/v1/posts/volunteers/{userId}", s.handleDeletePostVolunteer)

		rauth.Post("/api/v1/approve_volunteer", s.handleApproveVolunteer)
		rauth.Post("/api/v1/reject_volunteer", s.handleRejectVolunteer)

		rauth.Get("/api/v1/users/{userId}/stats", s.handleGetUserStats)
	})
	slog.Info("Server starting", "address", s.addr)
	if err := http.ListenAndServe(s.addr, r); err != nil {
		slog.Error("Failed to start server", "error", err)
		log.Fatalf("Failed to start server: %v", err)
	}
}
