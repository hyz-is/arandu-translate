// Package translate is one interface for machine translation and the drivers
// behind it.
//
// An application depends on [Translator]. A driver turns a [Request] into the
// call a provider expects and the answer into a [Result]: [NewDeepL] for DeepL
// and [NewLibreTranslate] for a LibreTranslate server, free or self-hosted.
// [Fallback] chains translators so the first one that answers without an error
// is used, and [NewFake] is an in-memory translator, so an application's tests
// never reach the network.
//
// Languages are BCP 47 tags on both sides ("fr", "pt-BR", "zh-Hans", "ar"). The
// code a provider wants for each of them lives inside its driver.
//
// The package imports only the standard library and registers nothing: there is
// no Module, no route and no table.
package translate
