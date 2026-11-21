package unit_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	translate "github.com/hyz-is/arandu-translate"
)

// echoDeepL answers every text as "T:<text>" with a detected source of EN.
func echoDeepL(s seen, w http.ResponseWriter) {
	type row struct {
		Text   string `json:"text"`
		Source string `json:"detected_source_language"`
	}
	rows := []row{}
	for _, text := range s.texts("text") {
		rows = append(rows, row{Text: "T:" + text, Source: "EN"})
	}
	reply(w, 200, mustJSON(map[string]any{"translations": rows}))
}

func newDeepL(t *testing.T, answer func(s seen, w http.ResponseWriter), mutate ...func(*translate.DeepLConfig)) (*translate.DeepL, *server) {
	t.Helper()
	srv := newServer(t, answer)
	cfg := translate.DeepLConfig{APIKey: "secret-key:fx", BaseURL: srv.URL}
	for _, m := range mutate {
		m(&cfg)
	}
	d, err := translate.NewDeepL(cfg)
	if err != nil {
		t.Fatalf("NewDeepL: %v", err)
	}
	return d, srv
}

func TestDeepLSendsTheRequestTheServiceDocuments(t *testing.T) {
	d, srv := newDeepL(t, echoDeepL)

	got, err := d.Translate(context.Background(), translate.Request{Text: []string{"bonjour", "merci"}, Source: "fr", Target: "pt-BR"})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if want := []string{"T:bonjour", "T:merci"}; !reflect.DeepEqual(got.Text, want) {
		t.Errorf("Text = %q, want %q", got.Text, want)
	}
	if got.Provider != "deepl" || d.Name() != "deepl" {
		t.Errorf("Provider = %q, Name = %q, want deepl", got.Provider, d.Name())
	}
	if got.DetectedSource != "en" {
		t.Errorf("DetectedSource = %q, want en", got.DetectedSource)
	}

	req := srv.last(t)
	if req.method != http.MethodPost || req.path != "/v2/translate" {
		t.Errorf("request = %s %s, want POST /v2/translate", req.method, req.path)
	}
	if h := req.header.Get("Authorization"); h != "DeepL-Auth-Key secret-key:fx" {
		t.Errorf("Authorization = %q", h)
	}
	if ct := req.header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	if req.str("target_lang") != "PT-BR" || req.str("source_lang") != "FR" {
		t.Errorf("target_lang = %q, source_lang = %q, want PT-BR and FR", req.str("target_lang"), req.str("source_lang"))
	}
	if _, present := req.body["tag_handling"]; present {
		t.Error("tag_handling is sent for plain text")
	}
}

func TestDeepLOmitsTheSourceToDetectAndHandlesHTML(t *testing.T) {
	d, srv := newDeepL(t, echoDeepL)
	if _, err := d.Translate(context.Background(), translate.Request{Text: []string{"<b>oi</b>"}, Target: "en", Format: "html"}); err != nil {
		t.Fatalf("Translate: %v", err)
	}
	req := srv.last(t)
	if _, present := req.body["source_lang"]; present {
		t.Error("source_lang is sent when the source is empty")
	}
	if req.str("tag_handling") != "html" {
		t.Errorf("tag_handling = %q, want html", req.str("tag_handling"))
	}
}

func TestDeepLMapsBCP47ToItsCodes(t *testing.T) {
	for target, want := range map[string]string{
		"pt-BR":   "PT-BR",
		"pt_br":   "PT-BR",
		"pt-PT":   "PT-PT",
		"pt":      "PT-PT",
		"en":      "EN-GB",
		"en-GB":   "EN-GB",
		"en-US":   "EN-US",
		"zh-Hans": "ZH-HANS",
		"zh":      "ZH-HANS",
		"zh-CN":   "ZH-HANS",
		"zh-Hant": "ZH-HANT",
		"zh-TW":   "ZH-HANT",
		"ar":      "AR",
		"ja":      "JA",
		"ru":      "RU",
		"de":      "DE",
		"it":      "IT",
		"es":      "ES",
		"es-ES":   "ES",
		"es-419":  "ES-419",
		"fr":      "FR",
		"no":      "NB",
	} {
		t.Run(target, func(t *testing.T) {
			d, srv := newDeepL(t, echoDeepL)
			if _, err := d.Translate(context.Background(), translate.Request{Text: []string{"x"}, Target: target}); err != nil {
				t.Fatalf("Translate: %v", err)
			}
			if got := srv.last(t).str("target_lang"); got != want {
				t.Errorf("target %q is sent as %q, want %q", target, got, want)
			}
		})
	}
}

func TestDeepLSourceHasNoVariant(t *testing.T) {
	for source, want := range map[string]string{"pt-BR": "PT", "en-US": "EN", "en": "EN", "zh-Hans": "ZH", "fr": "FR"} {
		d, srv := newDeepL(t, echoDeepL)
		if _, err := d.Translate(context.Background(), translate.Request{Text: []string{"x"}, Source: source, Target: "de"}); err != nil {
			t.Fatalf("Translate: %v", err)
		}
		if got := srv.last(t).str("source_lang"); got != want {
			t.Errorf("source %q is sent as %q, want %q", source, got, want)
		}
	}
}

