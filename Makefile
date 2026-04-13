BINDIR     := bin
INSTALLDIR := $(HOME)/.local/bin
SERVICEDIR := $(HOME)/.config/systemd/user

VERSION    := $(shell dpkg-parsechangelog -S Version | cut -d- -f1)
ARCH       := $(shell dpkg --print-architecture)
PKGNAME    := clipd_$(VERSION)_$(ARCH)
PKGDIR     := $(BINDIR)/deb/$(PKGNAME)
PPA        := ppa:kantha2004/clipd

.PHONY: build install install-services enable uninstall deb vendor source ppa clean

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

deb: build
	rm -rf $(PKGDIR)
	mkdir -p $(PKGDIR)/usr/local/bin
	mkdir -p $(PKGDIR)/usr/lib/systemd/user
	mkdir -p $(PKGDIR)/DEBIAN
	cp $(BINDIR)/clipd $(PKGDIR)/usr/local/bin/clipd
	sed 's|%h/.local/bin/clipd|/usr/local/bin/clipd|' \
		systemd/clipd.service > $(PKGDIR)/usr/lib/systemd/user/clipd.service
	cp systemd/ydotoold.service $(PKGDIR)/usr/lib/systemd/user/ydotoold.service
	sed "s/VERSION/$(VERSION)/; s/ARCH/$(ARCH)/" \
		packaging/DEBIAN/control > $(PKGDIR)/DEBIAN/control
	install -m 755 packaging/DEBIAN/postinst $(PKGDIR)/DEBIAN/postinst
	install -m 755 packaging/DEBIAN/prerm    $(PKGDIR)/DEBIAN/prerm
	fakeroot dpkg-deb --build $(PKGDIR) $(BINDIR)/$(PKGNAME).deb
	@echo ""
	@echo "Built: $(BINDIR)/$(PKGNAME).deb"
	@echo "Install with: sudo dpkg -i $(BINDIR)/$(PKGNAME).deb"

# Vendor all Go dependencies so the PPA build has no internet access needed.
vendor:
	go mod tidy
	go mod vendor

# Build a signed source package for upload to Launchpad PPA.
# Requires: devscripts, debhelper, gpg key matching debian/changelog maintainer.
source: vendor
	tar --exclude=.git --exclude=bin -czf ../clipd_$(VERSION).orig.tar.gz .
	debuild -S -sa

# Upload the signed source package to Launchpad.
# Run 'make source' first, then 'make ppa'.
ppa:
	dput $(PPA) ../clipd_$(VERSION)-1_source.changes

clean:
	rm -rf $(BINDIR)
	rm -rf vendor
