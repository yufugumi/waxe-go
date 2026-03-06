VERSION ?= dev
LDFLAGS := -s -w -X main.version=$(VERSION)
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

.PHONY: build release clean

build:
	go build -ldflags="$(LDFLAGS)" -o axel ./cmd/scanner

release: clean
	@mkdir -p dist
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		output="dist/axel-$${os}-$${arch}"; \
		if [ "$${os}" = "windows" ]; then output="$${output}.exe"; fi; \
		printf "Building %s...\n" "$${output}"; \
		GOOS=$${os} GOARCH=$${arch} CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o "$${output}" ./cmd/scanner; \
	done
	@cd dist && sha256sum * > checksums.txt
	@printf "Built %s binaries in dist/\n" "$(words $(PLATFORMS))"

clean:
	rm -rf dist
