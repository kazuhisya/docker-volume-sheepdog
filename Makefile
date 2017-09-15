NAME := docker-volume-sheepdog
TODAY := $(shell LANG=c date +"%a %b %e %Y")
GIT_COMMIT := $(shell git show -s --format=%H)
GIT_USER := $(shell git config user.name)
GIT_EMAIL := $(shell git config user.email)
MAINTAINER := $(GIT_USER) <$(GIT_EMAIL)>
DVPSD_VERSION := $(shell grep "VERSION =" main.go  | tr -s " "| cut -d " " -f 3 | sed "s/\"//g")


all: deps compile

compile:
	go build

deps:
	go get

fmt:
	gofmt -s -w -l .

clean:
	rm -fr dist $(NAME)

rpm-deps:
	yum install -y rpm-build go git redhat-rpm-config

rpm: deps compile rpm-deps
	mkdir -p dist/{BUILD,RPMS,SPECS,SOURCES,SRPMS,install}
	cat etc/$(NAME).spec.template | \
		GIT_COMMIT="$(GIT_COMMIT)" \
		MAINTAINER="$(MAINTAINER)" \
		DVPSD_VERSION="$(DVPSD_VERSION)" \
		TODAY="$(TODAY)" \
		envsubst '$$GIT_COMMIT, $$MAINTAINER, $$DVPSD_VERSION, $$TODAY' > dist/SPECS/$(NAME).spec
	cp $(NAME) dist/SOURCES/
	cp etc/$(NAME).service dist/SOURCES/
	cp etc/dockerdriver.json dist/SOURCES/
	cp README.md dist/SOURCES/
	cp LICENSE dist/SOURCES/
	rpmbuild -ba \
		--define "_topdir $(PWD)/dist" \
		--define "buildroot $(PWD)/dist/install" \
		--clean \
		dist/SPECS/$(NAME).spec

# for RHEL based system
# if you want to do this on a debian based system:
#   apt-get install -y ruby ruby-dev gcc golang git make
deb-deps:
	yum install -y go git ruby ruby-devel rubygems
	gem install fpm

deb: deps compile deb-deps
	mkdir -p dist/debian/usr/sbin
	mkdir -p dist/debian/lib/systemd/system/
	mkdir -p dist/debian/etc/sheepdog
	mkdir -p dist/debian/usr/share/doc/docker-volume-sheepdog
	install -m 0755 docker-volume-sheepdog dist/debian/usr/sbin
	install -m 0644 etc/docker-volume-sheepdog.service dist/debian/lib/systemd/system
	install -m 0644 etc/dockerdriver.json dist/debian/etc/sheepdog
	install -m 0644 README.md dist/debian/usr/share/doc/docker-volume-sheepdog
	install -m 0644 LICENSE dist/debian/usr/share/doc/docker-volume-sheepdog
	fpm -C dist/debian -m "khara@sios.com" -f \
		-s dir -t deb -n docker-volume-sheepdog \
		--license "MIT" --vendor "N/A" \
		--url "https://github.com/kazuhisya/docker-volume-sheepdog" \
		--description "Docker Volume Plugin for Sheepdog" \
		-d tgt -d open-iscsi -d xfsprogs \
		--version $(DVPSD_VERSION) .
	rm -rf dist/debian

.PHONY: compile deps fmt clean rpm-deps rpm deb-deps deb
