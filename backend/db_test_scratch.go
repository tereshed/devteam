package main

import (
	"fmt"
	"github.com/devteam/backend/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
)

func main() {
	dsn := "host=localhost port=5433 user=yugabyte password=yugabyte dbname=yugabyte sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	var task models.Task
	taskID := "5600eeec-dc27-45ef-ad28-a28953a85b32"
	err = db.Preload("Project").Where("id = ?", taskID).First(&task).Error
	if err != nil {
		log.Fatalf("failed to query task: %v", err)
	}

	fmt.Printf("Task ID: %s\n", task.ID)
	fmt.Printf("Task BranchName: %v\n", task.BranchName)
	if task.BranchName != nil {
		fmt.Printf("Task BranchName string: %s\n", *task.BranchName)
	}
	if task.Project != nil {
		fmt.Printf("Project Loaded: %+v\n", task.Project)
		fmt.Printf("Project GitURL: %s\n", task.Project.GitURL)
	} else {
		fmt.Printf("Project is NIL!\n")
	}
}
