package unit_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	translate "github.com/hyz-is/arandu-translate"
)

func TestFakeTranslatesIntoTheTargetTag(t *testing.T) {
	f := translate.NewFake()
	got, err := f.Translate(context.Background(), translate.Request{Text: []string{"bonjour", "merci"}, Source: "fr", Target: "pt-BR"})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	want := translate.Result{Text: []string{"[pt-BR] bonjour", "[pt-BR] merci"}, DetectedSource: "fr", Provider: "fake"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if f.Name() != "fake" {
		t.Errorf("Name = %q", f.Name())
	}
	detected, _ := f.Translate(context.Background(), translate.Request{Text: []string{"x"}, Target: "en"})
	if detected.DetectedSource != "und" {
		t.Errorf("DetectedSource without a source = %q, want und", detected.DetectedSource)
	}
}

func TestFakeAcceptsAnyTargetUntilLanguagesAreSet(t *testing.T) {
	f := translate.NewFake()
	if _, err := f.Translate(context.Background(), translate.Request{Text: []string{"x"}, Target: "ko"}); err != nil {
		t.Errorf("Translate into ko: %v", err)
	}
	f.SetLanguages([]string{"en", "pt-BR"})
	if got, _ := f.Languages(context.Background()); !reflect.DeepEqual(got, []string{"en", "pt-BR"}) {
		t.Errorf("Languages = %q", got)
	}
	if _, err := f.Translate(context.Background(), translate.Request{Text: []string{"x"}, Target: "PT-br"}); err != nil {
		t.Errorf("Translate into PT-br: %v", err)
	}
	if _, err := f.Translate(context.Background(), translate.Request{Text: []string{"x"}, Target: "ko"}); !errors.Is(err, translate.ErrUnsupportedLanguage) {
		t.Errorf("Translate into ko = %v, want ErrUnsupportedLanguage", err)
	}
}

func TestFakeFailsOnDemandAndRecovers(t *testing.T) {
	f := translate.NewFake()
	f.FailWith(translate.ErrQuotaExceeded)
	if _, err := f.Translate(context.Background(), translate.Request{Text: []string{"x"}, Target: "en"}); !errors.Is(err, translate.ErrQuotaExceeded) {
		t.Errorf("Translate = %v, want ErrQuotaExceeded", err)
	}
	if _, err := f.Languages(context.Background()); !errors.Is(err, translate.ErrQuotaExceeded) {
		t.Errorf("Languages = %v, want ErrQuotaExceeded", err)
	}
	f.FailWith(nil)
	if _, err := f.Translate(context.Background(), translate.Request{Text: []string{"x"}, Target: "en"}); err != nil {
		t.Errorf("Translate after FailWith(nil) = %v", err)
	}
}

func TestFakeRecordsACopyOfEachRequest(t *testing.T) {
	f := translate.NewFake()
	text := []string{"a"}
	_, _ = f.Translate(context.Background(), translate.Request{Text: text, Target: "en"})
	text[0] = "changed"
	recorded := f.Requests()
	if len(recorded) != 1 || recorded[0].Text[0] != "a" {
		t.Errorf("Requests = %+v, want the request as it was sent", recorded)
	}
	recorded[0].Text[0] = "mutated"
	if f.Requests()[0].Text[0] != "a" {
		t.Error("a caller changed what the Fake recorded")
	}
}

func TestFakeRefusesWhatNoProviderAccepts(t *testing.T) {
	f := translate.NewFake()
	if _, err := f.Translate(context.Background(), translate.Request{Text: []string{"x"}}); err == nil {
		t.Error("a request with no target was answered")
	}
}
