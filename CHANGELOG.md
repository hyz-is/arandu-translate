# Changelog

Everything worth knowing about a release of Arandu Translate is recorded here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
the versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

A published module version is immutable: Go serves it from the proxy forever, so
a release is corrected by another release and never by moving a tag.

## [Unreleased]

## [0.1.0] - 2026-11-21

### Added

- `Translator`, the interface an application depends on: `Translate`,
  `Languages` and `Name`. A `Request` carries `Text` (one or more texts, answered
  in the same order), `Source` (BCP 47, empty to detect), `Target` (BCP 47) and
  `Format` (`text` or `html`); a `Result` carries `Text`, `DetectedSource` and
  `Provider`.
- `NewDeepL(DeepLConfig)`, a driver for the DeepL API. A key ending in `:fx` uses
  `api-free.deepl.com`, any other `api.deepl.com`. BCP 47 tags become DeepL's
  codes (`pt-BR` is `PT-BR`, `zh-Hans` is `ZH-HANS`, `en` is `EN-GB` unless
  `DeepLConfig.EnglishTarget` says `EN-US`), HTML is translated with
  `tag_handling=html`, and a list is split into calls of at most fifty texts.
- `NewLibreTranslate(LibreConfig)`, a driver for a LibreTranslate server, with an
  optional API key.
- `Fallback(...Translator)`, which tries each translator in order and answers
  with the first that responds without an error.
- `NewFake`, a `Translator` in memory that answers `[<target>] <text>`, with
  `FailWith` and `SetLanguages`, for the tests of whatever translates.
- `ErrUnauthorized`, `ErrQuotaExceeded` and `ErrUnsupportedLanguage`, read from
  each provider's refusals: DeepL 403, 456, and a 400 naming a language;
  LibreTranslate 403, 429, and a 400 saying a language is not supported.
- `DeepLConfig` and `LibreConfig` print without their key, and no error carries
  it. The package imports the standard library only.
