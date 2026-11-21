package unit_test

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"

	translate "github.com/hyz-is/arandu-translate"
)

func TestFallbackUsesTheFirstTranslatorThatAnswers(t *testing.T) {
	broken, working, spare := translate.NewFake(), translate.NewFake(), translate.NewFake()
	broken.FailWith(translate.ErrQuotaExceeded)

	chain := translate.Fallback(broken, working, spare)
	got, err := chain.Translate(context.Background(), translate.Request{Text: []string{"oi"}, Target: "en"})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if !reflect.DeepEqual(got.Text, []string{"[en] oi"}) {
		t.Errorf("Text = %q", got.Text)
	}
	if len(broken.Requests()) != 1 || len(working.Requests()) != 1 {
		t.Errorf("requests: broken %d, working %d, want 1 and 1", len(broken.Requests()), len(working.Requests()))
	}
	if n := len(spare.Requests()); n != 0 {
		t.Errorf("the translator after the one that answered was called %d times", n)
	}
}

func TestFallbackStopsAtTheFirstSuccess(t *testing.T) {
	first, second := translate.NewFake(), translate.NewFake()
	if _, err := translate.Fallback(first, second).Translate(context.Background(), translate.Request{Text: []string{"x"}, Target: "en"}); err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if len(second.Requests()) != 0 {
		t.Error("the second translator was called although the first answered")
	}
}

func TestFallbackReportsEveryFailure(t *testing.T) {
	a, b := translate.NewFake(), translate.NewFake()
	a.FailWith(translate.ErrUnauthorized)
	b.FailWith(translate.ErrQuotaExceeded)

	_, err := translate.Fallback(a, b).Translate(context.Background(), translate.Request{Text: []string{"x"}, Target: "en"})
	if !errors.Is(err, translate.ErrUnauthorized) || !errors.Is(err, translate.ErrQuotaExceeded) {
		t.Errorf("error = %v, want both causes", err)
	}
}

func TestFallbackStopsWhenTheContextIsDone(t *testing.T) {
	a, b := translate.NewFake(), translate.NewFake()
	a.FailWith(errors.New("down"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := translate.Fallback(a, b).Translate(ctx, translate.Request{Text: []string{"x"}, Target: "en"})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
	if len(a.Requests())+len(b.Requests()) != 0 {
		t.Error("a translator was called with a cancelled context")
	}
}

func TestFallbackNamesAndSkipsNothing(t *testing.T) {
	chain := translate.Fallback(translate.NewFake(), nil, translate.NewFake())
	if got := chain.Name(); got != "fallback(fake,fake)" {
		t.Errorf("Name = %q", got)
	}
	empty := translate.Fallback()
	if _, err := empty.Translate(context.Background(), translate.Request{Text: []string{"x"}, Target: "en"}); err == nil {
		t.Error("an empty fallback answered")
	}
	if _, err := empty.Languages(context.Background()); err == nil {
		t.Error("an empty fallback listed languages")
	}
}

func TestFallbackListsTheUnionOfLanguages(t *testing.T) {
	a, b, c := translate.NewFake(), translate.NewFake(), translate.NewFake()
	a.SetLanguages([]string{"en", "pt-BR"})
	b.FailWith(errors.New("down"))
	c.SetLanguages([]string{"EN", "ja"})

	got, err := translate.Fallback(a, b, c).Languages(context.Background())
	if err != nil {
		t.Fatalf("Languages: %v", err)
	}
	if want := []string{"en", "pt-BR", "ja"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Languages = %q, want %q", got, want)
	}

	if _, err := translate.Fallback(b).Languages(context.Background()); err == nil {
		t.Error("Languages succeeded when no translator answered")
	}
}

func TestFallbackWorksWithTheDrivers(t *testing.T) {
	down, _ := newDeepL(t, func(_ seen, w http.ResponseWriter) { reply(w, 456, `{"message":"Quota exceeded"}`) })
	up, _ := newLibre(t, echoLibre)

	got, err := translate.Fallback(down, up).Translate(context.Background(), translate.Request{Text: []string{"oi"}, Target: "en"})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if got.Provider != "libretranslate" || !reflect.DeepEqual(got.Text, []string{"T:oi"}) {
		t.Errorf("got %+v, want the LibreTranslate answer", got)
	}
}

func TestFallbackSatisfiesTranslator(t *testing.T) {
	var _ translate.Translator = translate.Fallback()
	var _ translate.Translator = translate.NewFake()
}
