package main

import (
	"context"
	"errors"
	"log"
	"mindblog/internal/config"
	"mindblog/internal/firebaseauth"
	"mindblog/internal/media"
	"mindblog/internal/middleware"
	"mindblog/internal/posts"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()
	config.ConnectMongo(cfg.MongoURI, cfg.MongoDB)
	firebaseauth.Init(cfg.FirebaseProjectID, cfg.AdminEmails)
	if err := posts.EnsureIndexes(); err != nil {
		log.Fatal("MongoDB index error:", err)
	}
	media.Init(cfg.S3Bucket, cfg.AWSRegion)

	r := gin.Default()

	// Configurar proxies de confianza. Corre detrás del proxy de Render:
	// si TRUSTED_PROXIES está vacío no se confía en ninguno (nil).
	var proxies []string
	if len(cfg.TrustedProxies) > 0 {
		proxies = cfg.TrustedProxies
	}
	if err := r.SetTrustedProxies(proxies); err != nil {
		log.Printf("SetTrustedProxies: %v", err)
	}

	origins := []string{}
	for _, o := range strings.Split(cfg.FrontendOrigin, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			origins = append(origins, o)
		}
	}

	r.Use(cors.New(cors.Config{
		AllowOrigins: origins,
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Content-Type", "Authorization"},
	}))
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.BodyLimit(1 << 20))
	r.Use(middleware.RateLimit(10))

	api := r.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
			defer cancel()
			if err := config.PingMongo(ctx); err != nil {
				log.Printf("health: mongo ping failed: %v", err)
				c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		api.POST("/upload", middleware.AuthRequired(), media.UploadHandler)

		api.GET("/posts", posts.GetPublished)
		api.GET("/posts/random", posts.GetRandom)
		api.GET("/posts/:slug", posts.GetBySlug)

		protected := api.Group("/")
		protected.Use(middleware.AuthRequired())
		{
			protected.GET("posts/all", posts.GetAll)
			protected.GET("posts/id/:id", posts.GetByID)
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

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// Arrancar el servidor en una goroutine para poder escuchar señales.
	go func() {
		log.Println("Server running on :" + port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("server error:", err)
		}
	}()

	// Graceful shutdown ante SIGINT/SIGTERM.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
	log.Println("Server stopped")
}
