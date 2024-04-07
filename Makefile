## SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
## SPDX-License-Identifier: GPL-3.0-or-later

.PHONY: all lint build test run-example
.FORCE:

all: test build lint

memfs_www.go: .FORCE
	go run ./internal/cmd/karajo-build embed

lint:
	-revive ./...
	-fieldalignment ./...
	-shadow ./...

build: memfs_www.go
	go build ./cmd/karajo

test:
	CGO_ENABLED=1 go test -race -timeout 10s -coverprofile cover.out ./...
	go tool cover -html=cover.out -o cover.html

run-example:
	CGO_ENABLED=1 go run -race ./internal/cmd/karajo-example

##
## Install the karajo directly to GNU/Linux system.
## Usage:
##
##	$ sudo make PREFIX=/ install
##

.PHONY: install
install:
	install -dm0750 $(PREFIX)/etc/karajo
	install -dm0750 $(PREFIX)/etc/karajo/job.d
	install -dm0750 $(PREFIX)/srv/karajo
	install -dm0750 $(PREFIX)/var/lib/karajo
	install -dm0750 $(PREFIX)/var/log/karajo
	install -dm0750 $(PREFIX)/run/karajo

	install -Dm0755 karajo   $(PREFIX)/usr/bin/karajo
	install -Dm0644 COPYING  $(PREFIX)/usr/share/licenses/karajo/COPYING

	install -Dm0640 _sys/etc/karajo/karajo.conf  $(PREFIX)/etc/karajo/karajo.conf
	install -Dm0644 _sys/srv/karajo/index.html   $(PREFIX)/srv/karajo/index.html

	install -Dm0644 _sys/usr/lib/systemd/system/karajo.path                       $(PREFIX)/usr/lib/systemd/system/karajo.path
	install -Dm0644 _sys/usr/lib/systemd/system/karajo.service                    $(PREFIX)/usr/lib/systemd/system/karajo.service
	install -Dm0644 _sys/usr/lib/systemd/system/systemctl-restart-karajo@.service $(PREFIX)/usr/lib/systemd/system/systemctl-restart-karajo@.service
	install -Dm0644 _sys/usr/lib/tmpfiles.d/karajo.conf                           $(PREFIX)/usr/lib/tmpfiles.d/karajo.conf
	install -Dm0644 _sys/usr/lib/sysusers.d/karajo.conf                           $(PREFIX)/usr/lib/sysusers.d/karajo.conf

.PHONY: uninstall
uninstall:
	systemctl stop karajo
	systemctl disable karajo
	rm -f $(PREFIX)/usr/lib/sysusers.d/karajo.conf
	rm -f $(PREFIX)/usr/lib/tmpfiles.d/karajo.conf
	rm -f $(PREFIX)/usr/lib/systemd/system/systemctl-restart-karajo@.service
	rm -f $(PREFIX)/usr/lib/systemd/system/karajo.service
	rm -f $(PREFIX)/usr/lib/systemd/system/karajo.path
	rm -f $(PREFIX)/usr/share/licenses/karajo/COPYING
	rm -f $(PREFIX)/usr/bin/karajo

## Deploy karajo to internal server.
.PHONY: deploy-kilabit
deploy-kilabit: build
	rsync --progress karajo build.kilabit.info:/tmp/karajo
	ssh build.kilabit.info "sudo mv /tmp/karajo /usr/bin/karajo"
	ssh build.kilabit.info "systemctl status karajo"
