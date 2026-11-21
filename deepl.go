package translate

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	// deeplMaxTexts is how many texts DeepL takes in one /v2/translate call.
	deeplMaxTexts = 50
	// deeplMaxBytes keeps one request well under DeepL's 128 KiB limit, which
	// counts the JSON around the text too.
	deeplMaxBytes = 64 << 10
)

// DeepL translates with the DeepL API (https://developers.deepl.com). It
// implements [Translator] and is safe for concurrent use.
type DeepL struct {
	transport  *transport
	header     map[string]string
	english    string
	portuguese string
}

var _ Translator = (*DeepL)(nil)

// NewDeepL builds the driver. It makes no request.
func NewDeepL(c DeepLConfig) (*DeepL, error) {
	key := strings.TrimSpace(c.APIKey)
	if key == "" {
		return nil, errors.New("translate: DeepLConfig.APIKey is empty")
	}
	base := c.BaseURL
	if strings.TrimSpace(base) == "" {
		base = DeepLProBaseURL
		if strings.HasSuffix(key, ":fx") {
			base = DeepLFreeBaseURL
		}
	}
	english, err := variant("EnglishTarget", c.EnglishTarget, "EN-GB", "EN-GB", "EN-US")
	if err != nil {
		return nil, err
	}
	portuguese, err := variant("PortugueseTarget", c.PortugueseTarget, "PT-PT", "PT-PT", "PT-BR")
	if err != nil {
		return nil, err
	}
	t, err := newTransport("deepl", base, c.HTTPClient, c.Timeout, key)
	if err != nil {
		return nil, err
	}
	return &DeepL{
		transport:  t,
		header:     map[string]string{"Authorization": "DeepL-Auth-Key " + key},
		english:    english,
		portuguese: portuguese,
	}, nil
}

// variant checks an optional DeepL code against the ones it may be.
func variant(field, value, fallback string, allowed ...string) (string, error) {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return fallback, nil
	}
	for _, a := range allowed {
		if value == a {
			return a, nil
		}
	}
	return "", fmt.Errorf("translate: DeepLConfig.%s %q is not one of %s", field, value, strings.Join(allowed, ", "))
}

// Name is "deepl".
func (d *DeepL) Name() string { return "deepl" }

// Translate translates the texts of r. More than fifty texts, or more than
// 64 KiB of them, are split over several calls and answered in order. An empty
// text is answered with itself and costs nothing.
func (d *DeepL) Translate(ctx context.Context, r Request) (Result, error) {
	if err := validate(r); err != nil {
		return Result{}, err
	}
	target, err := d.target(r.Target)
	if err != nil {
		return Result{}, err
	}
	source := ""
	if strings.TrimSpace(r.Source) != "" {
		if source, err = deeplSource(r.Source); err != nil {
			return Result{}, err
		}
	}

	type payload struct {
		Text        []string `json:"text"`
		TargetLang  string   `json:"target_lang"`
		SourceLang  string   `json:"source_lang,omitempty"`
		TagHandling string   `json:"tag_handling,omitempty"`
	}
	handling := ""
	if r.Format == FormatHTML {
		handling = "html"
	}

	texts, detected, err := translateAll(r.Text, deeplMaxTexts, deeplMaxBytes, func(batch []string) ([]string, string, error) {
		var answer struct {
			Translations []struct {
				Text                   string `json:"text"`
				DetectedSourceLanguage string `json:"detected_source_language"`
			} `json:"translations"`
		}
		body := payload{Text: batch, TargetLang: target, SourceLang: source, TagHandling: handling}
		if err := d.transport.do(ctx, http.MethodPost, "/v2/translate", nil, d.header, body, &answer, deeplRefusal); err != nil {
			return nil, "", err
		}
		out := make([]string, len(answer.Translations))
		for i, translation := range answer.Translations {
			out[i] = translation.Text
		}
		first := ""
		if len(answer.Translations) > 0 {
			first = fromDeepL(answer.Translations[0].DetectedSourceLanguage)
		}
		return out, first, nil
	})
	if err != nil {
		return Result{}, err
	}
	return Result{Text: texts, DetectedSource: detected, Provider: d.Name()}, nil
}

