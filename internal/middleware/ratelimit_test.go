package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// nuevoRouterRate construye un router con el limitador de peticiones aplicado.
func nuevoRouterRate(maxPorSegundo int) *gin.Engine {
	r := gin.New()
	r.GET("/rl", RateLimit(maxPorSegundo), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

// hacerPeticion ejecuta una petición GET a /rl usando la IP indicada como
// origen y devuelve el status HTTP resultante.
func hacerPeticion(r *gin.Engine, ip string) int {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rl", nil)
	// Fijamos RemoteAddr para que ClientIP devuelva una IP distinta por test y
	// así aislar el estado global del limitador entre casos.
	req.RemoteAddr = ip + ":12345"
	r.ServeHTTP(w, req)
	return w.Code
}

func TestRateLimit_BloqueaTrasSuperarLimite(t *testing.T) {
	const max = 3
	r := nuevoRouterRate(max)
	ip := "10.0.0.1"

	// Las primeras 'max' peticiones deben pasar dentro de la misma ventana.
	for i := 0; i < max; i++ {
		if code := hacerPeticion(r, ip); code != http.StatusOK {
			t.Fatalf("peticion %d: status = %d; se esperaba 200", i+1, code)
		}
	}

	// La siguiente petición supera el límite y debe devolver 429.
	if code := hacerPeticion(r, ip); code != http.StatusTooManyRequests {
		t.Errorf("peticion %d: status = %d; se esperaba 429", max+1, code)
	}
}

func TestRateLimit_IPsIndependientes(t *testing.T) {
	const max = 2
	r := nuevoRouterRate(max)

	// Cada IP tiene su propio contador: agotar una no debe afectar a otras.
	for n := 0; n < 3; n++ {
		ip := fmt.Sprintf("172.16.0.%d", n+1)
		if code := hacerPeticion(r, ip); code != http.StatusOK {
			t.Errorf("IP %s: primera petición status = %d; se esperaba 200", ip, code)
		}
	}
}
