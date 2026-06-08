package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devteam/backend/internal/config"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/service"
	"github.com/google/uuid"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := repository.NewPostgresDB(cfg.DB)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.DB()

	tm := repository.NewTransactionManager(db)
	teamRepo := repository.NewTeamRepository(db)
	teamSvc := service.NewTeamService(teamRepo, tm)

	projectID := uuid.MustParse("cd339e86-9fa1-4666-bfc0-44ac29bc1bee")
	agentID := uuid.MustParse("55bb6db0-cb3b-4a74-82be-fc7feb7ee82d")

	teams, err := teamSvc.ListByProjectID(context.Background(), projectID)
	if err != nil {
		log.Fatalf("ListByProjectID failed: %v", err)
	}
	
	found := false
	for _, team := range teams {
		fmt.Printf("Team %s (%s) has %d agents\n", team.ID, team.Type, len(team.Agents))
		for _, a := range team.Agents {
			fmt.Printf(" - Agent %s (%s)\n", a.ID, a.Name)
			if a.ID == agentID {
				found = true
			}
		}
	}
	
	if found {
		fmt.Println("Agent IS in team! checkAgentInTeam will return nil.")
	} else {
		fmt.Println("Agent NOT found in team! checkAgentInTeam will return ErrAgentNotInTeam.")
	}
}