// Languages lists the languages DeepL translates into, as BCP 47 tags.
func (d *DeepL) Languages(ctx context.Context) ([]string, error) {
	var answer []struct {
		Language string `json:"language"`
	}
	query := url.Values{"type": {"target"}}
	if err := d.transport.do(ctx, http.MethodGet, "/v2/languages", query, d.header, nil, &answer, deeplRefusal); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(answer))
	for _, row := range answer {
		if code := fromDeepL(row.Language); code != "" {
			out = append(out, code)
		}
	}
	return dedupe(out), nil
}

// deeplRefusal reads DeepL's refusals: 403 is the key, 456 the quota, 429 and
// 529 the rate, and a 400 that names a language it does not have.
func deeplRefusal(status int, message string) error {
	switch status {
	case http.StatusForbidden:
		return fmt.Errorf("%w (HTTP 403)", ErrUnauthorized)
	case 456:
		return fmt.Errorf("%w (HTTP 456)", ErrQuotaExceeded)
	case http.StatusTooManyRequests, 529:
		return fmt.Errorf("%w: too many requests (HTTP %d)", ErrQuotaExceeded, status)
	case http.StatusBadRequest:
		lowered := strings.ToLower(message)
		if strings.Contains(lowered, "lang") && strings.Contains(lowered, "not supported") {
			return fmt.Errorf("%w: %s", ErrUnsupportedLanguage, message)
		}
	}
	return fmt.Errorf("translate: deepl answered HTTP %d: %s", status, message)
}

// target is the DeepL code for a BCP 47 target.
//
// DeepL names a target's variant where the language has one: English is EN-GB
// or EN-US, Portuguese PT-BR or PT-PT, Chinese ZH-HANS or ZH-HANT, and Latin
// American Spanish ES-419. A tag that carries a variant keeps it; one that does
// not takes the configured default.
func (d *DeepL) target(raw string) (string, error) {
	t, ok := parseTag(raw)
	if !ok {
		return "", fmt.Errorf("%w: %q is not a BCP 47 tag", ErrUnsupportedLanguage, raw)
	}
	switch t.lang {
	case "en":
		switch t.region {
		case "US", "CA", "PH":
			return "EN-US", nil
		case "GB", "AU", "NZ", "IE", "ZA", "IN", "SG":
			return "EN-GB", nil
		}
		return d.english, nil
	case "pt":
		switch t.region {
		case "BR":
			return "PT-BR", nil
		case "PT", "AO", "MZ":
			return "PT-PT", nil
		}
		return d.portuguese, nil
	case "zh":
		if t.traditional() {
			return "ZH-HANT", nil
		}
		return "ZH-HANS", nil
	case "es":
		if t.region == "419" {
			return "ES-419", nil
		}
	}
	return deeplLanguage(t.lang), nil
}

// deeplSource is the DeepL code for a BCP 47 source. A source has no variant:
// "pt-BR" is "PT", "zh-Hans" is "ZH".
func deeplSource(raw string) (string, error) {
	t, ok := parseTag(raw)
	if !ok {
		return "", fmt.Errorf("%w: %q is not a BCP 47 tag", ErrUnsupportedLanguage, raw)
	}
	return deeplLanguage(t.lang), nil
}

// deeplLanguage is the language part of a DeepL code.
func deeplLanguage(lang string) string {
	switch lang {
	case "no", "nn":
		lang = "nb"
	case "iw":
		lang = "he"
	}
	return strings.ToUpper(lang)
}

// fromDeepL is a DeepL code as a BCP 47 tag: "PT-BR" is "pt-BR", "ZH-HANS" is
// "zh-Hans", "EN" is "en".
func fromDeepL(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}
	lang, rest, _ := strings.Cut(code, "-")
	out := strings.ToLower(lang)
	switch strings.ToUpper(rest) {
	case "":
		return out
	case "HANS":
		return out + "-Hans"
	case "HANT":
		return out + "-Hant"
	}
	return out + "-" + strings.ToUpper(rest)
}
