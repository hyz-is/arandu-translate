# Contributing

Thank you for considering a contribution.

## Before you open a pull request

Run the four gates. They are what CI runs, and a pull request that fails one of
them fails for everybody who pulls after it.

```bash
gofmt -l $(find . -name '*.go' -not -path '*/testdata/*' -not -name '*.kyse.go')
go build ./...
go vet ./...
go test -race ./...
```

The two filters on `gofmt` are not preference. `gofmt` is the only tool in the
chain that ignores build tags, so it reads files the compiler deliberately
excludes; `testdata/` holds fixtures that are invalid on purpose.

## What a change has to keep true

Four properties are the reason this package is what it is. A change that breaks
one of them will not be merged, whatever else it improves.

1. **One interface, many drivers.** An application depends on `Translator`. A
   provider is a driver that implements it, and what is specific to the provider
   (its codes, its limits, its errors) stays inside the driver.
2. **It is a library.** The standard library only, no route, no table, no
   `Module`, no `init`. `TestThePackageIsALibrary` holds it.
3. **The API key stays on the server.** It reaches the request that
   authenticates and nothing else: not an error, not a formatted configuration.
4. **`arandu.mod.toml` matches the code.** Adding a file write or a process
   means declaring it there in the same commit.

A new provider is a new driver in its own file with its own tests, written from
the provider's public documentation. A response is recorded by hand into a test,
never captured from the network with a real key.

## Style

Everything in the source is in English: identifiers, doc comments, internal
comments, error messages, log messages, and the names and messages of tests.
`pkg.go.dev` publishes the doc comments, and its readers are users of this
package.

Every exported symbol carries a doc comment saying what it does. The comment
documents the symbol and nothing else: no dates, no issue numbers, no
references to other repositories. Why a signature is what it is belongs there
when it is a fact about the code; anything else belongs in the pull request.

## Commits

One commit per logical change. Formatting is not mixed with behaviour, because
a diff that does both is a diff nobody reviews.

## Reporting a vulnerability

Do not open an issue. See [SECURITY.md](SECURITY.md).

## Licence

By contributing you agree that your contribution is licensed under the MIT
licence, the same as the rest of github.com/hyz-is/arandu-translate.
