package translate

import (
	"fmt"
	"net/http"
	"time"
)

const (
	// DeepLProBaseURL is the DeepL API Pro address, used for a key that does not
	// end in ":fx".
	DeepLProBaseURL = "https://api.deepl.com"
	// DeepLFreeBaseURL is the DeepL API Free address, used for a key that ends
	// in ":fx".
	DeepLFreeBaseURL = "https://api-free.deepl.com"
	// DefaultTimeout bounds one request when the configuration's Timeout is
	// empty.
	DefaultTimeout = 15 * time.Second
)

// DeepLConfig is what [NewDeepL] needs.
type DeepLConfig struct {
	// APIKey is the account's key, sent as "Authorization: DeepL-Auth-Key". It
	// belongs to the account that holds the subscription. A key that ends in
	// ":fx" is a DeepL API Free key.
	APIKey string
	// BaseURL is the service's address. Empty means [DeepLFreeBaseURL] for a
	// ":fx" key and [DeepLProBaseURL] for any other. A test points it at a local
	// server.
	BaseURL string
	// HTTPClient makes the requests; nil means a client with Timeout.
	HTTPClient *http.Client
	// Timeout bounds one request; empty means [DefaultTimeout].
	Timeout time.Duration
	// EnglishTarget is the DeepL code used when the target is English with no
	// region, or with one DeepL has no variant for: "EN-GB" (the default) or
	// "EN-US".
	EnglishTarget string
	// PortugueseTarget is the DeepL code used when the target is Portuguese with
	// no region: "PT-PT" (the default) or "PT-BR". "pt-BR" always becomes
	// "PT-BR".
	PortugueseTarget string
}

// String formats the configuration without the key.
func (c DeepLConfig) String() string {
	return fmt.Sprintf("translate.DeepLConfig{APIKey:%s BaseURL:%q Timeout:%s EnglishTarget:%q PortugueseTarget:%q}",
		redact(c.APIKey), c.BaseURL, c.Timeout, c.EnglishTarget, c.PortugueseTarget)
}

// GoString formats the configuration without the key, for %#v.
func (c DeepLConfig) GoString() string { return c.String() }

// LibreConfig is what [NewLibreTranslate] needs.
type LibreConfig struct {
	// BaseURL is the address of the LibreTranslate server, such as
	// "http://localhost:5000" for one the application hosts. It is required.
	BaseURL string
	// APIKey is the key the server asks for, if it asks for one. It is sent as
	// the api_key field of the request.
	APIKey string
	// HTTPClient makes the requests; nil means a client with Timeout.
	HTTPClient *http.Client
	// Timeout bounds one request; empty means [DefaultTimeout].
	Timeout time.Duration
}

// String formats the configuration without the key.
func (c LibreConfig) String() string {
	return fmt.Sprintf("translate.LibreConfig{BaseURL:%q APIKey:%s Timeout:%s}", c.BaseURL, redact(c.APIKey), c.Timeout)
}

// GoString formats the configuration without the key, for %#v.
func (c LibreConfig) GoString() string { return c.String() }

func redact(key string) string {
	if key == "" {
		return `""`
	}
	return "[redacted]"
}
