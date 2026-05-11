package server

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("session"); err == nil {
		if _, err := s.db.GetSession(cookie.Value); err == nil {
			http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
			return
		}
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("session"); err == nil {
		if _, err := s.db.GetSession(cookie.Value); err == nil {
			http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
			return
		}
	}
	s.render(w, "login.html", nil)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	projects, _ := s.db.ListProjects(user.ID)
	s.render(w, "dashboard.html", map[string]interface{}{
		"User":     user,
		"Projects": projects,
	})
}

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	projects, _ := s.db.ListProjects(user.ID)
	s.render(w, "projects.html", map[string]interface{}{
		"User":     user,
		"Projects": projects,
	})
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	_, err := s.db.CreateProject(user.ID, name, r.FormValue("description"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/projects", http.StatusSeeOther)
}

func (s *Server) handleProject(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	project, err := s.db.GetProject(id)
	if err != nil || project.UserID != user.ID {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}
	agents, _ := s.db.ListAgents(project.ID)
	s.render(w, "project.html", map[string]interface{}{
		"User":    user,
		"Project": project,
		"Agents":  agents,
	})
}

func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	project, err := s.db.GetProject(id)
	if err != nil || project.UserID != user.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	agents, _ := s.db.ListAgents(project.ID)
	for _, a := range agents {
		s.sched.RemoveAgent(a.ID)
	}
	s.db.DeleteProject(id)
	http.Redirect(w, r, "/projects", http.StatusSeeOther)
}

func (s *Server) handleCreateAgent(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	project, err := s.db.GetProject(projectID)
	if err != nil || project.UserID != user.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	name := r.FormValue("name")
	prompt := r.FormValue("prompt")
	cronExpr := r.FormValue("cron_expression")
	if name == "" || prompt == "" || cronExpr == "" {
		http.Error(w, "Name, prompt, and cron expression are required", http.StatusBadRequest)
		return
	}

	workingDir := r.FormValue("working_directory")
	if workingDir == "" {
		workingDir = "/work"
	}
	dockerImage := r.FormValue("docker_image")
	if dockerImage == "" {
		dockerImage = "alpine:latest"
	}

	agent, err := s.db.CreateAgent(projectID, name, prompt, cronExpr, workingDir, dockerImage)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.sched.AddAgent(agent.ID, cronExpr)
	http.Redirect(w, r, "/projects/"+strconv.FormatInt(projectID, 10), http.StatusSeeOther)
}

func (s *Server) handleAgent(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	agent, err := s.db.GetAgent(id)
	if err != nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	project, err := s.db.GetProject(agent.ProjectID)
	if err != nil || project.UserID != user.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	runs, _ := s.db.ListAgentRuns(agent.ID)
	s.render(w, "agent.html", map[string]interface{}{
		"User":    user,
		"Agent":   agent,
		"Project": project,
		"Runs":    runs,
	})
}

func (s *Server) handleUpdateAgent(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	agent, err := s.db.GetAgent(id)
	if err != nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	project, err := s.db.GetProject(agent.ProjectID)
	if err != nil || project.UserID != user.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	name := r.FormValue("name")
	prompt := r.FormValue("prompt")
	cronExpr := r.FormValue("cron_expression")
	if name == "" || prompt == "" || cronExpr == "" {
		http.Error(w, "Name, prompt, and cron expression are required", http.StatusBadRequest)
		return
	}

	isActive := r.FormValue("is_active") == "on"
	s.db.UpdateAgent(id, name, prompt, cronExpr, isActive)

	if isActive {
		s.sched.AddAgent(id, cronExpr)
	} else {
		s.sched.RemoveAgent(id)
	}

	http.Redirect(w, r, "/agents/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func (s *Server) handleDeleteAgent(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	agent, err := s.db.GetAgent(id)
	if err != nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	project, err := s.db.GetProject(agent.ProjectID)
	if err != nil || project.UserID != user.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	s.sched.RemoveAgent(id)
	s.db.DeleteAgent(id)
	http.Redirect(w, r, "/projects/"+strconv.FormatInt(agent.ProjectID, 10), http.StatusSeeOther)
}

func (s *Server) handleAgentRuns(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	agent, err := s.db.GetAgent(id)
	if err != nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	project, err := s.db.GetProject(agent.ProjectID)
	if err != nil || project.UserID != user.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	runs, _ := s.db.ListAgentRuns(agent.ID)
	s.render(w, "agent_runs.html", map[string]interface{}{
		"User":  user,
		"Agent": agent,
		"Runs":  runs,
	})
}
