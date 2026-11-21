---
name: translate-release
description: Get this Arandu package green, declared and published. Use when the request is to "run the gates", "check it builds", "the tests fail", "go build says the directory prefix does not contain modules listed in go.work", "aru doctor says this is not an Arandu project", "update arandu.mod.toml", "add a dependency", "add a provider", "tag a release", "publish it", "go get cannot find the module", "write the changelog", or before opening a pull request. Covers the four gates, what the manifest declares and who checks it, why the module requires nothing, the immutability of a published module version, and what breaks somebody else's build without touching a signature.
license: MIT
---

# Getting it green and out

## The four gates

Nothing is finished until all four exit zero.

```sh
export GOWORK=off
gofmt -l $(find . -name '*.go' -not -path '*/testdata/*' -not -name '*.kyse.go')
go build ./...
go vet ./...
go test -race ./...
```

`GOWORK=off` first, and here it is not a habit. This checkout may sit beside a
Go workspace that lists the framework repositories and does not list this one.
When it does, `go build ./...` exits 1 before compiling anything:

```
pattern ./...: directory prefix . does not contain modules listed in go.work
or their selected dependencies
```

That is the workspace refusing to answer for a module it was not told about, and
it says nothing about the code. With the workspace off, the module resolves the
versions in `go.mod`, which is what CI compiles against and what somebody's
`go get` will get.

`aru doctor` is not a gate. It exits 1 here with *"this is not an Arandu
project"*: it reads applications, and this is a library.

CI runs the same four, and so does the release workflow on the tagged archive.

## What the manifest declares, and who reads it

```toml
name = "hyz-is/translate"
framework = ">= 0.71"
profiles = ["conventional", "performance"]

[permissions]
network = true
filesystem = false
exec = false
migrations = false
```

`TestTheDeclaredCapabilitiesAreWhatTheCodeDoes`, in `tests/Unit/audit_test.go`,
compares the four permissions against what the code *calls*, by AST. `network`
is `true` because the client calls the service; a file write or a process added
here is a line changed in `arandu.mod.toml` in the same commit.

## Adding a dependency

Do not. `TestTheModuleRequiresNothing` fails on any `require`: a client for a few
HTTP services needs the standard library, and a require is a download and an
audit surface for every project that installs the package.

## Publishing

A published Go module version is **immutable**. The proxy serves it forever, and
a mistake is corrected by another release, never by moving a tag.

Before tagging:

- the four gates pass with `GOWORK=off`;
- `arandu.mod.toml` matches the code;
- `CHANGELOG.md` has the entry under the version, with the date, and `UPGRADE.md`
  has a `## vX.Y.Z` heading for it. The release workflow publishes exactly that
  entry as the release notes and fails when there are none;
- `currentVersion` in `tests/Unit/release_test.go` is the version being tagged.

Tag with an annotated tag and push it; the release workflow creates the GitHub
release.

While the version starts with `v0.`, breaking is allowed. Breaking *quietly* is
not. A field that stops existing is a build that stops in somebody else's
repository, weeks later. Say what it was, what it is, and what they have to write
instead. CI runs `apidiff` against the last release and fails unless `UPGRADE.md`
names each removal.

Three changes break an installer without touching a signature, and each one is
worth a line in the changelog:

- **A code mapping.** Moving a BCP 47 tag to another provider code (`en` to
  `EN-GB`, `zh` to `ZH-HANS`) changes what an application gets back.
- **A default.** `DefaultTimeout`, the DeepL addresses and the default variants
  are what an application gets by leaving the field empty.
- **A refusal.** Reading another status as `ErrQuotaExceeded` or
  `ErrUnauthorized` changes what an application does when a provider fails.

## Reporting a vulnerability

Not in an issue and not in a pull request. `SECURITY.md` has the private
advisory address.
