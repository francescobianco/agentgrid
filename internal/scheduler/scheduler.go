package scheduler

import (
	"log"
	"sync"

	"agentgrid/internal/db"
	"agentgrid/internal/docker"
	"github.com/robfig/cron/v3"
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
	agent, err := s.db.GetAgent(agentID)
	if err != nil {
		log.Printf("Error fetching agent %d: %v", agentID, err)
		return
	}

	run, err := s.db.CreateAgentRun(agentID)
	if err != nil {
		log.Printf("Error creating run for agent %d: %v", agentID, err)
		return
	}

	s.db.UpdateAgentRun(run.ID, "running", "")

	result := docker.RunAgent(agent.Prompt, agent.DockerImage)

	status := "completed"
	output := result.Output
	if result.Error != nil {
		status = "failed"
		output += "\nError: " + result.Error.Error()
	}

	s.db.UpdateAgentRun(run.ID, status, output)
	log.Printf("Agent %d run %d completed with status %s", agentID, run.ID, status)
}
