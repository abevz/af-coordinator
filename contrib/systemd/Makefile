.PHONY: install uninstall

SERVICE_FILE = af-coordinatord.service
SERVICE_DIR = $(HOME)/.config/systemd/user

install:
	@mkdir -p $(SERVICE_DIR)
	cp $(SERVICE_FILE) $(SERVICE_DIR)/
	systemctl --user daemon-reload
	@echo "Service installed. Run: systemctl --user enable --now af-coordinatord"

uninstall:
	systemctl --user stop af-coordinatord 2>/dev/null || true
	systemctl --user disable af-coordinatord 2>/dev/null || true
	rm -f $(SERVICE_DIR)/$(SERVICE_FILE)
	systemctl --user daemon-reload
	@echo "Service uninstalled."
