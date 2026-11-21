package translate

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
)

// Fake is a [Translator] that holds no provider: it translates "texto" into
// "[pt-BR] texto". It is for the tests of whatever uses a translator, and it is
// safe for concurrent use.
type Fake struct {
	mu        sync.Mutex
	err       error
	languages []string
	limited   bool
	requests  []Request
}

var _ Translator = (*Fake)(nil)

// NewFake builds a [Fake] that lists a few languages and accepts any target.
func NewFake() *Fake {
	return &Fake{languages: []string{"fr", "en", "es", "pt-BR", "de", "it", "ar", "zh-Hans", "ja", "ru"}}
}

// Name is "fake".
func (f *Fake) Name() string { return "fake" }

// FailWith makes every later call return err, until FailWith(nil).
func (f *Fake) FailWith(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// SetLanguages replaces what Languages lists and, from then on, refuses a
// target that is not in the list with [ErrUnsupportedLanguage]. Tags are
// compared without case.
func (f *Fake) SetLanguages(languages []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.languages = slices.Clone(languages)
	f.limited = true
}

// Requests returns every request Translate received, in order.
func (f *Fake) Requests() []Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Request, len(f.requests))
	for i, r := range f.requests {
		r.Text = slices.Clone(r.Text)
		out[i] = r
	}
	return out
}

// Translate answers "[<target>] <text>" for each text. DetectedSource is
// Request.Source, or "und" when it is empty.
func (f *Fake) Translate(_ context.Context, r Request) (Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r.Text = slices.Clone(r.Text)
	f.requests = append(f.requests, r)
	if f.err != nil {
		return Result{}, f.err
	}
	if err := validate(r); err != nil {
		return Result{}, err
	}
	if f.limited && !slices.ContainsFunc(f.languages, func(l string) bool { return strings.EqualFold(l, r.Target) }) {
		return Result{}, fmt.Errorf("%w: %q", ErrUnsupportedLanguage, r.Target)
	}
	out := make([]string, len(r.Text))
	for i, text := range r.Text {
		out[i] = "[" + r.Target + "] " + text
	}
	source := r.Source
	if source == "" {
		source = "und"
	}
	return Result{Text: out, DetectedSource: source, Provider: f.Name()}, nil
}

// Languages lists the languages set with SetLanguages, or the default few.
func (f *Fake) Languages(context.Context) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	return slices.Clone(f.languages), nil
}
