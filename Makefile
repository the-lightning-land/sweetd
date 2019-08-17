PKG := github.com/the-lightning-land/sweetd
VERSION := $(shell git describe --tags)
COMMIT := $(shell git rev-parse HEAD)
DATE := $(shell date +%Y-%m-%d)
LDFLAGS := '-ldflags="-X main.Commit=$(COMMIT) -X main.Version=$(VERSION) -X main.Date=$(DATE)"'

default: build

get:
	@$(call print, "Getting dependencies.")
	go get

packr2:
	@$(call print, "Installing packr2.")
	go install github.com/gobuffalo/packr/v2/packr2

pos/node_modules: pos/package.json pos/package-lock.json
	@$(call print, "Getting node dependencies.")
	(cd pos && npm install)

npm-export: pos/node_modules pos/components/*.js
	@$(call print, "Compiling point-of-sale assets.")
	(cd pos && npm run export)

pack: npm-export
	@$(call print, "Packaging static assets.")
	(cd pos && packr2)

build: pack
	@$(call print, "Building sweetd.")
	go build $(LDFLAGS) $(GOBUILDFLAGS) -o sweetd $(PKG)

test:
	@$(call print, "Testing sweetd.")
	go test -v ./...

clean:
	@$(call print, "Cleaning static asset packages.")
	(cd pos && packr2 clean)
	@$(call print, "Cleaning builds and module cache")
	rm -rf ./sweetd

clean-cache:
	@$(call print, "Cleaning go module cache")
	go clean --modcache
