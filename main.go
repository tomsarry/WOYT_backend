package main

import (
	"os"

	"github.com/tomsarry/woyt_backend/handlers"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

// retrieve the env variables first
func init() {
	godotenv.Load()
}

func main() {

	r := gin.Default()
	r.MaxMultipartMemory = 8 << 20
	r.Static("/", "./public")
	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{os.Getenv("WEBSITE")},
		AllowMethods: []string{"GET", "PUT", "POST"},
	}))

	r.POST("/upload", handlers.UploadHandler)

	r.Run()
}
