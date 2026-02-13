package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("%s is required", key)
	}
	return v
}

func main() {
	plansHost := requireEnv("RALPH_PLANS_HOST")
	plansPort := requireEnv("RALPH_PLANS_PORT")
	showsHost := requireEnv("RALPH_SHOWS_HOST")
	showsPort := requireEnv("RALPH_SHOWS_PORT")

	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	stateDir := filepath.Join(home, ".local", "state", "ralph")
	logDir := filepath.Join(stateDir, "logs")

	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Fatal(err)
	}

	dbPath := filepath.Join(stateDir, "plans.db")
	db, err := openDB(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	logFile, err := os.OpenFile(filepath.Join(logDir, "ralph-plans.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer logFile.Close()

	lg := &requestLogger{f: logFile, corsOrigin: "http://" + showsHost + ":" + showsPort}

	mux := http.NewServeMux()
	registerRoutes(mux, db)

	addr := plansHost + ":" + plansPort
	fmt.Printf("ralph-plans listening on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, lg.wrap(mux)))
}
