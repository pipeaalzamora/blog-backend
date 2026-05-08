package posts

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

func wordCount(s string) int {
	return len(strings.FieldsFunc(s, func(r rune) bool { return unicode.IsSpace(r) }))
}

func calcReadingTime(content string) int {
	wc := wordCount(content)
	return int(math.Ceil(float64(wc) / 200.0))
}

func slugify(title string) string {
	// Normalizar unicode: descomponer caracteres acentuados (NFD)
	// para separar la letra base del diacrítico y luego eliminar los diacríticos.
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	normalized, _, err := transform.String(t, title)
	if err != nil {
		normalized = title
	}
	s := strings.ToLower(normalized)
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else if unicode.IsSpace(r) || r == '-' {
			b.WriteRune('-')
		}
	}
	// Colapsar guiones múltiples consecutivos
	result := strings.Trim(b.String(), "-")
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	return result
}

func GetPublished(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	posts, total, err := FindPublished(page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"posts": posts, "total": total, "page": page, "limit": limit})
}

func GetAll(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	posts, total, err := FindAll(page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"posts": posts, "total": total, "page": page, "limit": limit})
}

func GetBySlug(c *gin.Context) {
	post, err := FindBySlug(c.Param("slug"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
		return
	}
	c.JSON(http.StatusOK, post)
}

func GetRandom(c *gin.Context) {
	post, err := FindRandom()
	if err != nil || post == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no posts found"})
		return
	}
	c.JSON(http.StatusOK, post)
}

func CreatePost(c *gin.Context) {
	var req CreatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	slug := req.Slug
	if slug == "" {
		slug = slugify(req.Title)
	}
	if req.Status == "" {
		req.Status = "draft"
	}
	post := &Post{
		Title:       req.Title,
		Slug:        slug,
		Content:     req.Content,
		CoverImage:  req.CoverImage,
		Mood:        req.Mood,
		Tags:        req.Tags,
		ReadingTime: calcReadingTime(req.Content),
		Status:      req.Status,
	}
	if err := Create(post); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, post)
}

func UpdatePost(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req CreatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	slug := req.Slug
	if slug == "" {
		slug = slugify(req.Title)
	}
	update := bson.M{
		"title":       req.Title,
		"slug":        slug,
		"content":     req.Content,
		"coverImage":  req.CoverImage,
		"mood":        req.Mood,
		"tags":        req.Tags,
		"readingTime": calcReadingTime(req.Content),
		"status":      req.Status,
		"updatedAt":   time.Now(),
	}
	if err := Update(id, update); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	post, _ := FindByID(id)
	c.JSON(http.StatusOK, post)
}

func DeletePost(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func TogglePublishPost(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	post, err := TogglePublish(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, post)
}
