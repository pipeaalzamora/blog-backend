package posts

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Post struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Title       string        `bson:"title" json:"title"`
	Slug        string        `bson:"slug" json:"slug"`
	Content     string        `bson:"content" json:"content"`
	CoverImage  string        `bson:"coverImage" json:"coverImage"`
	Mood        string        `bson:"mood" json:"mood"`
	Tags        []string      `bson:"tags" json:"tags"`
	ReadingTime int           `bson:"readingTime" json:"readingTime"`
	// WordCount se calcula al vuelo en el listado admin (no se persiste).
	WordCount int    `bson:"-" json:"wordCount,omitempty"`
	Status    string `bson:"status" json:"status"` // draft | published
	CreatedAt   time.Time     `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time     `bson:"updatedAt" json:"updatedAt"`
}

type CreatePostRequest struct {
	Title      string   `json:"title" binding:"required"`
	Slug       string   `json:"slug"`
	Content    string   `json:"content"`
	CoverImage string   `json:"coverImage"`
	Mood       string   `json:"mood"`
	Tags       []string `json:"tags"`
	Status     string   `json:"status"`
}
