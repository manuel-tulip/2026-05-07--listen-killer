BINARY  := listen-killer
CMD     := ./cmd/listen-killer
DIST    := dist

.PHONY: all build test vet fmt check clean build-darwin sign-darwin

all: check build

## build: native binary into ./bin
build:
	go build -o bin/$(BINARY) $(CMD)

## test: run the unit tests
test:
	go test ./...

## vet: run go vet
vet:
	go vet ./...

## fmt: fail if any touched file is not gofmt-clean
fmt:
	@files="$$(gofmt -l $$(git ls-files '*.go'))"; \
	if [ -n "$$files" ]; then echo "not gofmt-clean:"; echo "$$files"; exit 1; fi

check: vet test fmt

## build-darwin: build signed arm64 + amd64 binaries (run on a macOS host)
## CGO_ENABLED=1 keeps cwd/exe accurate via libproc. Ad-hoc signing stops
## Gatekeeper's "damaged" dialog for local use; public distribution needs a
## Developer ID + notarization.
build-darwin:
	mkdir -p $(DIST)
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -o $(DIST)/$(BINARY)-darwin-arm64 $(CMD)
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -o $(DIST)/$(BINARY)-darwin-amd64 $(CMD)
	@if [ "$$(uname)" = "Darwin" ]; then $(MAKE) sign-darwin; fi

## sign-darwin: ad-hoc codesign the Darwin binaries in ./dist
sign-darwin:
	@if [ "$$(uname)" = "Darwin" ]; then \
		for b in $(DIST)/$(BINARY)-darwin-*; do \
			[ -f "$$b" ] || continue; \
			codesign --force -s - "$$b" && echo "ad-hoc signed $$b"; \
		done; \
	else \
		echo "sign-darwin requires macOS"; exit 1; \
	fi

clean:
	rm -rf bin $(DIST)
