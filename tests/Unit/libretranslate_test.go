package unit_test

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"

	translate "github.com/hyz-is/arandu-translate"
)

// echoLibre answers a list of texts as "T:<text>", the way a server answers a
// request whose q is a list, with a detection for the whole call.
func echoLibre(s seen, w http.ResponseWriter) {
	out := []string{}
	for _, text := range s.texts("q") {
		out = append(out, "T:"+text)
	}
	reply(w, 200, mustJSON(map[string]any{
		"translatedText":   out,
		"detectedLanguage": map[string]any{"confidence": 90, "language": "fr"},
	}))
}

func newLibre(t *testing.T, answer func(s seen, w http.ResponseWriter), mutate ...func(*translate.LibreConfig)) (*translate.LibreTranslate, *server) {
	t.Helper()
	srv := newServer(t, answer)
	cfg := translate.LibreConfig{BaseURL: srv.URL}
	for _, m := range mutate {
		m(&cfg)
	}
	l, err := translate.NewLibreTranslate(cfg)
	if err != nil {
		t.Fatalf("NewLibreTranslate: %v", err)
	}
	return l, srv
}

func TestLibreTranslateSendsTheRequestTheServerDocuments(t *testing.T) {
	l, srv := newLibre(t, echoLibre, func(c *translate.LibreConfig) { c.APIKey = "libre-secret" })

	got, err := l.Translate(context.Background(), translate.Request{Text: []string{"bonjour", "merci"}, Source: "fr", Target: "pt-BR", Format: "html"})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if want := []string{"T:bonjour", "T:merci"}; !reflect.DeepEqual(got.Text, want) {
		t.Errorf("Text = %q, want %q", got.Text, want)
	}
	if got.Provider != "libretranslate" || l.Name() != "libretranslate" {
		t.Errorf("Provider = %q, Name = %q", got.Provider, l.Name())
	}
	if got.DetectedSource != "fr" {
		t.Errorf("DetectedSource = %q, want fr", got.DetectedSource)
	}
	req := srv.last(t)
	if req.method != http.MethodPost || req.path != "/translate" {
		t.Errorf("request = %s %s, want POST /translate", req.method, req.path)
	}
	for key, want := range map[string]string{"source": "fr", "target": "pt", "format": "html", "api_key": "libre-secret"} {
		if got := req.str(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestLibreTranslateDetectsAndDefaultsToText(t *testing.T) {
	l, srv := newLibre(t, echoLibre)
	if _, err := l.Translate(context.Background(), translate.Request{Text: []string{"x"}, Target: "en"}); err != nil {
		t.Fatalf("Translate: %v", err)
	}
	req := srv.last(t)
	if req.str("source") != "auto" || req.str("format") != "text" {
		t.Errorf("source = %q, format = %q, want auto and text", req.str("source"), req.str("format"))
	}
	if _, present := req.body["api_key"]; present {
		t.Error("api_key is sent without a key")
	}
}

func TestLibreTranslateMapsBCP47ToItsCodes(t *testing.T) {
	for target, want := range map[string]string{
		"pt-BR": "pt", "en-GB": "en", "zh-Hans": "zh", "zh": "zh", "zh-Hant": "zt", "zh-TW": "zt", "ar": "ar", "ja": "ja", "no": "nb",
	} {
		l, srv := newLibre(t, echoLibre)
		if _, err := l.Translate(context.Background(), translate.Request{Text: []string{"x"}, Target: target}); err != nil {
			t.Fatalf("Translate: %v", err)
		}
		if got := srv.last(t).str("target"); got != want {
			t.Errorf("target %q is sent as %q, want %q", target, got, want)
		}
	}
}

func TestLibreTranslateReadsEveryShapeOfAnswer(t *testing.T) {
	for name, body := range map[string]string{
		"a text and an object": `{"translatedText":"T:x","detectedLanguage":{"confidence":50,"language":"es"}}`,
		"a list and a list":    `{"translatedText":["T:x"],"detectedLanguage":[{"confidence":50,"language":"es"}]}`,
		"no detection":         `{"translatedText":["T:x"]}`,
	} {
		l, _ := newLibre(t, func(_ seen, w http.ResponseWriter) { reply(w, 200, body) })
		got, err := l.Translate(context.Background(), translate.Request{Text: []string{"x"}, Target: "en"})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !reflect.DeepEqual(got.Text, []string{"T:x"}) {
			t.Errorf("%s: Text = %q", name, got.Text)
		}
		wantSource := "es"
		if name == "no detection" {
			wantSource = ""
		}
		if got.DetectedSource != wantSource {
			t.Errorf("%s: DetectedSource = %q, want %q", name, got.DetectedSource, wantSource)
		}
	}
}

func TestLibreTranslateSplitsLongListsAndSkipsEmptyTexts(t *testing.T) {
	l, srv := newLibre(t, echoLibre)
	texts := make([]string, 120)
	for i := range texts {
		texts[i] = "t" + string(rune('A'+i%26))
	}
	texts[5] = ""
	got, err := l.Translate(context.Background(), translate.Request{Text: texts, Target: "de"})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if n := len(srv.all()); n != 3 {
		t.Errorf("requests = %d, want 3", n)
	}
	for i, text := range got.Text {
		want := "T:" + texts[i]
		if texts[i] == "" {
			want = ""
		}
		if text != want {
			t.Fatalf("Text[%d] = %q, want %q", i, text, want)
		}
	}
}

func TestLibreTranslateReadsItsErrors(t *testing.T) {
	for _, c := range []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"key", 403, `{"error":"Invalid API key"}`, translate.ErrUnauthorized},
		{"key as 400", 400, `{"error":"Invalid API key"}`, translate.ErrUnauthorized},
		{"quota", 429, `{"error":"Slowdown: 100 per 1 minute"}`, translate.ErrQuotaExceeded},
		{"language", 400, `{"error":"zz is not supported"}`, translate.ErrUnsupportedLanguage},
	} {
		t.Run(c.name, func(t *testing.T) {
			l, _ := newLibre(t, func(_ seen, w http.ResponseWriter) { reply(w, c.status, c.body) })
			if _, err := l.Translate(context.Background(), translate.Request{Text: []string{"x"}, Target: "zz"}); !errors.Is(err, c.want) {
				t.Errorf("error = %v, want %v", err, c.want)
			}
			if _, err := l.Languages(context.Background()); !errors.Is(err, c.want) {
				t.Errorf("Languages error = %v, want %v", err, c.want)
			}
		})
	}

	l, _ := newLibre(t, func(_ seen, w http.ResponseWriter) { reply(w, 400, `{"error":"Invalid request: missing q parameter"}`) })
	if _, err := l.Translate(context.Background(), translate.Request{Text: []string{"x"}, Target: "en"}); err == nil ||
		errors.Is(err, translate.ErrUnsupportedLanguage) || errors.Is(err, translate.ErrUnauthorized) {
		t.Errorf("an unrelated 400 = %v, want a plain error", err)
	}
}

func TestLibreTranslateListsItsTargetsAsBCP47(t *testing.T) {
	l, srv := newLibre(t, func(_ seen, w http.ResponseWriter) {
		reply(w, 200, `[{"code":"en","name":"English","targets":["ar","en","pt","zh","zt"]},
			{"code":"fr","name":"French","targets":["ar","en","ja"]},
			{"code":"xx","name":"Only a code"}]`)
	})
	got, err := l.Languages(context.Background())
	if err != nil {
		t.Fatalf("Languages: %v", err)
	}
	want := []string{"ar", "en", "pt", "zh-Hans", "zh-Hant", "ja", "xx"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Languages = %q, want %q", got, want)
	}
	if req := srv.last(t); req.method != http.MethodGet || req.path != "/languages" {
		t.Errorf("request = %s %s, want GET /languages", req.method, req.path)
	}
}

func TestLibreTranslateConfigurationIsChecked(t *testing.T) {
	for name, cfg := range map[string]translate.LibreConfig{
		"no address":  {},
		"bad address": {BaseURL: "localhost:5000"},
		"bad timeout": {BaseURL: "http://localhost:5000", Timeout: -1},
	} {
		if _, err := translate.NewLibreTranslate(cfg); err == nil {
			t.Errorf("%s: NewLibreTranslate accepted it", name)
		}
	}
}
