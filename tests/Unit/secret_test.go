package unit_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	translate "github.com/hyz-is/arandu-translate"
)

const secretKey = "very-secret-key-123"

// TestTheKeyIsInNoFormattedConfiguration holds what a log line can print.
func TestTheKeyIsInNoFormattedConfiguration(t *testing.T) {
	deepl := translate.DeepLConfig{APIKey: secretKey, BaseURL: "https://x.example"}
	libre := translate.LibreConfig{APIKey: secretKey, BaseURL: "https://y.example"}
	for _, c := range []any{deepl, &deepl, libre, &libre} {
		for _, text := range []string{fmt.Sprint(c), fmt.Sprintf("%v", c), fmt.Sprintf("%+v", c), fmt.Sprintf("%#v", c), fmt.Sprintf("%s", c)} {
			if strings.Contains(text, secretKey) {
				t.Errorf("%T prints its key: %s", c, text)
			}
		}
	}
	if s := deepl.String(); !strings.Contains(s, "[redacted]") || !strings.Contains(s, "x.example") {
		t.Errorf("DeepLConfig.String = %s, want the address and a redaction", s)
	}
	if s := (translate.LibreConfig{}).String(); strings.Contains(s, "[redacted]") {
		t.Errorf("an empty key is shown as a redaction: %s", s)
	}
}

// TestTheKeyIsInNoError holds what an error carries, including a provider that
// repeats the key back, and a connection that fails.
func TestTheKeyIsInNoError(t *testing.T) {
	echoKey := func(s seen, w http.ResponseWriter) {
		reply(w, 500, `{"message":"bad call from `+secretKey+`","error":"`+secretKey+`"}`)
	}
	d, _ := newDeepL(t, echoKey, func(c *translate.DeepLConfig) { c.APIKey = secretKey })
	l, _ := newLibre(t, echoKey, func(c *translate.LibreConfig) { c.APIKey = secretKey })

	request := translate.Request{Text: []string{"x"}, Target: "de"}
	for name, call := range map[string]func() error{
		"deepl translate": func() error { _, err := d.Translate(context.Background(), request); return err },
		"deepl languages": func() error { _, err := d.Languages(context.Background()); return err },
		"libre translate": func() error { _, err := l.Translate(context.Background(), request); return err },
		"libre languages": func() error { _, err := l.Languages(context.Background()); return err },
	} {
		err := call()
		if err == nil {
			t.Errorf("%s: no error", name)
			continue
		}
		if strings.Contains(err.Error(), secretKey) || strings.Contains(fmt.Sprintf("%+v", err), secretKey) {
			t.Errorf("%s: the error carries the key: %v", name, err)
		}
	}

	// A connection that fails: the server is gone.
	dead := httptest.NewServer(http.NotFoundHandler())
	url := dead.URL
	dead.Close()
	for name, driver := range map[string]translate.Translator{
		"deepl": mustDeepL(t, translate.DeepLConfig{APIKey: secretKey, BaseURL: url}),
		"libre": mustLibre(t, translate.LibreConfig{APIKey: secretKey, BaseURL: url}),
	} {
		_, err := driver.Translate(context.Background(), request)
		if err == nil || strings.Contains(err.Error(), secretKey) {
			t.Errorf("%s: a failed connection gave %v", name, err)
		}
	}
}

// TestAnErrorKeepsItsCauseWhenTheKeyIsScrubbed holds errors.Is working on a
// message the package had to clean.
func TestAnErrorKeepsItsCauseWhenTheKeyIsScrubbed(t *testing.T) {
	d, _ := newDeepL(t, func(_ seen, w http.ResponseWriter) {
		reply(w, 400, `{"message":"Value for 'target_lang' not supported, `+secretKey+`"}`)
	}, func(c *translate.DeepLConfig) { c.APIKey = secretKey })
	_, err := d.Translate(context.Background(), translate.Request{Text: []string{"x"}, Target: "de"})
	if !errors.Is(err, translate.ErrUnsupportedLanguage) || strings.Contains(err.Error(), secretKey) {
		t.Errorf("error = %v, want ErrUnsupportedLanguage without the key", err)
	}
}

func mustDeepL(t *testing.T, c translate.DeepLConfig) translate.Translator {
	t.Helper()
	d, err := translate.NewDeepL(c)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func mustLibre(t *testing.T, c translate.LibreConfig) translate.Translator {
	t.Helper()
	l, err := translate.NewLibreTranslate(c)
	if err != nil {
		t.Fatal(err)
	}
	return l
}
