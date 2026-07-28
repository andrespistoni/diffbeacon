VERSION ?=
VERSION := $(strip $(VERSION))
BINARY := diffbeacon
BIN_DIR := bin
RAW_DIR := build/release
PACKAGE_DIR := build/packages
DIST_DIR := dist
LDFLAGS := -X main.version=$(VERSION) -s -w

RAW_ARTIFACTS := \
	$(RAW_DIR)/$(BINARY)_darwin_amd64 \
	$(RAW_DIR)/$(BINARY)_darwin_arm64 \
	$(RAW_DIR)/$(BINARY)_linux_amd64 \
	$(RAW_DIR)/$(BINARY)_linux_arm64 \
	$(RAW_DIR)/$(BINARY)_windows_amd64.exe \
	$(RAW_DIR)/$(BINARY)_windows_arm64.exe

ARCHIVES := \
	$(DIST_DIR)/$(BINARY)_$(VERSION)_darwin_amd64.tar.gz \
	$(DIST_DIR)/$(BINARY)_$(VERSION)_darwin_arm64.tar.gz \
	$(DIST_DIR)/$(BINARY)_$(VERSION)_linux_amd64.tar.gz \
	$(DIST_DIR)/$(BINARY)_$(VERSION)_linux_arm64.tar.gz \
	$(DIST_DIR)/$(BINARY)_$(VERSION)_windows_amd64.zip \
	$(DIST_DIR)/$(BINARY)_$(VERSION)_windows_arm64.zip

.PHONY: validate-version build build-all dist install uninstall release-metadata test test-e2e test-race test-performance benchmark test-git-matrix vet govulncheck check release-check clean

validate-version:
	@if [ -z "$(VERSION)" ]; then \
		echo "ERROR: VERSION is missing or empty; expected MAJOR.MINOR.PATCH (for example 0.1.0)" >&2; \
		exit 1; \
	fi
	@if ! printf '%s\n' "$(VERSION)" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$$'; then \
		echo "ERROR: VERSION '$(VERSION)' is not SemVer without a v prefix" >&2; \
		exit 1; \
	fi

build:
	@mkdir -p "$(BIN_DIR)"
	CGO_ENABLED=0 go build -trimpath -o "$(BIN_DIR)/$(BINARY)" ./cmd/diffbeacon

build-all: validate-version
	@rm -rf "$(RAW_DIR)"
	@mkdir -p "$(RAW_DIR)"
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o "$(RAW_DIR)/$(BINARY)_darwin_amd64" ./cmd/diffbeacon
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o "$(RAW_DIR)/$(BINARY)_darwin_arm64" ./cmd/diffbeacon
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o "$(RAW_DIR)/$(BINARY)_linux_amd64" ./cmd/diffbeacon
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o "$(RAW_DIR)/$(BINARY)_linux_arm64" ./cmd/diffbeacon
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o "$(RAW_DIR)/$(BINARY)_windows_amd64.exe" ./cmd/diffbeacon
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o "$(RAW_DIR)/$(BINARY)_windows_arm64.exe" ./cmd/diffbeacon
	@for artifact in $(RAW_ARTIFACTS); do go version -m "$$artifact" >/dev/null; done

release-metadata: build-all
	DIFFBEACON_RELEASE_VERSION="$(VERSION)" go run ./scripts/release-metadata "$(RAW_DIR)"
	@cd "$(RAW_DIR)" && if command -v sha256sum >/dev/null 2>&1; then sha256sum --check SHA256SUMS; else shasum -a 256 -c SHA256SUMS; fi

dist: release-metadata
	@command -v tar >/dev/null 2>&1 || { echo "ERROR: tar is required to package Unix builds" >&2; exit 1; }
	@command -v zip >/dev/null 2>&1 || { echo "ERROR: zip is required to package Windows builds" >&2; exit 1; }
	@rm -rf "$(DIST_DIR)" "$(PACKAGE_DIR)"
	@mkdir -p "$(DIST_DIR)" "$(PACKAGE_DIR)"
	@set -eu; \
		dist_dir=$$(cd "$(DIST_DIR)" && pwd); \
		package_unix() { \
			platform=$$1; arch=$$2; staging="$(PACKAGE_DIR)/$${platform}_$${arch}"; \
			mkdir -p "$$staging"; \
			cp "$(RAW_DIR)/$(BINARY)_$${platform}_$${arch}" "$$staging/$(BINARY)"; \
			cp scripts/install.sh scripts/uninstall.sh LICENSE "$$staging/"; \
			chmod 0755 "$$staging/$(BINARY)" "$$staging/install.sh" "$$staging/uninstall.sh"; \
			tar -C "$$staging" -czf "$$dist_dir/$(BINARY)_$(VERSION)_$${platform}_$${arch}.tar.gz" $(BINARY) install.sh uninstall.sh LICENSE; \
		}; \
		package_windows() { \
			arch=$$1; staging="$(PACKAGE_DIR)/windows_$${arch}"; \
			mkdir -p "$$staging"; \
			cp "$(RAW_DIR)/$(BINARY)_windows_$${arch}.exe" "$$staging/$(BINARY).exe"; \
			cp scripts/install.ps1 scripts/uninstall.ps1 LICENSE "$$staging/"; \
			(cd "$$staging" && zip -q "$$dist_dir/$(BINARY)_$(VERSION)_windows_$${arch}.zip" $(BINARY).exe install.ps1 uninstall.ps1 LICENSE); \
		}; \
		package_unix darwin amd64; \
		package_unix darwin arm64; \
		package_unix linux amd64; \
		package_unix linux arm64; \
		package_windows amd64; \
		package_windows arm64
	@cp "$(RAW_DIR)/diffbeacon.spdx.json" "$(RAW_DIR)/provenance.intoto.json" "$(DIST_DIR)/"
	@cd "$(DIST_DIR)" && if command -v sha256sum >/dev/null 2>&1; then \
		sha256sum $(notdir $(ARCHIVES)) diffbeacon.spdx.json provenance.intoto.json > SHA256SUMS; \
	else \
		shasum -a 256 $(notdir $(ARCHIVES)) diffbeacon.spdx.json provenance.intoto.json > SHA256SUMS; \
	fi
	@cd "$(DIST_DIR)" && if command -v sha256sum >/dev/null 2>&1; then sha256sum --check SHA256SUMS; else shasum -a 256 -c SHA256SUMS; fi

install:
	./scripts/install.sh

uninstall:
	./scripts/uninstall.sh

test:
	go test ./...

test-e2e:
	go test ./test/e2e -count=1

test-race:
	go test -race ./...

test-performance:
	go test ./test/performance -count=1

benchmark:
	./scripts/benchmark-refresh.sh

test-git-matrix:
	./scripts/test-git-matrix.sh

vet:
	go vet ./...

govulncheck:
	govulncheck ./...

check: vet test test-e2e test-race govulncheck build

release-check: check test-performance dist

clean:
	rm -rf "$(BIN_DIR)" build "$(DIST_DIR)"
