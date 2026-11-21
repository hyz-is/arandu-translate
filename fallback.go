package translate

import (
	"context"
	"errors"
	"strings"
)

// errNoTranslator is what an empty [Fallback] answers.
var errNoTranslator = errors.New("translate: the fallback has no translator")

type fallback struct {
	chain []Translator
}

// Fallback chains translators. Translate tries each in order and returns the
// answer of the first that responds without an error; when every one fails the
// error joins theirs, so errors.Is finds each cause. A cancelled context stops
// the chain. Languages is the union of what the translators list, and fails
// only when none of them answers.
func Fallback(ts ...Translator) Translator {
	chain := make([]Translator, 0, len(ts))
	for _, t := range ts {
		if t != nil {
			chain = append(chain, t)
		}
	}
	return &fallback{chain: chain}
}

func (f *fallback) Name() string {
	names := make([]string, len(f.chain))
	for i, t := range f.chain {
		names[i] = t.Name()
	}
	return "fallback(" + strings.Join(names, ",") + ")"
}

func (f *fallback) Translate(ctx context.Context, r Request) (Result, error) {
	if len(f.chain) == 0 {
		return Result{}, errNoTranslator
	}
	var failures []error
	for _, t := range f.chain {
		if err := ctx.Err(); err != nil {
			return Result{}, errors.Join(append(failures, err)...)
		}
		result, err := t.Translate(ctx, r)
		if err == nil {
			return result, nil
		}
		failures = append(failures, err)
	}
	return Result{}, errors.Join(failures...)
}

func (f *fallback) Languages(ctx context.Context) ([]string, error) {
	if len(f.chain) == 0 {
		return nil, errNoTranslator
	}
	var (
		all      []string
		failures []error
		answered bool
	)
	for _, t := range f.chain {
		languages, err := t.Languages(ctx)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		answered = true
		all = append(all, languages...)
	}
	if !answered {
		return nil, errors.Join(failures...)
	}
	return dedupe(all), nil
}
