PREFIX := /usr/local
DATADIR := ${PREFIX}/share
MANDIR := $(DATADIR)/man
GO ?= go
GOMD2MAN ?= go-md2man
ifeq ($(shell uname -s),FreeBSD)
SED=gsed
else
SED=sed
endif
# This must never include the 'hack' directory
export PATH := $(shell $(GO) env GOPATH)/bin:$(PATH)

docs:  $(patsubst %.md,%,$(wildcard *[15].md))

%.1: %.1.md
### sed is used to filter http/s links as well as relative links
### replaces "\" at the end of a line with two spaces
### this ensures that manpages are rendered correctly
	@$(SED) -e 's/\((podman-bootc[^)]*\.md\(#.*\)\?)\)//g' \
	 -e 's/\[\(podman-bootc[^]]*\)\]/\1/g' \
	 -e 's/\[\([^]]*\)](http[^)]\+)/\1/g' \
	 -e 's;<\(/\)\?\(a\|a\s\+[^>]*\|sup\)>;;g' \
	 -e 's/\\$$/  /g' $<  | \
	$(GOMD2MAN) -in /dev/stdin -out  $@

%.5: %.5.md
### sed is used to filter http/s links as well as relative links
### replaces "\" at the end of a line with two spaces
### this ensures that manpages are rendered correctly
	@$(SED) -e 's/\((podman-bootc[^)]*\.md\(#.*\)\?)\)//g' \
	 -e 's/\[\(podman-bootc[^]]*\)\]/\1/g' \
	 -e 's/\[\([^]]*\)](http[^)]\+)/\1/g' \
	 -e 's;<\(/\)\?\(a\|a\s\+[^>]*\|sup\)>;;g' \
	 -e 's/\\$$/  /g' $<  | \
	$(GOMD2MAN) -in /dev/stdin -out  $@

.PHONY: install
install: docs
	install -d ${DESTDIR}/${MANDIR}/man1
	install -m 0644 podman-bootc*.1 ${DESTDIR}/${MANDIR}/man1
	install -m 0644 links/podman-bootc*.1 ${DESTDIR}/${MANDIR}/man1
	install -d ${DESTDIR}/${MANDIR}/man5
	install -m 0644 podman-bootc*.5 ${DESTDIR}/${MANDIR}/man5

	install -d ${DESTDIR}/${DATADIR}/podman-bootc
	install -m 0644 podman-bootc.conf ${DESTDIR}/${DATADIR}/podman-bootc


.PHONY: install-tools
install-tools:
	go install github.com/cpuguy83/go-md2man@latest

.PHONY: clean
clean:
	$(RM) -f podman-bootc*.1 podman-bootc*.5
