// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flamego/flamego"
)

func TestSafeImageURLDataImageRendersWithoutTemplateSentinel(t *testing.T) {
	t.Parallel()

	photo := "data:image/png;base64,aGVsbG8="

	tpl, err := template.New("photo").Funcs(template.FuncMap{
		"safeImageURL": safeImageURL,
	}).Parse(`<img src="{{ safeImageURL .Photo }}">`)
	if err != nil {
		t.Fatalf("failed to parse template: %v", err)
	}

	var rendered strings.Builder

	if err := tpl.Execute(&rendered, map[string]*string{"Photo": &photo}); err != nil {
		t.Fatalf("failed to execute template: %v", err)
	}

	out := rendered.String()
	if strings.Contains(out, "#ZgotmplZ") {
		t.Fatalf("expected rendered html without template sentinel, got %q", out)
	}

	if !strings.Contains(out, `src="data:image/png;base64,aGVsbG8="`) {
		t.Fatalf("expected rendered html to contain data image URL, got %q", out)
	}
}

func TestSafeImageURLRejectsUnsafeScheme(t *testing.T) {
	t.Parallel()

	photo := "javascript:alert(1)"
	if got := safeImageURL(&photo); got != "" {
		t.Fatalf("expected unsafe image URL to be rejected, got %q", got)
	}
}

func TestSafeImageURLRejectsUnsupportedDataImageType(t *testing.T) {
	t.Parallel()

	photo := "data:image/svg+xml;base64,PHN2Zz48L3N2Zz4="
	if got := safeImageURL(&photo); got != "" {
		t.Fatalf("expected unsupported data image URL to be rejected, got %q", got)
	}
}

func TestConfigureEmptyNotFoundHandlerReturnsStatusOnly(t *testing.T) {
	t.Parallel()

	f := flamego.New()
	configureEmptyNotFoundHandler(f)

	req := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)
	rec := httptest.NewRecorder()
	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}

	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty 404 body, got %q", rec.Body.String())
	}
}
