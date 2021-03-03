## Copyright 2021, Shulhan <ms@kilabit.info>. All rights reserved.
## Use of this source code is governed by a BSD-style license that can be
## found in the LICENSE file.

.PHONY: build test run

all: build test

memfs_www.go:
	go run ./internal/cmd/gen-www

build: memfs_www.go
	go build ./cmd/karajo

test:
	go test -race ./...

run:
	DEBUG=2 go run -race ./cmd/karajo -config karajo_test.conf
