package translate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const (
	// libreMaxTexts and libreMaxBytes keep one request modest: a server limits
	// the characters it takes in one call, and the limit is the operator's.
	libreMaxTexts = 50
	libreMaxBytes = 16 << 10
)

// LibreTranslate translates with a LibreTranslate server
// (https://libretranslate.com), one the application hosts or one that sells
// access. It implements [Translator] and is safe for concurrent use.
type LibreTranslate struct {
	transport *transport
	apiKey    string
}

var _ Translator = (*LibreTranslate)(nil)

// NewLibreTranslate builds the driver. It makes no request.
func NewLibreTranslate(c LibreConfig) (*LibreTranslate, error) {
	if strings.TrimSpace(c.BaseURL) == "" {
		return nil, errors.New("translate: LibreConfig.BaseURL is empty")
	}
	key := strings.TrimSpace(c.APIKey)
	t, err := newTransport("libretranslate", c.BaseURL, c.HTTPClient, c.Timeout, key)
	if err != nil {
		return nil, err
	}
	return &LibreTranslate{transport: t, apiKey: key}, nil
}

// Name is "libretranslate".
func (l *LibreTranslate) Name() string { return "libretranslate" }

// Translate translates the texts of r. Long lists are split over several calls
// and answered in order. An empty text is answered with itself.
func (l *LibreTranslate) Translate(ctx context.Context, r Request) (Result, error) {
	if err := validate(r); err != nil {
		return Result{}, err
	}
	target, err := libreCode(r.Target)
	if err != nil {
		return Result{}, err
	}
	source := "auto"
	if strings.TrimSpace(r.Source) != "" {
		if source, err = libreCode(r.Source); err != nil {
			return Result{}, err
		}
	}
	format := r.Format
	if format == "" {
		format = FormatText
	}

	type payload struct {
		Q      []string `json:"q"`
		Source string   `json:"source"`
		Target string   `json:"target"`
		Format string   `json:"format"`
		APIKey string   `json:"api_key,omitempty"`
	}
	texts, detected, err := translateAll(r.Text, libreMaxTexts, libreMaxBytes, func(batch []string) ([]string, string, error) {
		var answer struct {
			Translated json.RawMessage `json:"translatedText"`
			Detected   json.RawMessage `json:"detectedLanguage"`
		}
		body := payload{Q: batch, Source: source, Target: target, Format: format, APIKey: l.apiKey}
		if err := l.transport.do(ctx, http.MethodPost, "/translate", nil, nil, body, &answer, libreRefusal); err != nil {
			return nil, "", err
		}
		translated, err := oneOrMany(answer.Translated)
		if err != nil {
			return nil, "", fmt.Errorf("translate: libretranslate: translatedText is neither text nor a list: %w", err)
		}
		return translated, libreDetected(answer.Detected), nil
	})
	if err != nil {
		return Result{}, err
	}
	return Result{Text: texts, DetectedSource: detected, Provider: l.Name()}, nil
}

// Languages lists the languages the server translates into, as BCP 47 tags.
func (l *LibreTranslate) Languages(ctx context.Context) ([]string, error) {
	var answer []struct {
		Code    string   `json:"code"`
		Targets []string `json:"targets"`
	}
	if err := l.transport.do(ctx, http.MethodGet, "/languages", nil, nil, nil, &answer, libreRefusal); err != nil {
		return nil, err
	}
	var out []string
	for _, row := range answer {
		if len(row.Targets) == 0 {
			out = append(out, fromLibre(row.Code))
		}
		for _, target := range row.Targets {
			out = append(out, fromLibre(target))
		}
	}
	return dedupe(out), nil
}

// libreRefusal reads LibreTranslate's refusals: 403 is the key, 429 the quota
// or the rate, and a 400 that says a language is not supported or the key is
// invalid.
func libreRefusal(status int, message string) error {
	lowered := strings.ToLower(message)
	switch status {
	case http.StatusForbidden:
		return fmt.Errorf("%w (HTTP 403)", ErrUnauthorized)
	case http.StatusTooManyRequests:
		return fmt.Errorf("%w (HTTP 429)", ErrQuotaExceeded)
	case http.StatusBadRequest:
		switch {
		case strings.Contains(lowered, "not supported"):
			return fmt.Errorf("%w: %s", ErrUnsupportedLanguage, message)
		case strings.Contains(lowered, "api key"):
			return fmt.Errorf("%w (HTTP 400)", ErrUnauthorized)
		}
	}
	return fmt.Errorf("translate: libretranslate answered HTTP %d: %s", status, message)
}

// libreCode is the code LibreTranslate uses for a BCP 47 tag. It takes the
// language alone ("pt-BR" is "pt"), and the two Chinese scripts as "zh" and
// "zt".
func libreCode(raw string) (string, error) {
	t, ok := parseTag(raw)
	if !ok {
		return "", fmt.Errorf("%w: %q is not a BCP 47 tag", ErrUnsupportedLanguage, raw)
	}
	switch t.lang {
	case "zh":
		if t.traditional() {
			return "zt", nil
		}
	case "no", "nn":
		return "nb", nil
	case "iw":
		return "he", nil
	}
	return t.lang, nil
}

// fromLibre is a LibreTranslate code as a BCP 47 tag.
func fromLibre(code string) string {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "zh":
		return "zh-Hans"
	case "zt":
		return "zh-Hant"
	}
	if t, ok := parseTag(code); ok {
		return t.String()
	}
	return strings.TrimSpace(code)
}

// oneOrMany reads a JSON string or a list of strings as a list.
func oneOrMany(raw json.RawMessage) ([]string, error) {
	var many []string
	if err := json.Unmarshal(raw, &many); err == nil {
		return many, nil
	}
	var one string
	if err := json.Unmarshal(raw, &one); err != nil {
		return nil, err
	}
	return []string{one}, nil
}

// libreDetected reads the detected language, which is an object for one text
// and a list of them for several.
func libreDetected(raw json.RawMessage) string {
	var one struct {
		Language string `json:"language"`
	}
	if json.Unmarshal(raw, &one) == nil && one.Language != "" {
		return fromLibre(one.Language)
	}
	var many []struct {
		Language string `json:"language"`
	}
	if json.Unmarshal(raw, &many) == nil && len(many) > 0 {
		return fromLibre(many[0].Language)
	}
	return ""
}
