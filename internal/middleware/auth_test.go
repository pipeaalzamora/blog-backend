package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	// Modo test de gin para silenciar logs y desactivar comprobaciones de debug.
	gin.SetMode(gin.TestMode)
}

// nuevoRouterAuth construye un router mínimo protegido por AuthRequired.
func nuevoRouterAuth() *gin.Engine {
	r := gin.New()
	r.GET("/protegido", AuthRequired(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func TestAuthRequired_SinHeader(t *testing.T) {
	r := nuevoRouterAuth()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protegido", nil)

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; se esperaba 401 sin header Authorization", w.Code)
	}
}

func TestAuthRequired_SinPrefijoBearer(t *testing.T) {
	r := nuevoRouterAuth()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protegido", nil)
	// Header presente pero sin el prefijo "Bearer ".
	req.Header.Set("Authorization", "Token abc123")

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; se esperaba 401 con formato inválido", w.Code)
	}
}

func TestAuthRequired_TokenMalformado(t *testing.T) {
	r := nuevoRouterAuth()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protegido", nil)
	// Un token malformado falla al parsear el JWT antes de resolver la clave,
	// por lo que no se realiza ninguna petición de red y se devuelve 401.
	req.Header.Set("Authorization", "Bearer token-malformado")

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; se esperaba 401 con token malformado", w.Code)
	}
}
