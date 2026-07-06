package firebaseauth

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// projectoTest es el ID de proyecto usado en todas las pruebas.
const proyectoTest = "proyecto-test"

// errRoundTripper es un RoundTripper que siempre falla, para garantizar que
// ningún test haga una petición de red real a Google.
type errRoundTripper struct{}

func (errRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("red deshabilitada en tests")
}

// setup inicializa el verificador y precarga la cache de claves con una clave
// pública controlada bajo el kid "testkid", evitando cualquier descarga de red.
func setup(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("no se pudo generar la clave RSA: %v", err)
	}

	Init(proyectoTest, []string{"admin@test.com"})

	// Inyectamos directamente la clave pública en la cache no exportada y
	// fijamos una expiración futura para saltarnos la descarga de Google.
	keysMu.Lock()
	keysCache = map[string]*rsa.PublicKey{"testkid": &priv.PublicKey}
	keysExpiry = time.Now().Add(time.Hour)
	keysMu.Unlock()

	// Cualquier refresh de claves (p. ej. kid desconocido) fallará sin red real.
	httpClient = &http.Client{Transport: errRoundTripper{}}

	return priv
}

// firmarToken crea un ID token RS256 firmado con priv y el kid indicado.
func firmarToken(t *testing.T, priv *rsa.PrivateKey, kid string, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	s, err := tok.SignedString(priv)
	if err != nil {
		t.Fatalf("no se pudo firmar el token: %v", err)
	}
	return s
}

// claimsValidos genera un set de claims correcto para el proyecto de prueba.
func claimsValidos(email string) jwt.MapClaims {
	now := time.Now()
	return jwt.MapClaims{
		"iss":   "https://securetoken.google.com/" + proyectoTest,
		"aud":   proyectoTest,
		"sub":   "uid-123",
		"email": email,
		"iat":   now.Add(-time.Minute).Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}
}

func TestVerifyIDToken_Valido(t *testing.T) {
	priv := setup(t)
	token := firmarToken(t, priv, "testkid", claimsValidos("admin@test.com"))

	email, err := VerifyIDToken(token)
	if err != nil {
		t.Fatalf("se esperaba token válido, error: %v", err)
	}
	if email != "admin@test.com" {
		t.Errorf("email = %q; se esperaba %q", email, "admin@test.com")
	}
}

func TestVerifyIDToken_AllowlistCaseInsensitive(t *testing.T) {
	priv := setup(t)
	// Email en mayúsculas: la allowlist debe compararse en minúsculas.
	token := firmarToken(t, priv, "testkid", claimsValidos("ADMIN@test.com"))

	email, err := VerifyIDToken(token)
	if err != nil {
		t.Fatalf("se esperaba autorizado (case-insensitive), error: %v", err)
	}
	if email != "ADMIN@test.com" {
		t.Errorf("email = %q; se esperaba %q (se preserva el original)", email, "ADMIN@test.com")
	}
}

func TestVerifyIDToken_EmailNoAutorizado(t *testing.T) {
	priv := setup(t)
	token := firmarToken(t, priv, "testkid", claimsValidos("otro@test.com"))

	_, err := VerifyIDToken(token)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("err = %v; se esperaba ErrForbidden", err)
	}
}

func TestVerifyIDToken_AudienciaIncorrecta(t *testing.T) {
	priv := setup(t)
	claims := claimsValidos("admin@test.com")
	claims["aud"] = "otro-proyecto"
	token := firmarToken(t, priv, "testkid", claims)

	_, err := VerifyIDToken(token)
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("err = %v; se esperaba ErrInvalidToken", err)
	}
}

func TestVerifyIDToken_IssuerIncorrecto(t *testing.T) {
	priv := setup(t)
	claims := claimsValidos("admin@test.com")
	claims["iss"] = "https://securetoken.google.com/otro-proyecto"
	token := firmarToken(t, priv, "testkid", claims)

	_, err := VerifyIDToken(token)
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("err = %v; se esperaba ErrInvalidToken", err)
	}
}

func TestVerifyIDToken_Expirado(t *testing.T) {
	priv := setup(t)
	claims := claimsValidos("admin@test.com")
	claims["exp"] = time.Now().Add(-time.Hour).Unix()
	token := firmarToken(t, priv, "testkid", claims)

	_, err := VerifyIDToken(token)
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("err = %v; se esperaba ErrInvalidToken", err)
	}
}

func TestVerifyIDToken_KidDesconocido(t *testing.T) {
	priv := setup(t)
	// kid no presente en la cache -> se intenta refrescar, pero la red está
	// deshabilitada, por lo que el resultado es un token inválido.
	token := firmarToken(t, priv, "kid-inexistente", claimsValidos("admin@test.com"))

	_, err := VerifyIDToken(token)
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("err = %v; se esperaba ErrInvalidToken", err)
	}
}

func TestParseMaxAge(t *testing.T) {
	casos := []struct {
		nombre   string
		header   string
		esperado time.Duration
	}{
		{"con_max_age", "public, max-age=3600, must-revalidate", time.Hour},
		{"solo_max_age", "max-age=120", 2 * time.Minute},
		{"sin_max_age", "no-cache", defaultCacheTTL},
		{"vacio", "", defaultCacheTTL},
		{"max_age_invalido", "max-age=abc", defaultCacheTTL},
		{"max_age_cero", "max-age=0", defaultCacheTTL},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := parseMaxAge(c.header); got != c.esperado {
				t.Errorf("parseMaxAge(%q) = %v; se esperaba %v", c.header, got, c.esperado)
			}
		})
	}
}
