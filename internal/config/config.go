package config

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Config struct {
	AdminEmail     string
	AdminPassHash  string
	JWTSecret      string
	MongoURI       string
	MongoDB        string
	Port           string
	FrontendOrigin string
	S3Bucket       string
	AWSRegion      string
}

var DB *mongo.Database

func Load() *Config {
	_ = godotenv.Load()
	cfg := &Config{
		AdminEmail:     env("ADMIN_EMAIL"),
		AdminPassHash:  env("ADMIN_PASSWORD_HASH"),
		JWTSecret:      env("JWT_SECRET"),
		MongoURI:       env("MONGODB_URI"),
		MongoDB:        env("MONGODB_DB"),
		Port:           env("PORT"),
		FrontendOrigin: env("FRONTEND_ORIGIN"),
		S3Bucket:       env("S3_BUCKET"),
		AWSRegion:      env("AWS_REGION"),
	}
	cfg.validate()
	return cfg
}

func env(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func (c *Config) validate() {
	required := map[string]string{
		"ADMIN_EMAIL":         c.AdminEmail,
		"ADMIN_PASSWORD_HASH": c.AdminPassHash,
		"JWT_SECRET":          c.JWTSecret,
		"MONGODB_URI":         c.MongoURI,
		"MONGODB_DB":          c.MongoDB,
		"FRONTEND_ORIGIN":     c.FrontendOrigin,
		"S3_BUCKET":           c.S3Bucket,
	}
	for key, value := range required {
		if value == "" {
			log.Fatalf("missing required environment variable: %s", key)
		}
	}
	if len(c.JWTSecret) < 32 {
		log.Fatal("JWT_SECRET must be at least 32 characters")
	}
}

func ConnectMongo(uri, dbName string) {
	opts := options.Client().
		ApplyURI(uri).
		// Personal blog service: keep the pool modest, reuse idle connections,
		// and fail fast on topology or network stalls.
		SetMaxPoolSize(20).
		SetMinPoolSize(0).
		SetMaxConnIdleTime(5 * time.Minute).
		SetConnectTimeout(10 * time.Second).
		SetServerSelectionTimeout(5 * time.Second)

	client, err := mongo.Connect(opts)
	if err != nil {
		log.Fatal("MongoDB connect error:", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatal("MongoDB ping error:", err)
	}
	DB = client.Database(dbName)
	log.Println("MongoDB connected")
}
