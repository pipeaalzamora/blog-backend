package main

import (
	"log"
	"mindblog/internal/auth"
	"mindblog/internal/config"
	"mindblog/internal/media"
	"mindblog/internal/middleware"
	"mindblog/internal/posts"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()
	config.ConnectMongo(cfg.MongoURI, cfg.MongoDB)
	auth.Init(cfg.JWTSecret)
	auth.InitCredentials(cfg.AdminEmail, cfg.AdminPassHash)
	posts.EnsureIndexes()
	media.Init()

	r := gin.Default()

	origins := []string{}
	for _, o := range strings.Split(cfg.FrontendOrigin, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			origins = append(origins, o)
		}
	}

	r.Use(cors.New(cors.Config{
		AllowOrigins:     origins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))
	r.Use(middleware.RateLimit(10))

	api := r.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		api.POST("/auth/login", auth.LoginHandler)
		api.POST("/upload", middleware.AuthRequired(), media.UploadHandler)

		api.GET("/posts", posts.GetPublished)
		api.GET("/posts/random", posts.GetRandom)
		api.GET("/posts/:slug", posts.GetBySlug)

		protected := api.Group("/")
		protected.Use(middleware.AuthRequired())
		{
			protected.GET("posts/all", posts.GetAll)
			protected.POST("posts", posts.CreatePost)
			protected.PUT("posts/:id", posts.UpdatePost)
			protected.DELETE("posts/:id", posts.DeletePost)
			protected.PATCH("posts/:id/publish", posts.TogglePublishPost)
		}
	}

	port := cfg.Port
	if port == "" {
		port = "8080"
	}
	log.Println("Server running on :" + port)
	r.Run(":" + port)
}
