# Working on Arandu Translate

This is an Arandu package for machine translation: one interface
(`Translator`) and a driver for each provider, `NewDeepL` and
`NewLibreTranslate`, plus `Fallback`, which chains translators, and `NewFake`,
an in-memory stand-in. It is a Go module somebody `go get`s and imports.

It began as the package skeleton, configured as `hyz-is/translate`, and the HTTP
module the template ships was removed on purpose: there is no entity, no table,
no migration, no policy, no route and no `Module` to register. Translating a text
is a call the application makes with values it already has, and nothing here
should grow back into a module.

Read `.agents/skills/` before writing code. Each skill is a procedure, and the
one you need is named by the situation you are in.

## The gates

Nothing is finished until all four exit zero.

```sh
export GOWORK=off
gofmt -l $(find . -name '*.go' -not -path '*/testdata/*' -not -name '*.kyse.go')
go build ./...
go vet ./...
go test -race ./...
```

`GOWORK=off` is not a preference. This checkout may sit beside a Go workspace
that lists the framework repositories and does not list this one; when it does,
every command above fails before it compiles anything:

```
pattern ./...: directory prefix . does not contain modules listed in go.work
or their selected dependencies
```

With the workspace off, the module resolves the versions in `go.mod`, which is
what CI compiles against and what somebody's `go get` will get.

`aru doctor` is not one of the gates. It reads applications, and exits 1 here.
It does not read this package after it is installed either, which is why
`tests/Unit/audit_test.go` compares `arandu.mod.toml` with what the code calls.

## What this repository holds

| | measured with |
| --- | --- |
| 9 Go files in the root package, and `doc.go` | `ls *.go` |
| 0 routes, 0 tables, 1 capability (`network`) | `arandu.mod.toml`, `TestThePackageIsALibrary` |
| 0 dependencies: the standard library only | `go list -m all` |

```
doc.go            the package comment
types.go          Request, Result, Translator and the three errors
config.go         DeepLConfig and LibreConfig, their defaults, and a String that hides the key
transport.go      the one bounded JSON request both drivers use, and the scrubbing of the key
batch.go          splitting a list into calls and putting the answers back in order
languages.go      BCP 47 tags read into language, script and region
deepl.go          DeepL: the codes, the limits, the refusals
libretranslate.go LibreTranslate: the same
fallback.go       Fallback: the first translator that answers
fake.go           Fake: the same Translator, in memory
```

## What does not exist here, on purpose

| A model reaches for | What is here instead |
| --- | --- |
| a `Module`, a route, a line in `WithProviders` | nothing to register. `NewDeepL` builds a driver the application keeps |
| a cache or a table of translations | nothing. The application stores what it keeps |
| a retry loop that waits out a quota | the error, returned at once. Waiting spends the caller's time and the account's allowance |
| a third dependency, or an import of the framework | the standard library. `TestThePackageIsALibrary` and `TestTheModuleRequiresNothing` hold it |
| the API key in a log line, an error or `%v` | `String` and `GoString` hide it and `transport.scrub` removes it from every error |
| a provider's own language codes in the public API | BCP 47 tags. The mapping lives inside each driver |
| a host written in the code besides DeepL's | the address the application configures. `TestThePackageNamesNoHostButDeepL` holds it |

## The four properties

A change that breaks one of them is not merged, whatever else it improves.

1. **One interface, many drivers.** A provider is a driver that implements
   `Translator`, in its own file with its own tests. Its codes, limits and error
   statuses stay inside it, and the public contract (`Request`, `Result`,
   `Translator`, the three errors) is unchanged by a new driver.
2. **It is a library.** The standard library only, no route, no table, no
   `Module`, no `init`.
3. **The key stays on the server.** It reaches the request that authenticates
   and nothing else, and `TestTheKeyIsInNoError` and
   `TestTheKeyIsInNoFormattedConfiguration` hold it.
4. **`arandu.mod.toml` matches the code.** `network = true` for the requests, the
   other three `false`, and `tests/Unit/audit_test.go` compares that with what
   the code calls.

## Adding a driver

1. Read the provider's public documentation: authentication, the translate and
   languages endpoints, the limits, the refusals. Write the JSON of the tests by
   hand from it; never capture it with a real key.
2. Build on `newTransport` and `translateAll`: they bound the request, scrub the
   key and split the list.
3. Map the provider's refusals to the three errors, and nothing else.
4. Map BCP 47 in and out inside the driver.
5. A config type with `String` and `GoString` that hide the key, a changelog line
   and a README row.

## Writing code

Everything in the source is in English: identifiers, doc comments, internal
comments, error messages, and the names and messages of tests.

Every exported symbol carries a doc comment, and the comment documents the
symbol and nothing else. Why a signature is what it is belongs there when it is
a fact about the code. A date, an issue number, a version in progress or the
name of another repository does not.

The exported names are a contract other repositories are written against. A
change to one is a break, and `UPGRADE.md` names it.

Tests go under `tests/Unit`, declaring `package unit_test`, and import the
package by its module path, which is what makes them see exactly what a caller
sees. No test reaches the network: the provider is a local `httptest` server,
and the JSON it answers with is written by hand from the public documentation.
