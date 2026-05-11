package db

import (
	"database/sql"
	"fmt"
	"time"

	"agentgrid/internal/models"

	_ "modernc.org/sqlite"
)

type Database struct {
	*sql.DB
}

func Open(path string) (*Database, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1)
	return &Database{db}, nil
}

func (db *Database) Migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			github_id INTEGER UNIQUE,
			username TEXT NOT NULL,
			avatar_url TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS projects (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id),
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS agents (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			prompt TEXT NOT NULL,
			cron_expression TEXT NOT NULL,
			working_directory TEXT DEFAULT '/work',
			docker_image TEXT DEFAULT 'alpine:latest',
			dockerfile TEXT DEFAULT '',
			is_active INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS agent_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			agent_id INTEGER NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
			status TEXT NOT NULL DEFAULT 'pending',
			output TEXT DEFAULT '',
			started_at DATETIME,
			finished_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

func (db *Database) CreateUser(githubID int64, username, avatarURL string) (*models.User, error) {
	res, err := db.Exec(`INSERT INTO users (github_id, username, avatar_url) VALUES (?, ?, ?)
		ON CONFLICT(github_id) DO UPDATE SET username=excluded.username, avatar_url=excluded.avatar_url`, githubID, username, avatarURL)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	if id == 0 {
		db.QueryRow(`SELECT id FROM users WHERE github_id = ?`, githubID).Scan(&id)
	}
	return db.GetUser(id)
}

func (db *Database) GetUser(id int64) (*models.User, error) {
	u := &models.User{}
	err := db.QueryRow(`SELECT id, github_id, username, avatar_url, created_at FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.GitHubID, &u.Username, &u.AvatarURL, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (db *Database) GetOrCreateLocalUser() (*models.User, error) {
	u := &models.User{}
	err := db.QueryRow(`SELECT id, github_id, username, avatar_url, created_at FROM users WHERE github_id = 0`).
		Scan(&u.ID, &u.GitHubID, &u.Username, &u.AvatarURL, &u.CreatedAt)
	if err == nil {
		return u, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	res, err := db.Exec(`INSERT INTO users (github_id, username, avatar_url) VALUES (0, 'local', '')`)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return db.GetUser(id)
}

func (db *Database) CreateSession(userID int64, sessionID string) error {
	_, err := db.Exec(`INSERT INTO sessions (id, user_id) VALUES (?, ?)`, sessionID, userID)
	return err
}

func (db *Database) GetSession(sessionID string) (*models.Session, error) {
	s := &models.Session{}
	err := db.QueryRow(`SELECT id, user_id, created_at FROM sessions WHERE id = ?`, sessionID).
		Scan(&s.ID, &s.UserID, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (db *Database) DeleteSession(sessionID string) error {
	_, err := db.Exec(`DELETE FROM sessions WHERE id = ?`, sessionID)
	return err
}

func (db *Database) CreateProject(userID int64, name, description string) (*models.Project, error) {
	res, err := db.Exec(`INSERT INTO projects (user_id, name, description) VALUES (?, ?, ?)`, userID, name, description)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return db.GetProject(id)
}

func (db *Database) GetProject(id int64) (*models.Project, error) {
	p := &models.Project{}
	err := db.QueryRow(`SELECT id, user_id, name, description, created_at FROM projects WHERE id = ?`, id).
		Scan(&p.ID, &p.UserID, &p.Name, &p.Description, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (db *Database) ListProjects(userID int64) ([]models.Project, error) {
	rows, err := db.Query(`SELECT id, user_id, name, description, created_at FROM projects WHERE user_id = ? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var projects []models.Project
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.Description, &p.CreatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, nil
}

func (db *Database) DeleteProject(id int64) error {
	_, err := db.Exec(`DELETE FROM projects WHERE id = ?`, id)
	return err
}

func (db *Database) CreateAgent(projectID int64, name, prompt, cronExpression, workingDirectory, dockerImage, dockerfile string) (*models.Agent, error) {
	res, err := db.Exec(`INSERT INTO agents (project_id, name, prompt, cron_expression, working_directory, docker_image, dockerfile) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		projectID, name, prompt, cronExpression, workingDirectory, dockerImage, dockerfile)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return db.GetAgent(id)
}

func (db *Database) GetAgent(id int64) (*models.Agent, error) {
	a := &models.Agent{}
	err := db.QueryRow(`SELECT id, project_id, name, prompt, cron_expression, working_directory, docker_image, dockerfile, is_active, created_at FROM agents WHERE id = ?`, id).
		Scan(&a.ID, &a.ProjectID, &a.Name, &a.Prompt, &a.CronExpression, &a.WorkingDirectory, &a.DockerImage, &a.Dockerfile, &a.IsActive, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (db *Database) ListAgents(projectID int64) ([]models.Agent, error) {
	rows, err := db.Query(`SELECT id, project_id, name, prompt, cron_expression, working_directory, docker_image, dockerfile, is_active, created_at FROM agents WHERE project_id = ? ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var agents []models.Agent
	for rows.Next() {
		var a models.Agent
		if err := rows.Scan(&a.ID, &a.ProjectID, &a.Name, &a.Prompt, &a.CronExpression, &a.WorkingDirectory, &a.DockerImage, &a.Dockerfile, &a.IsActive, &a.CreatedAt); err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, nil
}

func (db *Database) DeleteAgent(id int64) error {
	_, err := db.Exec(`DELETE FROM agents WHERE id = ?`, id)
	return err
}

func (db *Database) UpdateAgent(id int64, name, prompt, cronExpression, dockerfile string, isActive bool) error {
	_, err := db.Exec(`UPDATE agents SET name=?, prompt=?, cron_expression=?, dockerfile=?, is_active=? WHERE id=?`,
		name, prompt, cronExpression, dockerfile, isActive, id)
	return err
}

func (db *Database) ListActiveAgents() ([]models.Agent, error) {
	rows, err := db.Query(`SELECT id, project_id, name, prompt, cron_expression, working_directory, docker_image, dockerfile, is_active, created_at FROM agents WHERE is_active = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var agents []models.Agent
	for rows.Next() {
		var a models.Agent
		if err := rows.Scan(&a.ID, &a.ProjectID, &a.Name, &a.Prompt, &a.CronExpression, &a.WorkingDirectory, &a.DockerImage, &a.Dockerfile, &a.IsActive, &a.CreatedAt); err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, nil
}

func (db *Database) CreateAgentRun(agentID int64) (*models.AgentRun, error) {
	res, err := db.Exec(`INSERT INTO agent_runs (agent_id, status) VALUES (?, 'pending')`, agentID)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return db.GetAgentRun(id)
}

func (db *Database) GetAgentRun(id int64) (*models.AgentRun, error) {
	r := &models.AgentRun{}
	err := db.QueryRow(`SELECT id, agent_id, status, output, started_at, finished_at, created_at FROM agent_runs WHERE id = ?`, id).
		Scan(&r.ID, &r.AgentID, &r.Status, &r.Output, &r.StartedAt, &r.FinishedAt, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (db *Database) ListAgentRuns(agentID int64) ([]models.AgentRun, error) {
	rows, err := db.Query(`SELECT id, agent_id, status, output, started_at, finished_at, created_at FROM agent_runs WHERE agent_id = ? ORDER BY created_at DESC`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var runs []models.AgentRun
	for rows.Next() {
		var r models.AgentRun
		if err := rows.Scan(&r.ID, &r.AgentID, &r.Status, &r.Output, &r.StartedAt, &r.FinishedAt, &r.CreatedAt); err != nil {
			return nil, err
		}
		runs = append(runs, r)
	}
	return runs, nil
}

func (db *Database) UpdateAgentRun(id int64, status, output string) error {
	now := time.Now()
	if status == "running" {
		_, err := db.Exec(`UPDATE agent_runs SET status=?, started_at=? WHERE id=?`, status, now, id)
		return err
	}
	_, err := db.Exec(`UPDATE agent_runs SET status=?, output=?, finished_at=? WHERE id=?`, status, output, now, id)
	return err
}
