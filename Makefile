GO ?= go
BINDIR ?= $(HOME)/.local/bin
BACKUPDIR ?= $(HOME)/backups/af-coordinator

.PHONY: preflight fmt vet lint build test build-install install-service uninstall-service install-backup uninstall-backup

preflight:
	sh contrib/install/check-deps.sh

fmt:
	gofmt -w cmd internal

vet:
	$(GO) vet ./...

lint:
	golangci-lint run --disable errcheck,staticcheck

build:
	$(GO) build -buildvcs=false ./...

build-install:
	@mkdir -p $(BINDIR)
	$(GO) build -buildvcs=false -o $(BINDIR)/af-coordinatord ./cmd/af-coordinatord/
	$(GO) build -buildvcs=false -o $(BINDIR)/afctl ./cmd/afctl/
	$(GO) build -buildvcs=false -o $(BINDIR)/afc-mcp ./cmd/afc-mcp/

test:
	$(GO) test -race ./...

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

install-backup:
	@mkdir -p $(HOME)/.config/systemd/user
	@mkdir -p $(BINDIR)
	install -m 755 contrib/systemd/af-coordinator-backup.sh $(BINDIR)/af-coordinator-backup.sh
	cp contrib/systemd/af-coordinator-backup.service $(HOME)/.config/systemd/user/
	cp contrib/systemd/af-coordinator-backup.timer $(HOME)/.config/systemd/user/
	systemctl --user daemon-reload
	@echo "Backup service installed. Enable: systemctl --user enable --now af-coordinator-backup.timer"

uninstall-backup:
	-systemctl --user stop af-coordinator-backup.timer
	-systemctl --user disable af-coordinator-backup.timer
	rm -f $(HOME)/.config/systemd/user/af-coordinator-backup.service
	rm -f $(HOME)/.config/systemd/user/af-coordinator-backup.timer
	rm -f $(BINDIR)/af-coordinator-backup.sh
	systemctl --user daemon-reload
