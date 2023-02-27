## SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
## SPDX-License-Identifier: GPL-3.0-or-later

.PHONY: all lint build test run-example
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
	CGO_ENABLED=1 go test -race -coverprofile cover.out ./...
	go tool cover -html=cover.out -o cover.html

run-example:
	CGO_ENABLED=1 go run -race ./internal/cmd/karajo-example
