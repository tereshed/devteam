package main

import (
	"fmt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
	"os"
)

type User struct {
	ID   string `gorm:"primaryKey"`
	Name string
	Age  int
}

func main() {
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			LogLevel: logger.Info,
		},
	)
	db, _ := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{Logger: newLogger})
	db.AutoMigrate(&User{})

	db.Create(&User{ID: "old-id", Name: "John", Age: 20})

	newUser := &User{ID: "new-id", Name: "John", Age: 30}
	db.Where("name = ?", "John").Assign(map[string]interface{}{"age": 30}).FirstOrCreate(newUser)

	var users []User
	db.Find(&users)
	fmt.Printf("Users in DB: %+v\n", users)
	fmt.Printf("newUser struct after: %+v\n", newUser)
}