func TestDeepLVariantsAreConfigurable(t *testing.T) {
	d, srv := newDeepL(t, echoDeepL, func(c *translate.DeepLConfig) {
		c.EnglishTarget = "en-us"
		c.PortugueseTarget = "PT-BR"
	})
	for target, want := range map[string]string{"en": "EN-US", "pt": "PT-BR", "en-GB": "EN-GB", "pt-PT": "PT-PT"} {
		if _, err := d.Translate(context.Background(), translate.Request{Text: []string{"x"}, Target: target}); err != nil {
			t.Fatalf("Translate: %v", err)
		}
		if got := srv.last(t).str("target_lang"); got != want {
			t.Errorf("target %q is sent as %q, want %q", target, got, want)
		}
	}
	if _, err := translate.NewDeepL(translate.DeepLConfig{APIKey: "k", EnglishTarget: "EN-AU"}); err == nil {
		t.Error("NewDeepL accepted EnglishTarget EN-AU")
	}
}

func TestDeepLSplitsLargeBatchesAndKeepsTheOrder(t *testing.T) {
	d, srv := newDeepL(t, echoDeepL)
	texts := make([]string, 120)
	for i := range texts {
		texts[i] = fmt.Sprintf("text %d", i)
	}
	got, err := d.Translate(context.Background(), translate.Request{Text: texts, Target: "de"})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	requests := srv.all()
	if len(requests) != 3 {
		t.Fatalf("requests = %d, want 3 (50 + 50 + 20)", len(requests))
	}
	for i, r := range requests {
		if n := len(r.texts("text")); n > 50 {
			t.Errorf("request %d carried %d texts, DeepL takes 50", i, n)
		}
	}
	for i, text := range got.Text {
		if want := "T:" + texts[i]; text != want {
			t.Fatalf("Text[%d] = %q, want %q", i, text, want)
		}
	}
}

func TestDeepLSplitsBySizeToo(t *testing.T) {
	d, srv := newDeepL(t, echoDeepL)
	big := strings.Repeat("a", 40<<10)
	got, err := d.Translate(context.Background(), translate.Request{Text: []string{big, big, big}, Target: "de"})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if n := len(srv.all()); n != 3 {
		t.Errorf("requests = %d, want 3: two 40 KiB texts do not fit one 64 KiB request", n)
	}
	if len(got.Text) != 3 {
		t.Errorf("answers = %d, want 3", len(got.Text))
	}
}

func TestDeepLNeverSendsAnEmptyText(t *testing.T) {
	d, srv := newDeepL(t, echoDeepL)
	got, err := d.Translate(context.Background(), translate.Request{Text: []string{"", "oi", "  "}, Target: "en"})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if want := []string{"", "T:oi", "  "}; !reflect.DeepEqual(got.Text, want) {
		t.Errorf("Text = %q, want %q", got.Text, want)
	}
	if sent := srv.last(t).texts("text"); !reflect.DeepEqual(sent, []string{"oi"}) {
		t.Errorf("sent = %q, want only the text that has content", sent)
	}

	none, err := d.Translate(context.Background(), translate.Request{Text: nil, Target: "en"})
	if err != nil || len(none.Text) != 0 {
		t.Errorf("Translate with no text = %v, %v", none, err)
	}
	if n := len(srv.all()); n != 1 {
		t.Errorf("requests = %d, want 1", n)
	}
}

