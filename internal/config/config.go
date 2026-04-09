package config

import (
	"context"
	"log"
	"os"
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
}

var DB *mongo.Database

func Load() *Config {
	_ = godotenv.Load()
	return &Config{
		AdminEmail:     os.Getenv("ADMIN_EMAIL"),
		AdminPassHash:  os.Getenv("ADMIN_PASSWORD_HASH"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		MongoURI:       os.Getenv("MONGODB_URI"),
		MongoDB:        os.Getenv("MONGODB_DB"),
		Port:           os.Getenv("PORT"),
		FrontendOrigin: os.Getenv("FRONTEND_ORIGIN"),
	}
}

func ConnectMongo(uri, dbName string) {
	opts := options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(5).
		SetMinPoolSize(1).
		SetMaxConnIdleTime(25 * time.Second). // Atlas M0 cierra idle connections a los 30s
		SetHeartbeatInterval(10 * time.Second).
		SetRetryReads(true).
		SetRetryWrites(true)

	client, err := mongo.Connect(opts)
	if err != nil {
		log.Fatal("MongoDB connect error:", err)
	}
	if err := client.Ping(context.Background(), nil); err != nil {
		log.Fatal("MongoDB ping error:", err)
	}
	DB = client.Database(dbName)
	log.Println("MongoDB connected")
}
