package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	dsn := "host=localhost port=5433 user=yugabyte password=yugabyte dbname=yugabyte sslmode=disable"
	dbConn, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	projectID := uuid.MustParse("cd339e86-9fa1-4666-bfc0-44ac29bc1bee")
	
	task := models.Task{
		ProjectID:   projectID,
		Title:       "Test Merger parallel fix (Auto 2)",
		Description: "Создай два очень простых текстовых файла (test1.txt и test2.txt) параллельно и смерджи их.",
		State:       models.TaskStatePending,
	}

	if err := dbConn.Create(&task).Error; err != nil {
		log.Fatalf("failed to create task: %v", err)
	}
	fmt.Printf("Task created: %s\n", task.ID.String())

	// Also we might need to enqueue the router event for this task, 
	// but the app might poll pending tasks. If not, we just update state to 'pending'
	// and wait for the router to pick it up. Wait, orchestrator creates router_jobs!
	// Let's create an orchestrator record and a router_decisions entry if needed.
	// Actually, the HTTP handler calls taskRepo.Create() and that's it. Let's see.
}
