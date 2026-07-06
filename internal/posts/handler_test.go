package posts

import (
	"strings"
	"testing"
)

// TestSlugify verifica la generación de slugs: transliteración de acentos y ñ a
// ASCII, colapso de espacios y guiones múltiples, recorte de guiones y minúsculas.
func TestSlugify(t *testing.T) {
	casos := []struct {
		nombre   string
		entrada  string
		esperado string
	}{
		{"simple", "Hola Mundo", "hola-mundo"},
		{"acentos", "Canción de Otoño", "cancion-de-otono"},
		{"enie", "El Niño", "el-nino"},
		{"mayus_con_enie", "Título con Ñ", "titulo-con-n"},
		{"espacios_multiples", "  espacios   multiples  ", "espacios-multiples"},
		{"guiones_extremos", "---guiones---", "guiones"},
		{"simbolos_descartados", "Café & Té", "cafe-te"},
		{"solo_simbolos", "!!!", ""},
		{"numeros", "Post 123 v2", "post-123-v2"},
		{"guiones_ya_presentes", "uno - dos", "uno-dos"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			got := slugify(c.entrada)
			if got != c.esperado {
				t.Errorf("slugify(%q) = %q; se esperaba %q", c.entrada, got, c.esperado)
			}
		})
	}
}

// TestWordCount verifica el conteo de palabras separando por cualquier espacio.
func TestWordCount(t *testing.T) {
	casos := []struct {
		nombre   string
		entrada  string
		esperado int
	}{
		{"vacio", "", 0},
		{"solo_espacios", "     ", 0},
		{"dos_palabras", "hola mundo", 2},
		{"espacios_multiples", "  a   b  ", 2},
		{"tabs_y_saltos", "uno\ndos\ttres", 3},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := wordCount(c.entrada); got != c.esperado {
				t.Errorf("wordCount(%q) = %d; se esperaba %d", c.entrada, got, c.esperado)
			}
		})
	}
}

// TestCalcReadingTime verifica el redondeo hacia arriba (ceil) sobre 200 ppm.
func TestCalcReadingTime(t *testing.T) {
	casos := []struct {
		nombre   string
		palabras int
		esperado int
	}{
		{"cero", 0, 0},
		{"una_palabra", 1, 1},
		{"justo_200", 200, 1},
		{"201_redondea_a_2", 201, 2},
		{"400", 400, 2},
		{"401_redondea_a_3", 401, 3},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			// Construimos un contenido con exactamente c.palabras palabras.
			contenido := strings.TrimSpace(strings.Repeat("palabra ", c.palabras))
			if got := calcReadingTime(contenido); got != c.esperado {
				t.Errorf("calcReadingTime(%d palabras) = %d; se esperaba %d", c.palabras, got, c.esperado)
			}
		})
	}
}

// TestValidStatus verifica que solo draft y published sean estados válidos.
func TestValidStatus(t *testing.T) {
	validos := []string{"draft", "published"}
	for _, s := range validos {
		if !validStatus(s) {
			t.Errorf("validStatus(%q) = false; se esperaba true", s)
		}
	}
	invalidos := []string{"", "archived", "Draft", "PUBLISHED", "pending", " draft "}
	for _, s := range invalidos {
		if validStatus(s) {
			t.Errorf("validStatus(%q) = true; se esperaba false", s)
		}
	}
}
