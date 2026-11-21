# Arandu Translate

One Go interface for machine translation, and a driver for each provider behind
it.

An application depends on `translate.Translator` and never on a provider. Today
the drivers are **DeepL** and **LibreTranslate**; `translate.Fallback` chains
them so the first one that answers is used, and `translate.NewFake` stands in for
all of them in tests. It imports the standard library only, registers nothing and
owns no table: there is no module to wire, no route and no migration.

The package starts small and grows with the providers its users ask for. The
candidates are Google Cloud Translation, Azure Translator, Amazon Translate and
ModernMT; each arrives as one more driver, with the interface unchanged.

## What it is not

- **It does not come with a key.** The API key belongs to whoever holds the
  account with the provider, and every request spends that account's characters.
  Keep it in the environment, never in the repository.
- **It does not store translations.** There is no cache and no table. Keeping a
  translation, and deciding when it is stale, is the application's job.
- **It is not a judge of quality.** Machine translation is a draft. A text that
  carries legal, medical or brand weight needs a person.

## Privacy

The text an application translates **leaves the process and goes to the
provider**: to DeepL's servers, or to the LibreTranslate server the application
points at. Before sending anything a user wrote, know what that provider does
with it.

- **DeepL API Free and API Pro are different contracts.** They differ in how the
  texts are retained and in whether they may be used to improve the service. Read
  DeepL's current terms, and use API Pro (or a self-hosted LibreTranslate) for
  personal or confidential text. A key ending in `:fx` is a Free key.
- **LibreTranslate** keeps the text where you host it. A server run by somebody
  else is that operator's to answer for.
- Tell the people whose text is translated, in the application's privacy notice,
  which providers receive it.

## Install

```sh
go get github.com/hyz-is/arandu-translate@v0.1.0
```

Go 1.26.10 or later.

## Use

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	translate "github.com/hyz-is/arandu-translate"
)

func main() {
	deepl, err := translate.NewDeepL(translate.DeepLConfig{APIKey: os.Getenv("DEEPL_API_KEY")})
	if err != nil {
		panic(err)
	}
	libre, err := translate.NewLibreTranslate(translate.LibreConfig{
		BaseURL: os.Getenv("LIBRETRANSLATE_URL"), // for example http://localhost:5000
		APIKey:  os.Getenv("LIBRETRANSLATE_API_KEY"),
	})
	if err != nil {
		panic(err)
	}

	// Depend on the interface. DeepL first; LibreTranslate when DeepL fails.
	var t translate.Translator = translate.Fallback(deepl, libre)

	result, err := t.Translate(context.Background(), translate.Request{
		Text:   []string{"Pronostics, foot, classement", "Bienvenue"},
		Source: "fr", // empty detects the language
		Target: "pt-BR",
	})
	switch {
	case errors.Is(err, translate.ErrQuotaExceeded):
		fmt.Println("every provider is out of quota")
	case errors.Is(err, translate.ErrUnauthorized):
		fmt.Println("a provider refused its key")
	case errors.Is(err, translate.ErrUnsupportedLanguage):
		fmt.Println("no provider translates into this language")
	case err != nil:
		panic(err)
	}
	fmt.Println(result.Provider, result.DetectedSource, result.Text)
}
```

### The interface

```go
type Translator interface {
	Translate(ctx context.Context, r Request) (Result, error)
	Languages(ctx context.Context) ([]string, error) // targets, BCP 47
	Name() string
}

type Request struct {
	Text   []string // one or more short texts, answered in the same order
	Source string   // BCP 47 ("fr"); empty detects it
	Target string   // BCP 47 ("pt-BR", "zh-Hans", "ar")
	Format string   // "text" (the default) or "html"
}

