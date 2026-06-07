package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
)

var (
	s3Client *s3.Client
	bucket   string
	region   string
)

const maxUploadBytes = 5 << 20

var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

func Init(bucketName, regionName string) {
	bucket = strings.TrimSpace(bucketName)
	region = strings.TrimSpace(regionName)
	if region == "" {
		region = "us-east-1"
	}
	if bucket == "" {
		log.Fatal("missing required environment variable: S3_BUCKET")
	}

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(region))
	if err != nil {
		log.Fatal("failed to load AWS config:", err)
	}
	s3Client = s3.NewFromConfig(cfg)
}

func UploadHandler(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no valid image file provided"})
		return
	}
	defer file.Close()
	if header.Size > maxUploadBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "image exceeds 5 MB limit"})
		return
	}

	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot read file"})
		return
	}
	if n == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty file"})
		return
	}
	contentType := http.DetectContentType(buf[:n])
	ext, ok := allowedImageTypes[contentType]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only jpg, png, webp or gif images allowed"})
		return
	}

	body, err := io.ReadAll(io.LimitReader(io.MultiReader(bytes.NewReader(buf[:n]), file), maxUploadBytes+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot read file"})
		return
	}
	if int64(len(body)) > maxUploadBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "image exceeds 5 MB limit"})
		return
	}
	key := fmt.Sprintf("posts/%d%s", time.Now().UnixNano(), ext)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(body),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(int64(len(body))),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "upload failed"})
		return
	}

	url := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", bucket, region, key)
	c.JSON(http.StatusOK, gin.H{"url": url})
}
