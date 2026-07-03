GO ?= go
BINDIR ?= $(HOME)/.local/bin
BACKUPDIR ?= $(HOME)/backups/af-coordinator

.PHONY: fmt build test build-install install-service uninstall-service

fmt:
	gofmt -w cmd internal

build:
	$(GO) build ./...

build-install:
	$(GO) build -o $(BINDIR)/af-coordinatord ./cmd/af-coordinatord/
	$(GO) build -o $(BINDIR)/afctl ./cmd/afctl/

test:
	$(GO) test ./...

install-service:
	@mkdir -p $(HOME)/.config/systemd/user
	cp contrib/systemd/af-coordinatord.service $(HOME)/.config/systemd/user/
	systemctl --user daemon-reload
	@echo "Service installed. Enable: systemctl --user enable --now af-coordinatord"

uninstall-service:
	-systemctl --user stop af-coordinatord
	-systemctl --user disable af-coordinatord
	rm -f $(HOME)/.config/systemd/user/af-coordinatord.service
	systemctl --user daemon-reload
