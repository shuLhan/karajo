## SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
## SPDX-License-Identifier: GPL-3.0-or-later

.PHONY: all lint build test dev
.FORCE:

all: test build lint

memfs_www.go: .FORCE
	go run ./cmd/karajo embed

lint:
	-golangci-lint run ./...
	-fieldalignment ./...
	-reuse lint

build: memfs_www.go
	go build ./cmd/karajo

test:
	CGO_ENABLED=1 go test -race ./...

dev:
	CGO_ENABLED=1 go run -race ./cmd/karajo -dev -config testdata/karajo.conf
