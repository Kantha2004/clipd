BINDIR     := bin
INSTALLDIR := $(HOME)/.local/bin
LIBDIR     := $(HOME)/.local/lib/clipd
SERVICEDIR := $(HOME)/.config/systemd/user

VERSION    := $(shell dpkg-parsechangelog -S Version 2>/dev/null | cut -d- -f1)
VERSION    := $(or $(VERSION),$(shell grep -m1 '^clipd (' debian/changelog | sed 's/clipd (\([^-]*\).*/\1/'))
ARCH       := $(shell dpkg --print-architecture 2>/dev/null || uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
PKGNAME    := clipd_$(VERSION)_$(ARCH)
PKGDIR     := $(BINDIR)/deb/$(PKGNAME)
PPA        := ppa:kantha2004/clipd

.PHONY: build install install-services enable uninstall deb vendor source ppa clean

build:
	mkdir -p $(BINDIR)
	go build -o $(BINDIR)/clipd .

install: build install-services
	mkdir -p $(INSTALLDIR) $(LIBDIR)
	cp $(BINDIR)/clipd $(INSTALLDIR)/clipd
	install -m 755 packaging/setup-shortcut    $(LIBDIR)/setup-shortcut
	install -m 755 packaging/teardown-shortcut $(LIBDIR)/teardown-shortcut

install-services:
	mkdir -p $(SERVICEDIR)
	sed 's|%h/.local/bin/clipd|$(INSTALLDIR)/clipd|' \
		systemd/clipd.service > $(SERVICEDIR)/clipd.service
	cp systemd/ydotoold.service $(SERVICEDIR)/ydotoold.service
	systemctl --user daemon-reload

enable: install
	systemctl --user enable --now ydotoold
	systemctl --user enable --now clipd
	@$(LIBDIR)/setup-shortcut $(INSTALLDIR)/clipd || true

uninstall:
	systemctl --user disable --now clipd 2>/dev/null || true
	systemctl --user disable --now ydotoold 2>/dev/null || true
	rm -f $(SERVICEDIR)/clipd.service $(SERVICEDIR)/ydotoold.service
	-$(LIBDIR)/teardown-shortcut
	rm -f $(INSTALLDIR)/clipd
	rm -rf $(LIBDIR)
	systemctl --user daemon-reload

deb: build
	rm -rf $(PKGDIR)
	mkdir -p $(PKGDIR)/usr/local/bin
	mkdir -p $(PKGDIR)/usr/lib/systemd/user
	mkdir -p $(PKGDIR)/usr/lib/clipd
	mkdir -p $(PKGDIR)/etc/xdg/autostart
	mkdir -p $(PKGDIR)/DEBIAN
	cp $(BINDIR)/clipd $(PKGDIR)/usr/local/bin/clipd
	sed 's|%h/.local/bin/clipd|/usr/local/bin/clipd|' \
		systemd/clipd.service > $(PKGDIR)/usr/lib/systemd/user/clipd.service
	cp systemd/ydotoold.service $(PKGDIR)/usr/lib/systemd/user/ydotoold.service
	install -m 755 packaging/setup-shortcut    $(PKGDIR)/usr/lib/clipd/setup-shortcut
	install -m 755 packaging/teardown-shortcut $(PKGDIR)/usr/lib/clipd/teardown-shortcut
	cp packaging/clipd-shortcut-setup.desktop  $(PKGDIR)/etc/xdg/autostart/clipd-shortcut-setup.desktop
	sed "s/VERSION/$(VERSION)/; s/ARCH/$(ARCH)/" \
		packaging/DEBIAN/control > $(PKGDIR)/DEBIAN/control
	install -m 755 debian/postinst $(PKGDIR)/DEBIAN/postinst
	install -m 755 debian/prerm    $(PKGDIR)/DEBIAN/prerm
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
	# Create the .orig.tar.gz, excluding git, bin, and debian/ (standard for 3.0 quilt).
	# We include vendor/ so the source package is self-contained.
	tar --exclude=.git --exclude=bin --exclude=debian -czf ../clipd_$(VERSION).orig.tar.gz .
	# -i: ignore changes in .git and other repo metadata in the diff.
	# -I: ignore .git and other repo metadata when building the tarball.
	debuild -S -sa -i -I

# Upload the signed source package to Launchpad.
# Run 'make source' first, then 'make ppa'.
ppa:
	dput $(PPA) ../clipd_$(VERSION)-1_source.changes

clean:
	rm -rf $(BINDIR)
