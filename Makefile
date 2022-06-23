## SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
## SPDX-License-Identifier: GPL-3.0-or-later

.PHONY: all lint build test run serve-doc
.FORCE:

all: lint build test

memfs_www.go: .FORCE
	go run ./cmd/karajo embed

lint:
	-fieldalignment ./...
	-golangci-lint run ./...
	-reuse lint --quiet

build: memfs_www.go
	go build ./cmd/karajo

test:
	CGO_ENABLED=1 go test -race ./...
	fieldalignment ./...

dev:
	CGO_ENABLED=1 go run -race ./cmd/karajo -dev -config testdata/karajo.conf

${GOBIN}/mdgo:
	go install git.sr.ht/~shulhan/mdgo/cmd/mdgo

serve-doc: ${GOBIN}/mdgo
	mdgo -exclude="^_.*$$" serve .
