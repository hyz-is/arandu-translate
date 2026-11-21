---
name: translate-package
description: Translate text in an Arandu application with Arandu Translate. Use when the request is to "translate the catalogue", "machine translation", "translate this text", "DeepL", "LibreTranslate", "add a translation provider", "fall back to another translator", "go get github.com/hyz-is/arandu-translate", "ErrQuotaExceeded", "ErrUnauthorized", "ErrUnsupportedLanguage", "which languages can we translate into", "test the code that translates", or when a project's go.mod already requires github.com/hyz-is/arandu-translate. Covers the key, the drivers built once and kept as a translate.Translator, Fallback, the BCP 47 tags, the errors, and the Fake for tests.
license: MIT
metadata:
  audience: app
---

# Using Arandu Translate

Three pieces, each a few lines the application writes. Nothing is registered:
there is no module, no route and no migration.

## 1. The key and the privacy

An account at DeepL gives a key; a LibreTranslate server may ask for one. The key
belongs to whoever holds the account and every request spends that account's
characters. It goes in `.env` and in the deployment's secrets, never in the
repository:

```dotenv
DEEPL_API_KEY=...
LIBRETRANSLATE_URL=http://localhost:5000
```

The text leaves the application and goes to the provider. DeepL API Free (key
ending `:fx`) and API Pro have different terms for retention and for using texts
to improve the service: use Pro or a self-hosted LibreTranslate for anything
personal, and name the providers in the privacy notice.

## 2. The drivers, built once

```sh
go get github.com/hyz-is/arandu-translate@v0.1.0
```

```go
deepl, err := translate.NewDeepL(translate.DeepLConfig{APIKey: os.Getenv("DEEPL_API_KEY")})
if err != nil {
	return err // an empty key, or a BaseURL that is not http or https
}
libre, err := translate.NewLibreTranslate(translate.LibreConfig{BaseURL: os.Getenv("LIBRETRANSLATE_URL")})
if err != nil {
	return err
}
var t translate.Translator = translate.Fallback(deepl, libre)
```

Keep it where the application keeps its services (explicit wiring, for example in
`bootstrap/app.go`), and give the code that translates a `translate.Translator`,
not a `*translate.DeepL`, so a test can hand it `translate.NewFake()`.

## 3. The calls

```go
res, err := t.Translate(ctx, translate.Request{Text: []string{"Bienvenue"}, Source: "fr", Target: "pt-BR"})
langs, err := t.Languages(ctx) // the targets, as BCP 47
```

- Tags are BCP 47 on both sides: `fr`, `pt-BR`, `zh-Hans`, `ar`. Never a
  provider's own code (`PT-BR`, `zt`); the driver maps them.
- `Text` is a list and the answer keeps its order. Empty texts come back empty and
  cost nothing. Send short texts, not whole documents.
- `Source` empty means the provider detects it; `res.DetectedSource` says what it
  found, when it says.
- `Format: "html"` keeps the tags. The default is plain text.
- `res.Provider` names who answered, which matters with `Fallback`.
- Translate a catalogue once and store the result in the application; do not
  translate on every request.

## Errors

`errors.Is(err, translate.ErrQuotaExceeded)`: out of characters or requests. Do not
retry in a loop; tell the operator or try later. `ErrUnauthorized`: the key was
refused; retrying will not help. `ErrUnsupportedLanguage`: the provider does not
translate into or from that language; check `Languages` first. With `Fallback`,
the error joins every provider's, so `errors.Is` finds each. Anything else is the
provider's own message. No error carries the key.

## What the package does not do

- It stores nothing and caches nothing.
- It does not retry or wait.
- It does not judge quality: a machine translation of legal or brand text needs a
  person.

## Tests

```go
fake := translate.NewFake()
res, _ := fake.Translate(ctx, translate.Request{Text: []string{"bonjour"}, Target: "pt-BR"})
// res.Text == []string{"[pt-BR] bonjour"}
fake.SetLanguages([]string{"en", "pt-BR"}) // other targets now fail with ErrUnsupportedLanguage
fake.FailWith(translate.ErrQuotaExceeded)  // FailWith(nil) undoes it
asked := fake.Requests()
```

No test of the application should reach the network.

## Symptoms

| symptom | piece |
| --- | --- |
| `translate: DeepLConfig.APIKey is empty` | 1: the variable is not set in this environment |
| `ErrUnauthorized` on every call | 1: wrong key, or a Free key against a Pro account |
| `ErrQuotaExceeded` after a while | 3: the month's characters are spent; translate once and store |
| `ErrUnsupportedLanguage` for `pt` or `zh` | 3: use `pt-BR` and `zh-Hans`; ask `Languages` |
| the Chinese comes out traditional | 3: send `zh-Hans`; `zh-Hant` and `zh-TW` are traditional |
