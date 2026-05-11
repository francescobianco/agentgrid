package main

import (
	"log"
	"net/http"
	"os"

	"agentgrid/internal/db"
	"agentgrid/internal/scheduler"
	"agentgrid/internal/server"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "data/agentgrid.db"
	}

	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	if err := database.Migrate(); err != nil {
		log.Fatal(err)
	}

	sched := scheduler.New(database)
	sched.Start()
	defer sched.Stop()

	srv := server.New(database, sched)

	log.Printf("AgentGrid starting on :%s", port)
	if err := http.ListenAndServe(":"+port, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}