type Result struct {
	Text           []string
	DetectedSource string // what the provider says the language was, if it says
	Provider       string // the Name of the translator that answered
}
```

Languages are BCP 47 on both sides. Each driver turns a tag into the code its
provider wants, and a provider's codes into tags again in `Languages`. An empty
text is answered with itself and never sent, so it costs no characters.
`Format: "html"` keeps the tags and translates the text between them.

### DeepL

```go
translate.NewDeepL(translate.DeepLConfig{
	APIKey: "...:fx", // ":fx" selects api-free.deepl.com, anything else api.deepl.com
})
```

`DeepLConfig` takes the key, and optionally `BaseURL`, `HTTPClient`, `Timeout`
(15 seconds by default, applied to every request), `EnglishTarget` and
`PortugueseTarget`. The key travels in `Authorization: DeepL-Auth-Key`.

DeepL names the variant of a target language where there is one, so the driver
maps tags like this:

| BCP 47 | DeepL target |
| --- | --- |
| `pt-BR` | `PT-BR` |
| `pt-PT`; `pt` | `PT-PT`; `PortugueseTarget` (`PT-PT` by default) |
| `en-GB`, `en-AU`, ...; `en-US`, `en-CA`, ... | `EN-GB`; `EN-US` |
| `en` | `EnglishTarget` (`EN-GB` by default, or `EN-US`) |
| `zh-Hans`, `zh`, `zh-CN` | `ZH-HANS` |
| `zh-Hant`, `zh-TW`, `zh-HK` | `ZH-HANT` |
| `es-419` | `ES-419` |
| `ar`, `ja`, `ru`, `de`, `it`, `es`, `fr`, ... | the language in capitals |

A source has no variant: `pt-BR`, `en-US` and `zh-Hans` are sent as `PT`, `EN`
and `ZH`. A language DeepL does not know is refused by DeepL and arrives as
`ErrUnsupportedLanguage`. `Languages` lists DeepL's targets, so an application
can offer only what the account can translate.

A list is sent in calls of at most fifty texts and 64 KiB, and the answers are
put back in order.

### LibreTranslate

```go
translate.NewLibreTranslate(translate.LibreConfig{
	BaseURL: "http://localhost:5000", // required: the server is yours or somebody's
	APIKey:  "",                      // only if the server asks for one
})
```

LibreTranslate codes are the language alone (`pt-BR` is sent as `pt`), with the
two Chinese scripts as `zh` and `zt`; `Languages` answers them as `zh-Hans` and
`zh-Hant`. What a server translates depends on the models its operator installed.

### Errors

Test them with `errors.Is`.

| error | DeepL | LibreTranslate |
| --- | --- | --- |
| `ErrUnauthorized` | HTTP 403 | HTTP 403, or a 400 about the API key |
| `ErrQuotaExceeded` | HTTP 456, and 429 or 529 | HTTP 429 |
| `ErrUnsupportedLanguage` | a 400 saying a language is not supported, or a tag that is not BCP 47 | a 400 saying a language is not supported |

Any other refusal is an error carrying the provider's message. No error carries
the API key. The package does not retry: waiting spends the caller's time and
the account's allowance, and only the application knows how long it can wait.

### Fallback

`translate.Fallback(a, b, c)` returns a `Translator`. `Translate` tries each in
order and returns the first answer without an error, with `Result.Provider`
naming who gave it. When every translator fails, the error joins theirs, so
`errors.Is` finds each cause. A cancelled context stops the chain. `Languages` is
the union of what the translators list.

Any error moves to the next translator, including `ErrUnsupportedLanguage`, so a
language only the second provider knows still gets translated.

## Test your own code

`NewFake` returns a `Translator` that never touches the network and answers
`[<target>] <text>`:

```go
fake := translate.NewFake()
got, _ := fake.Translate(ctx, translate.Request{Text: []string{"bonjour"}, Source: "fr", Target: "pt-BR"})
// got.Text is []string{"[pt-BR] bonjour"}

fake.SetLanguages([]string{"en", "pt-BR"}) // other targets now fail with ErrUnsupportedLanguage
fake.FailWith(translate.ErrQuotaExceeded)  // every call now fails; FailWith(nil) undoes it
sent := fake.Requests()                    // what the code under test asked for
```

## The manifest

`arandu.mod.toml` declares `network = true` and nothing else: the package sends
requests to the provider a driver is built for. `tests/Unit/audit_test.go`
compares the declaration with what the code calls.

## Development

```sh
export GOWORK=off
gofmt -l $(find . -name '*.go' -not -path '*/testdata/*' -not -name '*.kyse.go')
go build ./...
go vet ./...
go test -race ./...
```

No test reaches the network: each provider is a local `httptest` server with JSON
written by hand from its public documentation.

See [CONTRIBUTING.md](CONTRIBUTING.md), [CHANGELOG.md](CHANGELOG.md) and
[UPGRADE.md](UPGRADE.md). Vulnerabilities follow [SECURITY.md](SECURITY.md).

## Licence

MIT, see [LICENSE.md](LICENSE.md).
