package translate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// maxBody caps what is read from one answer.
const maxBody = 16 << 20

// maxMessage caps how much of a provider's error message is kept.
const maxMessage = 200

// transport is what both drivers share: one bounded JSON request, and the
// guarantee that the key is in no error that comes out of it.
type transport struct {
	name    string
	baseURL string
	client  *http.Client
	timeout time.Duration
	secret  string
}

// newTransport validates the address and the timeout and builds the client.
func newTransport(name, base string, client *http.Client, timeout time.Duration, secret string) (*transport, error) {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	parsed, err := url.Parse(base)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("translate: the %s BaseURL is not an http or https address", name)
	}
	if timeout < 0 {
		return nil, fmt.Errorf("translate: the %s Timeout is negative", name)
	}
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	return &transport{name: name, baseURL: base, client: client, timeout: timeout, secret: secret}, nil
}

// classify turns a refused status and the provider's message into an error.
type classify func(status int, message string) error

// do sends one request and decodes a 200 answer into out.
func (t *transport) do(ctx context.Context, method, path string, query url.Values, header map[string]string, body any, out any, refuse classify) error {
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("translate: %s: encoding the request: %w", t.name, err)
		}
		reader = bytes.NewReader(encoded)
	}
	endpoint := t.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return t.scrubbed(fmt.Errorf("translate: %s: building the request: %w", t.name, err))
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "arandu-translate")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for name, value := range header {
		request.Header.Set(name, value)
	}

	response, err := t.client.Do(request)
	if err != nil {
		return t.scrubbed(fmt.Errorf("translate: %s: %s: %w", t.name, path, err))
	}
	defer response.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(response.Body, maxBody))
	if err != nil {
		return t.scrubbed(fmt.Errorf("translate: %s: reading %s: %w", t.name, path, err))
	}
	if response.StatusCode != http.StatusOK {
		return t.scrubbed(refuse(response.StatusCode, t.message(raw)))
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("translate: %s: %s sent an unexpected response: %w", t.name, path, err)
	}
	return nil
}

// message reads the explanation out of a provider's error body, without the
// key and without more than a short line.
func (t *transport) message(raw []byte) string {
	var shaped struct {
		Message string          `json:"message"`
		Detail  string          `json:"detail"`
		Error   json.RawMessage `json:"error"`
	}
	text := ""
	if json.Unmarshal(raw, &shaped) == nil {
		switch {
		case shaped.Message != "":
			text = shaped.Message
		case shaped.Detail != "":
			text = shaped.Detail
		case len(shaped.Error) > 0:
			var plain string
			var nested struct {
				Message string `json:"message"`
			}
			if json.Unmarshal(shaped.Error, &plain) == nil {
				text = plain
			} else if json.Unmarshal(shaped.Error, &nested) == nil {
				text = nested.Message
			}
		}
	} else {
		text = string(raw)
	}
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > maxMessage {
		text = text[:maxMessage]
	}
	return t.scrub(text)
}

func (t *transport) scrub(text string) string {
	if t.secret == "" {
		return text
	}
	return strings.ReplaceAll(text, t.secret, "[redacted]")
}

// scrubbedError prints without the key and still unwraps to the original.
type scrubbedError struct {
	text string
	err  error
}

func (e *scrubbedError) Error() string { return e.text }
func (e *scrubbedError) Unwrap() error { return e.err }

func (t *transport) scrubbed(err error) error {
	if err == nil || t.secret == "" {
		return err
	}
	text := err.Error()
	if !strings.Contains(text, t.secret) {
		return err
	}
	return &scrubbedError{text: t.scrub(text), err: errors.Unwrap(err)}
}
