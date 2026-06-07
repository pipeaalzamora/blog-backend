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
	return context.WithTimeout(context.Background(), 30*time.Second)
}

func FindPublished(page, limit int) ([]Post, int64, error) {
	if posts, total, ok := getCache(page, limit); ok {
		return posts, total, nil
	}

	filter := bson.M{"status": "published"}

	// Run count and find in parallel
	type countResult struct {
		n   int64
		err error
	}
	countCh := make(chan countResult, 1)
	go func() {
		c, cancel := ctx()
		defer cancel()
		n, err := col().CountDocuments(c, filter)
		countCh <- countResult{n, err}
	}()

	skip := int64((page - 1) * limit)
	// Exclude content field in list — fetch full content only on GetBySlug
	projection := bson.M{"content": 0}
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetSkip(skip).SetLimit(int64(limit)).SetProjection(projection)
	c, cancel := ctx()
	defer cancel()
	cur, err := col().Find(c, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	var result []Post
	c2, cancel2 := ctx()
	defer cancel2()
	cur.All(c2, &result)

	cr := <-countCh
	setCache(page, limit, result, cr.n)
	return result, cr.n, cr.err
}

func FindAll(page, limit int) ([]Post, int64, error) {
	type countResult struct {
		n   int64
		err error
	}
	countCh := make(chan countResult, 1)
	go func() {
		c, cancel := ctx()
		defer cancel()
		n, err := col().CountDocuments(c, bson.M{})
		countCh <- countResult{n, err}
	}()

	skip := int64((page - 1) * limit)
	opts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetSkip(skip).
		SetLimit(int64(limit)).
		SetProjection(bson.M{"content": 0})
	c, cancel := ctx()
	defer cancel()
	cur, err := col().Find(c, bson.M{}, opts)
	if err != nil {
		return nil, 0, err
	}
	var result []Post
	c2, cancel2 := ctx()
	defer cancel2()
	cur.All(c2, &result)

	cr := <-countCh
	return result, cr.n, cr.err
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
	if err == nil {
		InvalidateCache()
	}
	return err
}

func Update(id bson.ObjectID, update bson.M) error {
	update["updatedAt"] = time.Now()
	c, cancel := ctx()
	defer cancel()
	res, err := col().UpdateOne(c, bson.M{"_id": id}, bson.M{"$set": update})
	if err == nil {
		if res.MatchedCount == 0 {
			return mongo.ErrNoDocuments
		}
		InvalidateCache()
	}
	return err
}

func Delete(id bson.ObjectID) error {
	c, cancel := ctx()
	defer cancel()
	res, err := col().DeleteOne(c, bson.M{"_id": id})
	if err == nil {
		if res.DeletedCount == 0 {
			return mongo.ErrNoDocuments
		}
		InvalidateCache()
	}
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

func EnsureIndexes() error {
	c, cancel := ctx()
	defer cancel()
	_, err := col().Indexes().CreateMany(c, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "slug", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			// Cubre queries de posts publicados ordenados por fecha
			Keys: bson.D{{Key: "status", Value: 1}, {Key: "createdAt", Value: -1}},
		},
	})
	return err
}
