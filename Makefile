# VERSION is set at release time (e.g. VERSION=v1.0.0 make release).
# Artifacts go to dist/ for upload to GitHub Releases; install.sh and install.ps1 require checksums.txt.
VERSION ?= dev
LDFLAGS = -ldflags "-s -w -X stet/cli/internal/version.Version=$(VERSION)"

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
	GOOS=linux GOARCH=arm64 go build -buildvcs=false $(BUILD_LDFLAGS) -o bin/stet-linux-arm64 ./cli/cmd/stet
	GOOS=darwin GOARCH=amd64 go build -buildvcs=false $(BUILD_LDFLAGS) -o bin/stet-darwin-amd64 ./cli/cmd/stet
	GOOS=darwin GOARCH=arm64 go build -buildvcs=false $(BUILD_LDFLAGS) -o bin/stet-darwin-arm64 ./cli/cmd/stet

# Build all release binaries into dist/ and generate checksums.txt.
# The for-loop is a single shell recipe, so builds run sequentially even
# under make -j.  This avoids races in the shared Go module/build cache.
# CGO_ENABLED=0 keeps Linux binaries static when built on a Linux CI runner
# (cgo is otherwise on for native builds and would link against glibc).
# Run: VERSION=v1.0.0 make release
# The release workflow (.github/workflows/release.yml) uploads dist/* on tag push.
release:
	@case "$(VERSION)" in \
		v[0-9]*.[0-9]*.[0-9]*) ;; \
		*) echo "VERSION must be vX.Y.Z (got '$(VERSION)')" >&2; exit 1 ;; \
	esac
	rm -rf dist && mkdir -p dist
	@for p in $(RELEASE_PLATFORMS); do \
		GOOS=$${p%/*}; GOARCH=$${p#*/}; ext=""; \
		if [ "$$GOOS" = "windows" ]; then ext=".exe"; fi; \
		CGO_ENABLED=0 GOOS=$$GOOS GOARCH=$$GOARCH go build -trimpath -buildvcs=false $(LDFLAGS) \
			-o dist/stet-$$GOOS-$$GOARCH$$ext ./cli/cmd/stet || exit 1; \
	done
	@(cd dist && (sha256sum stet-* 2>/dev/null || shasum -a 256 stet-*) > checksums.txt)

clean:
	rm -f bin/stet bin/stet-* coverage.out
	# dist/ contains release binaries (stet-{os}-{arch}); rm -rf removes all
	rm -rf dist

test:
	go test ./cli/... -count=1

coverage: coverage.out
	@bash scripts/check-coverage.sh coverage.out

coverage.out:
	go test ./cli/... -coverprofile=coverage.out -count=1
