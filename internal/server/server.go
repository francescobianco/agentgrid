package server

import (
	"html/template"
	"log"
	"net/http"

	"agentgrid/internal/db"
	"agentgrid/internal/scheduler"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	db    *db.Database
	sched *scheduler.Scheduler
	tmpls *template.Template
}

func New(database *db.Database, sched *scheduler.Scheduler) *Server {
	s := &Server{
		db:    database,
		sched: sched,
	}
	s.parseTemplates()
	return s
}

func (s *Server) parseTemplates() {
	var err error
	s.tmpls, err = template.ParseGlob("templates/*.html")
	if err != nil {
		log.Printf("Warning: could not parse templates: %v", err)
	}
}

func (s *Server) render(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpls.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)

	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	r.Get("/", s.handleHome)
	r.Get("/login", s.handleLoginPage)

	r.Get("/auth/github", s.handleGitHubLogin)
	r.Get("/auth/github/callback", s.handleGitHubCallback)
	r.Post("/auth/logout", s.handleLogout)

	r.Group(func(r chi.Router) {
		r.Use(s.authMiddleware)

		r.Get("/dashboard", s.handleDashboard)

		r.Get("/projects", s.handleProjects)
		r.Post("/projects", s.handleCreateProject)
		r.Get("/projects/{id}", s.handleProject)
		r.Post("/projects/{id}/delete", s.handleDeleteProject)

		r.Post("/projects/{id}/agents", s.handleCreateAgent)

		r.Get("/agents/{id}", s.handleAgent)
		r.Post("/agents/{id}", s.handleUpdateAgent)
		r.Post("/agents/{id}/delete", s.handleDeleteAgent)
		r.Get("/agents/{id}/runs", s.handleAgentRuns)
	})

	return r
}
