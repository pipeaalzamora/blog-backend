// Package firebaseauth verifica ID tokens de Firebase Authentication usando
// las claves públicas de Google (JWKS/X.509), sin depender del Admin SDK ni de
// una service account. La autorización se basa en una allowlist de emails.
package firebaseauth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// googleCertsURL expone los certificados X.509 públicos (mapa kid -> PEM) que
// Firebase usa para firmar los ID tokens.
const googleCertsURL = "https://www.googleapis.com/robot/v1/metadata/x509/securetoken@system.gserviceaccount.com"

// defaultCacheTTL se usa como respaldo si el header Cache-Control no trae max-age.
const defaultCacheTTL = time.Hour

var (
	// ErrInvalidToken indica que la firma o los claims del token no son válidos.
	// El middleware debe responder 401.
	ErrInvalidToken = errors.New("invalid token")
	// ErrForbidden indica que el token es válido pero el email no está autorizado.
	// El middleware debe responder 403.
	ErrForbidden = errors.New("forbidden")
)

var (
	projectID   string
	adminEmails map[string]struct{}

	// Cache en memoria de las claves públicas de Google, protegida por RWMutex.
	keysMu     sync.RWMutex
	keysCache  map[string]*rsa.PublicKey
	keysExpiry time.Time

	httpClient = &http.Client{Timeout: 10 * time.Second}
)

// Init configura el verificador con el ID de proyecto de Firebase y la
// allowlist de emails administradores (normalizados a minúsculas).
func Init(pid string, emails []string) {
	projectID = strings.TrimSpace(pid)
	adminEmails = make(map[string]struct{}, len(emails))
	for _, e := range emails {
		e = strings.ToLower(strings.TrimSpace(e))
		if e != "" {
			adminEmails[e] = struct{}{}
		}
	}
}

// VerifyIDToken verifica la firma RS256 y los claims de un ID token de Firebase.
// Devuelve el email si el token es válido y el email está autorizado.
// Retorna ErrInvalidToken (401) o ErrForbidden (403) según el caso.
func VerifyIDToken(tokenStr string) (email string, err error) {
	claims := jwt.MapClaims{}
	token, parseErr := jwt.ParseWithClaims(tokenStr, claims, keyFunc,
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer("https://securetoken.google.com/"+projectID),
		jwt.WithAudience(projectID),
	)
	if parseErr != nil || !token.Valid {
		return "", ErrInvalidToken
	}

	// sub (subject) no debe estar vacío.
	if sub, _ := claims["sub"].(string); strings.TrimSpace(sub) == "" {
		return "", ErrInvalidToken
	}

	// Autorización por allowlist de emails (case-insensitive).
	email, _ = claims["email"].(string)
	email = strings.TrimSpace(email)
	if email == "" {
		return "", ErrForbidden
	}
	if _, ok := adminEmails[strings.ToLower(email)]; !ok {
		return "", ErrForbidden
	}
	return email, nil
}

// keyFunc resuelve la clave pública RSA correspondiente al kid del token.
func keyFunc(t *jwt.Token) (interface{}, error) {
	kid, ok := t.Header["kid"].(string)
	if !ok || kid == "" {
		return nil, errors.New("missing kid")
	}
	return getKey(kid)
}

// getKey devuelve la clave pública del kid indicado, refrescando la cache si
// está expirada o si el kid no se encuentra.
func getKey(kid string) (*rsa.PublicKey, error) {
	keysMu.RLock()
	if time.Now().Before(keysExpiry) {
		if key, ok := keysCache[kid]; ok {
			keysMu.RUnlock()
			return key, nil
		}
	}
	keysMu.RUnlock()

	if err := refreshKeys(); err != nil {
		return nil, err
	}

	keysMu.RLock()
	defer keysMu.RUnlock()
	key, ok := keysCache[kid]
	if !ok {
		return nil, errors.New("unknown key id")
	}
	return key, nil
}

// refreshKeys descarga y parsea las claves públicas de Google, y calcula su
// expiración según el max-age del header Cache-Control.
func refreshKeys() error {
	resp, err := httpClient.Get(googleCertsURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("failed to fetch public keys")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var certs map[string]string
	if err := json.Unmarshal(body, &certs); err != nil {
		return err
	}
	parsed := make(map[string]*rsa.PublicKey, len(certs))
	for kid, certPEM := range certs {
		pub, err := parsePublicKey(certPEM)
		if err != nil {
			continue // ignorar certificados que no se puedan parsear
		}
		parsed[kid] = pub
	}
	if len(parsed) == 0 {
		return errors.New("no valid public keys")
	}

	ttl := parseMaxAge(resp.Header.Get("Cache-Control"))
	keysMu.Lock()
	keysCache = parsed
	keysExpiry = time.Now().Add(ttl)
	keysMu.Unlock()
	return nil
}

// parsePublicKey extrae la clave pública RSA de un certificado X.509 en PEM.
func parsePublicKey(certPEM string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return nil, errors.New("invalid PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	pub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("not an RSA public key")
	}
	return pub, nil
}

// parseMaxAge extrae la directiva max-age (en segundos) del header Cache-Control.
// Si no está presente o es inválida, devuelve defaultCacheTTL.
func parseMaxAge(cacheControl string) time.Duration {
	for _, part := range strings.Split(cacheControl, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "max-age=") {
			secs, err := strconv.Atoi(strings.TrimPrefix(part, "max-age="))
			if err == nil && secs > 0 {
				return time.Duration(secs) * time.Second
			}
		}
	}
	return defaultCacheTTL
}
