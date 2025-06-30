package main

import (
	"log"
	"time"
	"wedding-back/config"
	"wedding-back/database"
	"wedding-back/handlers"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	db := config.ConnectDB()
	database.RunMigrations(db)

	r := gin.Default()

	// CORS middleware — настраиваем под фронт (замени origin на нужный адрес фронта)
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "https://wedding-call.ru"}, // пример: фронт работает на 3000 порту
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Authorization", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.POST("/login", handlers.LoginHandler)         // без авторизации
	r.POST("/guest", handlers.AddGuestHandler(db))  // тоже без авторизации
	r.GET("/guests", handlers.GetGuestsHandler(db)) // можно оставить без авторизации
	r.DELETE("/guest/:id", handlers.AuthMiddleware(), handlers.DeleteGuestGroupHandler(db))
	r.PATCH("/guest/:id", handlers.AuthMiddleware(), handlers.EditGuestHandler(db))

	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
