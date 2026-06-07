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
	"go.mongodb.org/mongo-driver/v2/mongo"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

const (
	defaultPublicLimit = 10
	defaultAdminLimit  = 20
	maxPublicLimit     = 50
	maxAdminLimit      = 100
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

func parsePagination(c *gin.Context, defaultLimit, maxLimit int) (int, int) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	limit, err := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(defaultLimit)))
	if err != nil || limit < 1 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	return page, limit
}

func validStatus(status string) bool {
	return status == "draft" || status == "published"
}

func GetPublished(c *gin.Context) {
	page, limit := parsePagination(c, defaultPublicLimit, maxPublicLimit)
	posts, total, err := FindPublished(page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"posts": posts, "total": total, "page": page, "limit": limit})
}

func GetAll(c *gin.Context) {
	page, limit := parsePagination(c, defaultAdminLimit, maxAdminLimit)
	posts, total, err := FindAll(page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"posts": posts, "total": total, "page": page, "limit": limit})
}

func GetByID(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	post, err := FindByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
		return
	}
	c.JSON(http.StatusOK, post)
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
	if !validStatus(req.Status) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
		return
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
	if req.Status == "" {
		req.Status = "draft"
	}
	if !validStatus(req.Status) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
		return
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
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
			return
		}
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
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
			return
		}
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
