package server

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"agentgrid/internal/db"
	"agentgrid/internal/scheduler"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	db    *db.Database
	sched *scheduler.Scheduler
	tmpls *template.Template
	mode  string
}

func New(database *db.Database, sched *scheduler.Scheduler, mode string) *Server {
	s := &Server{
		db:    database,
		sched: sched,
		mode:  mode,
	}
	s.parseTemplates()
	return s
}

func (s *Server) parseTemplates() {
	var err error
	files, _ := filepath.Glob("templates/*.html")
	partialFiles, _ := filepath.Glob("templates/**/*.html")
	allFiles := append(files, partialFiles...)
	if len(allFiles) > 0 {
		s.tmpls, err = template.New("").ParseFiles(allFiles...)
		if err != nil {
			log.Printf("Warning: could not parse templates: %v", err)
		}
	}
}

func (s *Server) render(w http.ResponseWriter, name string, data map[string]interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if data == nil {
		data = make(map[string]interface{})
	}
	data["ContentBlock"] = strings.TrimSuffix(name, ".html") + "_content"
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
	if s.mode == "local" {
		r.Get("/auth/local", s.handleLocalLogin)
	}
	r.Post("/auth/logout", s.handleLogout)

	r.Group(func(r chi.Router) {
		r.Use(s.authMiddleware)

		r.Get("/dashboard", s.handleDashboard)

		r.Get("/projects", s.handleProjects)
		r.Get("/projects/new", s.handleNewProjectPage)
		r.Post("/projects", s.handleCreateProject)
		r.Get("/projects/{id}", s.handleProject)
		r.Get("/projects/{id}/secrets", s.handleProjectSecrets)
		r.Post("/projects/{id}/secrets", s.handleCreateSecret)
		r.Post("/projects/{id}/secrets/delete", s.handleDeleteSecret)
		r.Post("/projects/{id}/delete", s.handleDeleteProject)

		r.Get("/projects/{id}/agents/new", s.handleNewAgentPage)
		r.Post("/projects/{id}/agents", s.handleCreateAgent)

		r.Get("/agents/{id}", s.handleAgent)
		r.Post("/agents/{id}", s.handleUpdateAgent)
		r.Post("/agents/{id}/delete", s.handleDeleteAgent)
		r.Get("/agents/{id}/wake", s.handleAgentWake)
		r.Get("/agents/{id}/docker", s.handleAgentDocker)
		r.Get("/agents/{id}/sessions", s.handleAgentSessions)
		r.Get("/agents/{id}/logs", s.handleAgentLogs)
		r.Get("/agents/{id}/runs/{run_id}", s.handleRunDetail)
		r.Get("/agents/{id}/runs/{run_id}/stream", s.handleRunStream)
		r.Post("/agents/{id}/run", s.handleRunAgentNow)
		r.Post("/agents/{id}/dry-run", s.handleDryRunAgent)
		r.Get("/agents/{id}/files", s.handleAgentFiles)
		r.Get("/agents/{id}/files/edit", s.handleEditFilePage)
		r.Post("/agents/{id}/files/save", s.handleSaveFile)
		r.Post("/agents/{id}/files/upload", s.handleUploadFile)
		r.Get("/agents/{id}/files/download", s.handleDownloadFile)
		r.Post("/agents/{id}/files/delete", s.handleDeleteFile)
		r.Post("/agents/{id}/files/mkdir", s.handleMkdir)
	})

	return r
}
