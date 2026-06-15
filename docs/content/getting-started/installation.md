---
title: "Installation"
description: "Install dbvar from a release, with go install, or from source."
weight: 20
---

## Prebuilt binaries

Every [release](https://github.com/tamnd/dbvar-cli/releases) carries archives for Linux, macOS,
and Windows on amd64 and arm64, plus deb, rpm, and apk packages for Linux.
Download, unpack, put `dbvar` on your `PATH`, done. The `checksums.txt`
on each release is signed with keyless [cosign](https://docs.sigstore.dev/) if
you want to verify before running.

## With Go

```bash
go install github.com/tamnd/dbvar-cli/cmd/dbvar@latest
```

That puts `dbvar` in `$(go env GOPATH)/bin`, which is `~/go/bin` unless
you moved it. Make sure that directory is on your `PATH`.

## From source

```bash
git clone https://github.com/tamnd/dbvar-cli
cd dbvar-cli
make build        # produces ./bin/dbvar
./bin/dbvar version
```

## Container image

```bash
docker run --rm ghcr.io/tamnd/dbvar:latest --help
```

## Checking the install

```bash
dbvar version
```

prints the version and exits.
