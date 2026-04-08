package posts

import (
	"context"
	"math/rand"
	"mindblog/internal/config"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func col() *mongo.Collection {
	return config.DB.Collection("posts")
}

func ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}

func FindPublished(page, limit int) ([]Post, int64, error) {
	filter := bson.M{"status": "published"}
	c, cancel := ctx()
	defer cancel()
	total, _ := col().CountDocuments(c, filter)
	skip := int64((page - 1) * limit)
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetSkip(skip).SetLimit(int64(limit))
	cur, err := col().Find(c, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	var result []Post
	c2, cancel2 := ctx()
	defer cancel2()
	cur.All(c2, &result)
	return result, total, nil
}

func FindAll() ([]Post, error) {
	c, cancel := ctx()
	defer cancel()
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cur, err := col().Find(c, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	var result []Post
	c2, cancel2 := ctx()
	defer cancel2()
	cur.All(c2, &result)
	return result, nil
}

func FindBySlug(slug string) (*Post, error) {
	c, cancel := ctx()
	defer cancel()
	var p Post
	err := col().FindOne(c, bson.M{"slug": slug, "status": "published"}).Decode(&p)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func FindByID(id bson.ObjectID) (*Post, error) {
	c, cancel := ctx()
	defer cancel()
	var p Post
	err := col().FindOne(c, bson.M{"_id": id}).Decode(&p)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func FindRandom() (*Post, error) {
	c, cancel := ctx()
	defer cancel()
	total, err := col().CountDocuments(c, bson.M{"status": "published"})
	if err != nil || total == 0 {
		return nil, err
	}
	skip := rand.Int63n(total)
	opts := options.FindOne().SetSkip(skip)
	c2, cancel2 := ctx()
	defer cancel2()
	var p Post
	err = col().FindOne(c2, bson.M{"status": "published"}, opts).Decode(&p)
	return &p, err
}

func Create(p *Post) error {
	p.ID = bson.NewObjectID()
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	c, cancel := ctx()
	defer cancel()
	_, err := col().InsertOne(c, p)
	return err
}

func Update(id bson.ObjectID, update bson.M) error {
	update["updatedAt"] = time.Now()
	c, cancel := ctx()
	defer cancel()
	_, err := col().UpdateOne(c, bson.M{"_id": id}, bson.M{"$set": update})
	return err
}

func Delete(id bson.ObjectID) error {
	c, cancel := ctx()
	defer cancel()
	_, err := col().DeleteOne(c, bson.M{"_id": id})
	return err
}

func TogglePublish(id bson.ObjectID) (*Post, error) {
	p, err := FindByID(id)
	if err != nil {
		return nil, err
	}
	newStatus := "published"
	if p.Status == "published" {
		newStatus = "draft"
	}
	err = Update(id, bson.M{"status": newStatus})
	if err != nil {
		return nil, err
	}
	p.Status = newStatus
	return p, nil
}

func EnsureIndexes() {
	c, cancel := ctx()
	defer cancel()
	col().Indexes().CreateOne(c, mongo.IndexModel{
		Keys:    bson.D{{Key: "slug", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
}
