package server

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"agentgrid/internal/scheduler"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if s.mode == "local" {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}
	if cookie, err := r.Cookie("session"); err == nil {
		if _, err := s.db.GetSession(cookie.Value); err == nil {
			http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
			return
		}
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if s.mode == "local" {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}
	if cookie, err := r.Cookie("session"); err == nil {
		if _, err := s.db.GetSession(cookie.Value); err == nil {
			http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
			return
		}
	}
	s.render(w, "login.html", map[string]interface{}{"Mode": s.mode})
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

func (s *Server) handleNewProjectPage(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	s.render(w, "project_new.html", map[string]interface{}{
		"User": user,
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
	runs24h, _ := s.db.CountAgentRunsLast24h(project.ID)

	activeAgents := 0
	for _, a := range agents {
		if a.IsActive {
			activeAgents++
		}
	}

	s.render(w, "project.html", map[string]interface{}{
		"User":         user,
		"Project":      project,
		"Agents":       agents,
		"AgentCount":   len(agents),
		"ActiveAgents": activeAgents,
		"Runs24h":      runs24h,
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

func (s *Server) handleProjectSecrets(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	project, err := s.db.GetProject(id)
	if err != nil || project.UserID != user.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	secrets, _ := s.db.ListProjectSecrets(project.ID)

	s.render(w, "project_secrets.html", map[string]interface{}{
		"User":    user,
		"Project": project,
		"Secrets": secrets,
	})
}

func (s *Server) handleCreateSecret(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	project, err := s.db.GetProject(id)
	if err != nil || project.UserID != user.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	name := r.FormValue("name")
	value := r.FormValue("value")
	if name == "" || value == "" {
		http.Error(w, "Name and value are required", http.StatusBadRequest)
		return
	}

	s.db.CreateProjectSecret(id, name, value)
	http.Redirect(w, r, "/projects/"+strconv.FormatInt(id, 10)+"?tab=secrets", http.StatusSeeOther)
}

func (s *Server) handleDeleteSecret(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	project, err := s.db.GetProject(id)
	if err != nil || project.UserID != user.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	name := r.FormValue("name")
	s.db.DeleteProjectSecret(id, name)
	http.Redirect(w, r, "/projects/"+strconv.FormatInt(id, 10)+"?tab=secrets", http.StatusSeeOther)
}

func (s *Server) handleNewAgentPage(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	project, err := s.db.GetProject(projectID)
	if err != nil || project.UserID != user.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	s.render(w, "agent_new.html", map[string]interface{}{
		"User":    user,
		"Project": project,
	})
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
	mission := r.FormValue("mission")
	prompt := r.FormValue("prompt")
	script := r.FormValue("script")
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
	dockerfile := r.FormValue("dockerfile")

	agent, err := s.db.CreateAgent(projectID, name, mission, prompt, script, cronExpr, workingDir, dockerImage, dockerfile)
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
		"User":      user,
		"Agent":     agent,
		"Project":   project,
		"Runs":      runs,
		"ActiveTab": "agent",
	})
}

func (s *Server) handleAgentWake(w http.ResponseWriter, r *http.Request) {
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
	s.render(w, "agent_wake.html", map[string]interface{}{
		"User":      user,
		"Agent":     agent,
		"Project":   project,
		"Runs":      runs,
		"ActiveTab": "wake",
	})
}

func (s *Server) handleAgentDocker(w http.ResponseWriter, r *http.Request) {
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
	s.render(w, "agent_docker.html", map[string]interface{}{
		"User":      user,
		"Agent":     agent,
		"Project":   project,
		"Runs":      runs,
		"ActiveTab": "docker",
	})
}

func (s *Server) handleAgentSessions(w http.ResponseWriter, r *http.Request) {
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
	s.render(w, "agent_sessions.html", map[string]interface{}{
		"User":      user,
		"Agent":     agent,
		"Project":   project,
		"Runs":      runs,
		"ActiveTab": "sessions",
	})
}

func (s *Server) handleAgentLogs(w http.ResponseWriter, r *http.Request) {
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
	s.render(w, "agent_logs.html", map[string]interface{}{
		"User":      user,
		"Agent":     agent,
		"Project":   project,
		"Runs":      runs,
		"ActiveTab": "logs",
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
	mission := r.FormValue("mission")
	prompt := r.FormValue("prompt")
	script := r.FormValue("script")
	cronExpr := r.FormValue("cron_expression")
	if name == "" || prompt == "" || cronExpr == "" {
		http.Error(w, "Name, prompt, and cron expression are required", http.StatusBadRequest)
		return
	}

	isActive := r.FormValue("is_active") == "on"
	dockerfile := r.FormValue("dockerfile")
	s.db.UpdateAgent(id, name, mission, prompt, script, cronExpr, dockerfile, isActive)

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

func (s *Server) handleRunAgentNow(w http.ResponseWriter, r *http.Request) {
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

	run := s.sched.RunNow(agent.ID)
	if run == nil {
		http.Error(w, "Run failed to start", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/agents/"+strconv.FormatInt(id, 10)+"/runs/"+strconv.FormatInt(run.ID, 10), http.StatusSeeOther)
}

func (s *Server) handleDryRunAgent(w http.ResponseWriter, r *http.Request) {
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

	run := s.sched.DryRun(agent.ID)
	if run == nil {
		http.Error(w, "Run failed to start", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/agents/"+strconv.FormatInt(id, 10)+"/runs/"+strconv.FormatInt(run.ID, 10), http.StatusSeeOther)
}

func (s *Server) handleRunDetail(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	agentID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	runID, _ := strconv.ParseInt(chi.URLParam(r, "run_id"), 10, 64)

	agent, err := s.db.GetAgent(agentID)
	if err != nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	project, err := s.db.GetProject(agent.ProjectID)
	if err != nil || project.UserID != user.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	run, err := s.db.GetAgentRun(runID)
	if err != nil || run.AgentID != agentID {
		http.Error(w, "Run not found", http.StatusNotFound)
		return
	}

	s.render(w, "run.html", map[string]interface{}{
		"User":    user,
		"Agent":   agent,
		"Project": project,
		"Run":     run,
	})
}

func (s *Server) handleRunStream(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	user := UserFromContext(r.Context())
	agentID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	runID, _ := strconv.ParseInt(chi.URLParam(r, "run_id"), 10, 64)

	agent, err := s.db.GetAgent(agentID)
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("Error: agent not found"))
		return
	}

	project, err := s.db.GetProject(agent.ProjectID)
	if err != nil || project.UserID != user.ID {
		conn.WriteMessage(websocket.TextMessage, []byte("Error: forbidden"))
		return
	}

	// Try to attach to live stream
	var stream *scheduler.RunStream
	scheduler.RunStreamsMu.RLock()
	stream = scheduler.RunStreams[runID]
	scheduler.RunStreamsMu.RUnlock()

	if stream != nil {
		lastLen := 0
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stream.Done:
				stream.Mu.Lock()
				data := stream.Buffer.String()[lastLen:]
				stream.Mu.Unlock()
				if len(data) > 0 {
					conn.WriteMessage(websocket.TextMessage, []byte(data))
				}
				conn.WriteMessage(websocket.TextMessage, []byte("\n\n=== RUN COMPLETE ==="))
				return
			case <-ticker.C:
				stream.Mu.Lock()
				newLen := stream.Buffer.Len()
				data := stream.Buffer.String()[lastLen:newLen]
				stream.Mu.Unlock()
				if newLen > lastLen {
					conn.WriteMessage(websocket.TextMessage, []byte(data))
					lastLen = newLen
				}
			}
		}
	}

	// Run already finished, read from DB
	run, err := s.db.GetAgentRun(runID)
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("Error: run not found"))
		return
	}
	conn.WriteMessage(websocket.TextMessage, []byte(run.Output))
	conn.WriteMessage(websocket.TextMessage, []byte("\n\n=== RUN COMPLETE ==="))
}

func (s *Server) handleAgentFiles(w http.ResponseWriter, r *http.Request) {
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

	workspace := filepath.Join("data", "workspaces", strconv.FormatInt(project.ID, 10))
	os.MkdirAll(workspace, 0755)

	relDir := r.URL.Query().Get("dir")
	cleanDir := filepath.Clean("/" + relDir)
	if strings.HasPrefix(cleanDir, "..") {
		cleanDir = "/"
	}
	targetDir := filepath.Join(workspace, cleanDir)

	entries, err := os.ReadDir(targetDir)
	if err != nil {
		entries = []os.DirEntry{}
	}

	var files []map[string]interface{}
	for _, e := range entries {
		info, _ := e.Info()
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		files = append(files, map[string]interface{}{
			"Name":  e.Name(),
			"IsDir": e.IsDir(),
			"Size":  size,
			"Path":  filepath.Join(relDir, e.Name()),
		})
	}

	s.render(w, "agent_files.html", map[string]interface{}{
		"User":       user,
		"Agent":      agent,
		"Project":    project,
		"Files":      files,
		"CurrentDir": relDir,
	})
}

func (s *Server) handleEditFilePage(w http.ResponseWriter, r *http.Request) {
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

	workspace := filepath.Join("data", "workspaces", strconv.FormatInt(project.ID, 10))
	filePath := r.URL.Query().Get("path")
	cleanPath := filepath.Clean("/" + filePath)
	targetPath := filepath.Join(workspace, cleanPath)

	info, err := os.Stat(targetPath)
	if err != nil || info.IsDir() {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	content, err := os.ReadFile(targetPath)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	s.render(w, "agent_file_edit.html", map[string]interface{}{
		"User":      user,
		"Agent":     agent,
		"Project":   project,
		"Path":      filePath,
		"Content":   string(content),
		"ParentDir": filepath.Dir(filePath),
	})
}

func (s *Server) handleSaveFile(w http.ResponseWriter, r *http.Request) {
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

	workspace := filepath.Join("data", "workspaces", strconv.FormatInt(project.ID, 10))
	filePath := r.FormValue("path")
	cleanPath := filepath.Clean("/" + filePath)
	targetPath := filepath.Join(workspace, cleanPath)

	content := r.FormValue("content")
	if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/agents/"+strconv.FormatInt(id, 10)+"/files?dir="+filepath.Dir(filePath), http.StatusSeeOther)
}

func (s *Server) handleUploadFile(w http.ResponseWriter, r *http.Request) {
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

	workspace := filepath.Join("data", "workspaces", strconv.FormatInt(project.ID, 10))
	relDir := r.URL.Query().Get("dir")
	cleanDir := filepath.Clean("/" + relDir)
	targetDir := filepath.Join(workspace, cleanDir)

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	dstPath := filepath.Join(targetDir, header.Filename)
	dst, err := os.Create(dstPath)
	if err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()
	io.Copy(dst, file)

	http.Redirect(w, r, "/agents/"+strconv.FormatInt(id, 10)+"?tab=files", http.StatusSeeOther)
}

func (s *Server) handleDownloadFile(w http.ResponseWriter, r *http.Request) {
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

	workspace := filepath.Join("data", "workspaces", strconv.FormatInt(project.ID, 10))
	filePath := r.URL.Query().Get("path")
	cleanPath := filepath.Clean("/" + filePath)
	targetPath := filepath.Join(workspace, cleanPath)

	info, err := os.Stat(targetPath)
	if err != nil || info.IsDir() {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=\""+filepath.Base(targetPath)+"\"")
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, targetPath)
}

func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
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

	workspace := filepath.Join("data", "workspaces", strconv.FormatInt(project.ID, 10))
	filePath := r.FormValue("path")
	cleanPath := filepath.Clean("/" + filePath)
	targetPath := filepath.Join(workspace, cleanPath)

	os.RemoveAll(targetPath)
	http.Redirect(w, r, "/agents/"+strconv.FormatInt(id, 10)+"?tab=files", http.StatusSeeOther)
}

func (s *Server) handleMkdir(w http.ResponseWriter, r *http.Request) {
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

	workspace := filepath.Join("data", "workspaces", strconv.FormatInt(project.ID, 10))
	relDir := r.URL.Query().Get("dir")
	cleanDir := filepath.Clean("/" + relDir)
	targetDir := filepath.Join(workspace, cleanDir)

	name := r.FormValue("name")
	if name != "" {
		os.MkdirAll(filepath.Join(targetDir, name), 0755)
	}
	http.Redirect(w, r, "/agents/"+strconv.FormatInt(id, 10)+"?tab=files", http.StatusSeeOther)
}