func TestDeepLRefusesABadRequestBeforeTheNetwork(t *testing.T) {
	d, srv := newDeepL(t, echoDeepL)
	for name, r := range map[string]translate.Request{
		"no target":    {Text: []string{"x"}},
		"bad format":   {Text: []string{"x"}, Target: "en", Format: "markdown"},
		"bad tag":      {Text: []string{"x"}, Target: "not a tag"},
		"bad source":   {Text: []string{"x"}, Target: "en", Source: "!!"},
		"too many sub": {Text: []string{"x"}, Target: "en-US-x-y"},
	} {
		if _, err := d.Translate(context.Background(), r); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if n := len(srv.all()); n != 0 {
		t.Errorf("requests = %d, want 0", n)
	}
	if _, err := d.Translate(context.Background(), translate.Request{Text: []string{"x"}, Target: "not a tag"}); !errors.Is(err, translate.ErrUnsupportedLanguage) {
		t.Errorf("a tag that is not BCP 47 = %v, want ErrUnsupportedLanguage", err)
	}
}

func TestDeepLReadsItsErrors(t *testing.T) {
	for _, c := range []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"key", 403, `{"message":"Forbidden"}`, translate.ErrUnauthorized},
		{"quota", 456, `{"message":"Quota exceeded. The character limit has been reached."}`, translate.ErrQuotaExceeded},
		{"rate", 429, `{"message":"Too many requests"}`, translate.ErrQuotaExceeded},
		{"rate 529", 529, `{"message":"busy"}`, translate.ErrQuotaExceeded},
		{"language", 400, `{"message":"Value for 'target_lang' not supported."}`, translate.ErrUnsupportedLanguage},
	} {
		t.Run(c.name, func(t *testing.T) {
			d, _ := newDeepL(t, func(_ seen, w http.ResponseWriter) { reply(w, c.status, c.body) })
			_, err := d.Translate(context.Background(), translate.Request{Text: []string{"x"}, Target: "de"})
			if !errors.Is(err, c.want) {
				t.Errorf("error = %v, want %v", err, c.want)
			}
			if _, err := d.Languages(context.Background()); !errors.Is(err, c.want) {
				t.Errorf("Languages error = %v, want %v", err, c.want)
			}
		})
	}

	d, _ := newDeepL(t, func(_ seen, w http.ResponseWriter) { reply(w, 500, `{"message":"boom"}`) })
	_, err := d.Translate(context.Background(), translate.Request{Text: []string{"x"}, Target: "de"})
	if err == nil || errors.Is(err, translate.ErrUnauthorized) || errors.Is(err, translate.ErrQuotaExceeded) || !strings.Contains(err.Error(), "500") {
		t.Errorf("HTTP 500 = %v, want a plain error naming the status", err)
	}

	d, _ = newDeepL(t, func(_ seen, w http.ResponseWriter) { reply(w, 400, `{"message":"Parameter 'text' not specified."}`) })
	if _, err := d.Translate(context.Background(), translate.Request{Text: []string{"x"}, Target: "de"}); errors.Is(err, translate.ErrUnsupportedLanguage) {
		t.Error("a 400 about something else reads as an unsupported language")
	}
}

func TestDeepLRefusesAnAnswerWithTheWrongCount(t *testing.T) {
	d, _ := newDeepL(t, func(_ seen, w http.ResponseWriter) { reply(w, 200, `{"translations":[]}`) })
	if _, err := d.Translate(context.Background(), translate.Request{Text: []string{"x"}, Target: "de"}); err == nil {
		t.Error("an answer with no translation was accepted")
	}
}

func TestDeepLListsItsTargetsAsBCP47(t *testing.T) {
	d, srv := newDeepL(t, func(_ seen, w http.ResponseWriter) {
		reply(w, 200, `[{"language":"AR","name":"Arabic"},{"language":"EN-GB","name":"English (British)"},
			{"language":"EN-US","name":"English (American)"},{"language":"PT-BR","name":"Portuguese (Brazilian)"},
			{"language":"ZH-HANS","name":"Chinese (simplified)"},{"language":"ZH-HANT","name":"Chinese (traditional)"},
			{"language":"ES-419","name":"Spanish (Latin American)"},{"language":"JA","name":"Japanese"}]`)
	})
	got, err := d.Languages(context.Background())
	if err != nil {
		t.Fatalf("Languages: %v", err)
	}
	want := []string{"ar", "en-GB", "en-US", "pt-BR", "zh-Hans", "zh-Hant", "es-419", "ja"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Languages = %q, want %q", got, want)
	}
	req := srv.last(t)
	if req.method != http.MethodGet || req.path != "/v2/languages" || req.query != "type=target" {
		t.Errorf("request = %s %s?%s, want GET /v2/languages?type=target", req.method, req.path, req.query)
	}
}

// roundTrip lets a test see the address a client would have called without
// calling it.
type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDeepLPicksItsAddressFromTheKey(t *testing.T) {
	for key, want := range map[string]string{
		"abc:fx": "api-free.deepl.com",
		"abc":    "api.deepl.com",
	} {
		var host string
		client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
			host = r.URL.Host
			return nil, errors.New("not sent")
		})}
		d, err := translate.NewDeepL(translate.DeepLConfig{APIKey: key, HTTPClient: client})
		if err != nil {
			t.Fatalf("NewDeepL: %v", err)
		}
		_, _ = d.Translate(context.Background(), translate.Request{Text: []string{"x"}, Target: "de"})
		if host != want {
			t.Errorf("key ending %q calls %q, want %q", key[len(key)-2:], host, want)
		}
	}
}

func TestDeepLConfigurationIsChecked(t *testing.T) {
	for name, cfg := range map[string]translate.DeepLConfig{
		"no key":         {},
		"blank key":      {APIKey: "  "},
		"bad address":    {APIKey: "k", BaseURL: "ftp://x"},
		"no host":        {APIKey: "k", BaseURL: "https://"},
		"bad timeout":    {APIKey: "k", Timeout: -1},
		"bad portuguese": {APIKey: "k", PortugueseTarget: "PT"},
	} {
		if _, err := translate.NewDeepL(cfg); err == nil {
			t.Errorf("%s: NewDeepL accepted it", name)
		}
	}
}
