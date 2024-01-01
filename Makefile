.DEFAULT_GOAL := compile

NOTES_API_PORT ?= 1111
INSTALL_DIR ?= ~/bin

CMD_DIR = $(CURDIR)/cmd
PACKAGE_DIR = $(CURDIR)/build/package

compile:
	go build -o $(CMD_DIR)/notes-api $(CMD_DIR)/notes-api.go

integration:
	go run $(CMD_DIR)/notes-test.go

install:
	mkdir -p $(INSTALL_DIR)
	cp $(CMD_DIR)/notes-api $(INSTALL_DIR)/notes-api
	echo "==> Registering as local service running on port: $(NOTES_API_PORT)"
	cat $(PACKAGE_DIR)/notes-api.service | sed s/NOTES_API_PORT=[0-9]*/NOTES_API_PORT=$(NOTES_API_PORT)/g >/usr/lib/systemd/system/notes-api.service
	systemctl daemon-reload
	systemctl restart notes-api

.PHONY: compile integration install
