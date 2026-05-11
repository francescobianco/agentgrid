package scheduler

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"agentgrid/internal/db"
	"agentgrid/internal/docker"
	"agentgrid/internal/models"
	"github.com/robfig/cron/v3"
)

type RunStream struct {
	Buffer bytes.Buffer
	Mu     sync.Mutex
	Done   chan struct{}
}

var (
	RunStreams   = make(map[int64]*RunStream)
	RunStreamsMu sync.RWMutex
)

type Scheduler struct {
	db      *db.Database
	cron    *cron.Cron
	entries map[int64]cron.EntryID
	mu      sync.RWMutex
}

func New(database *db.Database) *Scheduler {
	return &Scheduler{
		db:      database,
		cron:    cron.New(),
		entries: make(map[int64]cron.EntryID),
	}
}

func (s *Scheduler) Start() {
	agents, err := s.db.ListActiveAgents()
	if err != nil {
		log.Printf("Error loading active agents: %v", err)
		return
	}
	for _, a := range agents {
		if err := s.addAgent(a.ID, a.CronExpression); err != nil {
			log.Printf("Error scheduling agent %d: %v", a.ID, err)
		}
	}
	s.cron.Start()
	log.Printf("Scheduler started with %d active agents", len(agents))
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}

func (s *Scheduler) addAgent(agentID int64, cronExpr string) error {
	entryID, err := s.cron.AddFunc(cronExpr, func() {
		s.executeAgent(agentID)
	})
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.entries[agentID] = entryID
	s.mu.Unlock()
	return nil
}

func (s *Scheduler) RemoveAgent(agentID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entryID, ok := s.entries[agentID]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, agentID)
	}
}

func (s *Scheduler) AddAgent(agentID int64, cronExpr string) error {
	s.RemoveAgent(agentID)
	return s.addAgent(agentID, cronExpr)
}

func (s *Scheduler) executeAgent(agentID int64) {
	s.runAgent(agentID, false)
}

func (s *Scheduler) runAgent(agentID int64, dry bool) *models.AgentRun {
	agent, err := s.db.GetAgent(agentID)
	if err != nil {
		log.Printf("Error fetching agent %d: %v", agentID, err)
		return nil
	}

	run, err := s.db.CreateAgentRun(agentID)
	if err != nil {
		log.Printf("Error creating run for agent %d: %v", agentID, err)
		return nil
	}

	if dry {
		s.db.UpdateAgentRun(run.ID, "dry-run", "")
	} else {
		s.db.UpdateAgentRun(run.ID, "running", "")
	}

	stream := &RunStream{Done: make(chan struct{})}
	RunStreamsMu.Lock()
	RunStreams[run.ID] = stream
	RunStreamsMu.Unlock()

	image := agent.DockerImage
	if agent.Dockerfile != "" {
		tag := fmt.Sprintf("agentgrid-agent-%d", agent.ID)
		buildResult := docker.BuildImage(agent.Dockerfile, tag)
		if buildResult.Error != nil {
			msg := "Build failed:\n" + buildResult.Output + "\nError: " + buildResult.Error.Error()
			stream.Mu.Lock()
			stream.Buffer.WriteString(msg)
			stream.Mu.Unlock()
			s.db.UpdateAgentRun(run.ID, "failed", msg)
			close(stream.Done)
			RunStreamsMu.Lock()
			delete(RunStreams, run.ID)
			RunStreamsMu.Unlock()
			log.Printf("Agent %d run %d build failed", agentID, run.ID)
			return run
		}
		image = tag
	}

	workspace := filepath.Join("data", "workspaces", strconv.FormatInt(agent.ProjectID, 10))
	os.MkdirAll(workspace, 0755)

	secrets, _ := s.db.ListProjectSecrets(agent.ProjectID)
	env := map[string]string{
		"WAKEUP_PROMPT": agent.Prompt,
		"AGENT_MISSION": agent.Mission,
	}
	for _, sec := range secrets {
		env[sec.Name] = sec.Value
	}

	if dry {
		msg := "Image built and ready.\n" + image
		stream.Mu.Lock()
		stream.Buffer.WriteString(msg)
		stream.Mu.Unlock()
		s.db.UpdateAgentRun(run.ID, "dry-run", msg)
		close(stream.Done)
		RunStreamsMu.Lock()
		delete(RunStreams, run.ID)
		RunStreamsMu.Unlock()
		log.Printf("Agent %d dry-run completed", agentID)
		return run
	}

	reader, err := docker.RunAgentStream(image, workspace, agent.WorkingDirectory, env, agent.Script)
	if err != nil {
		msg := "Failed to start container: " + err.Error()
		stream.Mu.Lock()
		stream.Buffer.WriteString(msg)
		stream.Mu.Unlock()
		s.db.UpdateAgentRun(run.ID, "failed", msg)
		close(stream.Done)
		RunStreamsMu.Lock()
		delete(RunStreams, run.ID)
		RunStreamsMu.Unlock()
		return run
	}

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		stream.Mu.Lock()
		stream.Buffer.WriteString(line + "\n")
		stream.Mu.Unlock()
	}
	reader.Close()

	status := "completed"
	output := stream.Buffer.String()
	if scanner.Err() != nil {
		status = "failed"
		output += "\nError: " + scanner.Err().Error()
	}

	s.db.UpdateAgentRun(run.ID, status, output)
	close(stream.Done)
	RunStreamsMu.Lock()
	delete(RunStreams, run.ID)
	RunStreamsMu.Unlock()
	log.Printf("Agent %d run %d completed with status %s", agentID, run.ID, status)
	return run
}

func (s *Scheduler) RunNow(agentID int64) *models.AgentRun {
	return s.runAgent(agentID, false)
}

func (s *Scheduler) DryRun(agentID int64) *models.AgentRun {
	return s.runAgent(agentID, true)
}
