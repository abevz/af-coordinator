.PHONY: install uninstall

SERVICE_FILE = af-coordinatord.service
SERVICE_DIR = $(HOME)/.config/systemd/user
SYSTEMCTL_USER ?= sh ../install/systemctl-user.sh

install:
	@mkdir -p $(SERVICE_DIR)
	cp $(SERVICE_FILE) $(SERVICE_DIR)/
	$(SYSTEMCTL_USER) daemon-reload
	@echo "Service installed. Run: $(SYSTEMCTL_USER) enable --now af-coordinatord"

uninstall:
	-$(SYSTEMCTL_USER) stop af-coordinatord
	-$(SYSTEMCTL_USER) disable af-coordinatord
	rm -f $(SERVICE_DIR)/$(SERVICE_FILE)
	$(SYSTEMCTL_USER) daemon-reload
	@echo "Service uninstalled."
