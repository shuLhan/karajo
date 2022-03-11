## SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
## SPDX-License-Identifier: GPL-3.0-or-later

.PHONY: all build test run serve-doc
.FORCE:

all: build test

memfs_www.go: .FORCE
	go run ./internal/cmd/gen-www

build: memfs_www.go
	go build ./cmd/karajo

test:
	CGO_ENABLED=1 go test -race ./...

run:
	CGO_ENABLED=1 KARAJO_DEVELOPMENT=1 go run -race ./cmd/karajo -config karajo_test.conf

${GOBIN}/mdgo:
	go install git.sr.ht/~shulhan/mdgo/cmd/mdgo

serve-doc: ${GOBIN}/mdgo
	mdgo -exclude="^_.*$$" serve .
