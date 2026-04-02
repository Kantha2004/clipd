BINDIR     := bin
INSTALLDIR := $(HOME)/.local/bin
SERVICEDIR := $(HOME)/.config/systemd/user

.PHONY: build install install-services enable uninstall clean

build:
	mkdir -p $(BINDIR)
	go build -o $(BINDIR)/clipd .

install: build install-services
	mkdir -p $(INSTALLDIR)
	cp $(BINDIR)/clipd $(INSTALLDIR)/clipd

install-services:
	mkdir -p $(SERVICEDIR)
	sed 's|%h/.local/bin/clipd|$(INSTALLDIR)/clipd|' \
		systemd/clipd.service > $(SERVICEDIR)/clipd.service
	cp systemd/ydotoold.service $(SERVICEDIR)/ydotoold.service
	systemctl --user daemon-reload

enable: install
	systemctl --user enable --now ydotoold
	systemctl --user enable --now clipd

uninstall:
	systemctl --user disable --now clipd 2>/dev/null || true
	systemctl --user disable --now ydotoold 2>/dev/null || true
	rm -f $(SERVICEDIR)/clipd.service $(SERVICEDIR)/ydotoold.service
	rm -f $(INSTALLDIR)/clipd
	systemctl --user daemon-reload

clean:
	rm -rf $(BINDIR)
