# VERSION is set at release time (e.g. VERSION=v1.0.0 make release).
# Artifacts go to dist/ for upload to GitHub Releases; checksums.txt is optional for verification.
VERSION ?= dev
LDFLAGS = -ldflags "-X stet/cli/internal/version.Version=$(VERSION)"

# Tier-1 platforms for release: linux, darwin, windows × amd64, arm64.
RELEASE_PLATFORMS = linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

# COMMIT is the short (7-char) git hash for dev build traceability; used only for build target.
COMMIT := $(shell git rev-parse --short=7 HEAD 2>/dev/null || true)
BUILD_LDFLAGS := -ldflags "-X stet/cli/internal/version.Commit=$(COMMIT)"

.PHONY: build clean test coverage release
build:
	mkdir -p bin
	go build -buildvcs=false $(BUILD_LDFLAGS) -o bin/stet ./cli/cmd/stet
	GOOS=linux GOARCH=amd64 go build -buildvcs=false $(BUILD_LDFLAGS) -o bin/stet-linux-amd64 ./cli/cmd/stet
	GOOS=darwin GOARCH=amd64 go build -buildvcs=false $(BUILD_LDFLAGS) -o bin/stet-darwin-amd64 ./cli/cmd/stet

# Build all release binaries into dist/ and generate checksums.txt.
# Run: VERSION=v1.0.0 make release
# Then upload dist/* to a GitHub Release for that tag.
release:
	mkdir -p dist
	@for p in $(RELEASE_PLATFORMS); do \
		GOOS=$${p%/*}; GOARCH=$${p#*/}; \
		if [ "$$GOOS" = "windows" ]; then \
			GOOS=$$GOOS GOARCH=$$GOARCH go build -buildvcs=false $(LDFLAGS) -o dist/stet-$$GOOS-$$GOARCH.exe ./cli/cmd/stet; \
		else \
			GOOS=$$GOOS GOARCH=$$GOARCH go build -buildvcs=false $(LDFLAGS) -o dist/stet-$$GOOS-$$GOARCH ./cli/cmd/stet; \
		fi; \
	done
	@(cd dist && (sha256sum stet-* 2>/dev/null || shasum -a 256 stet-*) > checksums.txt)

clean:
	rm -f bin/stet bin/stet-linux-amd64 bin/stet-darwin-amd64 coverage.out
	rm -rf dist

test:
	go test ./cli/... -count=1

coverage: coverage.out
	@bash scripts/check-coverage.sh coverage.out

coverage.out:
	go test ./cli/... -coverprofile=coverage.out -count=1
