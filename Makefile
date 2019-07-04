PKG := github.com/the-lightning-land/sweetd

GO_BIN := ${GOPATH}/bin
VERSION := $(shell git describe --tags)
COMMIT := $(shell git rev-parse HEAD)
DATE := $(shell date +%Y-%m-%d)

LDFLAGS := "-X main.Commit=$(COMMIT) -X main.Version=$(VERSION) -X main.Date=$(DATE)"

GOBUILD := GO111MODULE=on go build
RM := rm -f

# commands

default: build

compile:
	@$(call print, "Getting node dependencies.")
	(cd pos && npm install)
	@$(call print, "Compiling point-of-sale assets.")
	(cd pos && npm run export)
	@$(call print, "Getting dependencies.")
	go get github.com/gobuffalo/packr/v2/...
	@$(call print, "Packaging static assets.")
	packr2
	@$(call print, "Building sweetd.")
	$(GOBUILD) -o sweetd -ldflags $(LDFLAGS) $(PKG)

test:
	@$(call print, "Testing sweetd.")
	go test -v ./...

clean:
	@$(call print, "Cleaning static asset packages.")
	packr2 clean
	@$(call print, "Cleaning builds and module cache")
	$(RM) ./sweetd

clean-cache:
	@$(call print, "Cleaning go module cache")
	go clean --modcache

build: compile
