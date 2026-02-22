package main

import (
	"database/sql"
	"log"
	"time"
)

func startPRPoller(db *sql.DB) {
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			pollSubmittedGoals(db)
		}
	}()
}

func pollSubmittedGoals(db *sql.DB) {
	goals, err := listSubmittedGoalsWithPR(db)
	if err != nil {
		log.Printf("pr-poller: failed to list submitted goals: %v", err)
		return
	}

	for _, g := range goals {
		state, cached := prCache.get(g.Org, g.Repo, *g.PR)
		if !cached {
			freshState, err := checkPRState(g.Org, g.Repo, *g.PR)
			if err != nil {
				log.Printf("pr-poller: goal %d: failed to check PR state: %v", g.ID, err)
				continue
			}
			state = freshState
			prCache.set(g.Org, g.Repo, *g.PR, *freshState)
		}

		var newStatus string
		if state.Merged {
			newStatus = "merged"
		} else if state.Closed {
			newStatus = "rejected"
		}

		if newStatus == "" {
			continue
		}

		if err := updateGoalStatus(db, g.ID, "submitted", newStatus); err != nil {
			if err != sql.ErrNoRows {
				log.Printf("pr-poller: goal %d: failed to transition to %s: %v", g.ID, newStatus, err)
			}
			continue
		}

		log.Printf("pr-poller: goal %d transitioned submitted → %s (PR %s/%s#%d)", g.ID, newStatus, g.Org, g.Repo, *g.PR)
	}
}
