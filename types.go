package translate

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// The values [Request.Format] takes.
const (
	// FormatText is plain text, and the default when Format is empty.
	FormatText = "text"
	// FormatHTML is markup: the tags are kept and only the text between them is
	// translated.
	FormatHTML = "html"
)

// Request is what a [Translator] translates.
type Request struct {
	// Text is one or more short texts. The answer holds one translation per
	// text, in the same order.
	Text []string
	// Source is the language of Text, a BCP 47 tag such as "fr". Empty means the
	// provider detects it.
	Source string
	// Target is the language to translate into, a BCP 47 tag such as "pt-BR",
	// "zh-Hans" or "ar". It is required.
	Target string
	// Format is [FormatText] (the default) or [FormatHTML].
	Format string
}

// Result is what a [Translator] answers.
type Result struct {
	// Text holds the translations, in the order of [Request.Text].
	Text []string
	// DetectedSource is the language of the text as the provider reports it, a
	// BCP 47 tag with no region ("en", "pt", "zh"). It is empty when the
	// provider does not say.
	DetectedSource string
	// Provider is the [Translator.Name] of the translator that answered.
	Provider string
}

// Translator translates text. Every driver, [Fallback] and the [Fake] implement
// it, and it is what an application should depend on.
type Translator interface {
	// Translate translates Request.Text into Request.Target.
	Translate(ctx context.Context, r Request) (Result, error)
	// Languages lists the languages it can translate into, as BCP 47 tags.
	Languages(ctx context.Context) ([]string, error)
	// Name identifies the provider, for logs and for [Result.Provider].
	Name() string
}

var (
	// ErrUnauthorized means the provider refused the API key.
	ErrUnauthorized = errors.New("translate: the provider refused the API key")
	// ErrQuotaExceeded means the account is out of characters or requests for
	// the period, or the provider is limiting the rate. The fix is a larger
	// plan or a later request.
	ErrQuotaExceeded = errors.New("translate: the provider's quota was exceeded")
	// ErrUnsupportedLanguage means the provider cannot translate from or into
	// a language of the request.
	ErrUnsupportedLanguage = errors.New("translate: the language is not supported by the provider")
)

// validate refuses a request no provider could answer, before it costs a call.
func validate(r Request) error {
	if strings.TrimSpace(r.Target) == "" {
		return errors.New("translate: Request.Target is empty")
	}
	switch r.Format {
	case "", FormatText, FormatHTML:
	default:
		return fmt.Errorf("translate: Request.Format %q is neither %q nor %q", r.Format, FormatText, FormatHTML)
	}
	return nil
}
