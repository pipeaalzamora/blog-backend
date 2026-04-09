package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	bucket   = "mindblog-media"
	region   = "us-east-1"
	inputFile  = "blog.posts.json"
	outputFile = "blog.posts.migrated.json"
)

var s3c *s3.Client

func main() {
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(region))
	if err != nil {
		log.Fatal("AWS config error:", err)
	}
	s3c = s3.NewFromConfig(cfg)

	in, err := os.Open(inputFile)
	if err != nil {
		log.Fatal("Cannot open input file:", err)
	}
	defer in.Close()

	out, err := os.Create(outputFile)
	if err != nil {
		log.Fatal("Cannot create output file:", err)
	}
	defer out.Close()

	// Atlas exports as a JSON array
	data, err := os.ReadFile(inputFile)
	if err != nil {
		log.Fatal("Cannot read input file:", err)
	}

	var docs []map[string]interface{}
	if err := json.Unmarshal(data, &docs); err != nil {
		log.Fatal("JSON parse error:", err)
	}
	log.Printf("Loaded %d posts", len(docs))

	for _, doc := range docs {
		title, _ := doc["title"].(string)
		id := fmt.Sprintf("%v", doc["_id"])
		log.Printf("Processing post: %s", title)

		// Migrate coverImage
		if cover, ok := doc["coverImage"].(string); ok && strings.HasPrefix(cover, "data:image/") {
			base := strings.Split(cover, "#pos=")[0]
			pos := ""
			if strings.Contains(cover, "#pos=") {
				pos = "#pos=" + strings.Split(cover, "#pos=")[1]
			}
			url, err := uploadBase64(base, id+"-cover")
			if err != nil {
				log.Printf("  ERROR coverImage: %v", err)
			} else {
				doc["coverImage"] = url + pos
				log.Printf("  ✓ coverImage -> %s", url)
			}
		}

		// Migrate inline images in content
		if content, ok := doc["content"].(string); ok && strings.Contains(content, "data:image/") {
			newContent, count := migrateContentImages(content, id)
			if count > 0 {
				doc["content"] = newContent
				log.Printf("  ✓ %d inline images migrated", count)
			}
		}

		migrated, err := json.Marshal(doc)
		if err != nil {
			log.Printf("Marshal error for %s: %v", title, err)
			continue
		}
		out.Write(migrated)
		out.WriteString("\n")
	}

	log.Printf("Done! Migrated file saved to %s", outputFile)
	log.Println("Now run: mongoimport --uri=\"<your-uri>\" --collection=posts --drop --file=blog.posts.migrated.json")
}

func uploadBase64(dataURL string, name string) (string, error) {
	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid data URL")
	}
	meta := parts[0]
	ext := "jpg"
	if strings.Contains(meta, "png") {
		ext = "png"
	} else if strings.Contains(meta, "webp") {
		ext = "webp"
	} else if strings.Contains(meta, "gif") {
		ext = "gif"
	}
	contentType := "image/jpeg"
	if ext != "jpg" {
		contentType = "image/" + ext
	}

	data, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		// try without padding
		data, err = base64.RawStdEncoding.DecodeString(parts[1])
		if err != nil {
			return "", fmt.Errorf("base64 decode error: %v", err)
		}
	}

	key := fmt.Sprintf("posts/%s-%d.%s", name, time.Now().UnixNano(), ext)
	_, err = s3c.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("s3 upload error: %v", err)
	}
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", bucket, region, key), nil
}

func migrateContentImages(content string, postID string) (string, int) {
	count := 0
	for strings.Contains(content, "data:image/") {
		start := strings.Index(content, "data:image/")
		end := -1
		for _, delim := range []string{"\"", "'", "`"} {
			idx := strings.Index(content[start:], delim)
			if idx != -1 && (end == -1 || idx < end) {
				end = idx
			}
		}
		if end == -1 {
			break
		}
		dataURL := content[start : start+end]
		url, err := uploadBase64(dataURL, fmt.Sprintf("%s-inline-%d", postID, count))
		if err != nil {
			log.Printf("  ERROR inline image %d: %v", count, err)
			break
		}
		content = strings.Replace(content, dataURL, url, 1)
		count++
	}
	return content, count
}
