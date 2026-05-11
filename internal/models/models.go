package models

import "time"

type User struct {
	ID        int64     `json:"id"`
	GitHubID  int64     `json:"github_id"`
	Username  string    `json:"username"`
	AvatarURL string    `json:"avatar_url"`
	CreatedAt time.Time `json:"created_at"`
}

type Session struct {
	ID        string    `json:"id"`
	UserID    int64     `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

type Project struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type Agent struct {
	ID               int64     `json:"id"`
	ProjectID        int64     `json:"project_id"`
	Name             string    `json:"name"`
	Mission          string    `json:"mission"`
	Prompt           string    `json:"prompt"`
	Script           string    `json:"script"`
	CronExpression   string    `json:"cron_expression"`
	WorkingDirectory string    `json:"working_directory"`
	DockerImage      string    `json:"docker_image"`
	Dockerfile       string    `json:"dockerfile"`
	IsActive         bool      `json:"is_active"`
	CreatedAt        time.Time `json:"created_at"`
}

type ProjectSecret struct {
	ID        int64     `json:"id"`
	ProjectID int64     `json:"project_id"`
	Name      string    `json:"name"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
}

type AgentRun struct {
	ID         int64      `json:"id"`
	AgentID    int64      `json:"agent_id"`
	Status     string     `json:"status"`
	Output     string     `json:"output"`
	StartedAt  *time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	CreatedAt  time.Time  `json:"created_at"`
}
