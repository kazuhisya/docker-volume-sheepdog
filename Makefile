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
	yum install -y rpm-build git redhat-rpm-config

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

.PHONY: compile deps fmt clean rpm-deps rpm
