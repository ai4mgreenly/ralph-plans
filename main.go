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

	if err := os.MkdirAll(stateDir, 0755); err != nil {
		log.Fatal(err)
	}

	dbFilename := "plans.db"
	if v := os.Getenv("RALPH_PLANS_DB"); v != "" {
		dbFilename = v
	}
	dbPath := filepath.Join(stateDir, dbFilename)
	db, err := openDB(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	lg := &requestLogger{corsOrigin: "http://" + showsHost + ":" + showsPort}

	mux := http.NewServeMux()
	registerRoutes(mux, db)

	addr := plansHost + ":" + plansPort
	fmt.Printf("ralph-plans listening on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, lg.wrap(mux)))
}
