# input-go — build the ObjC companion dylib (universal arm64+x86_64),
# then Go tests & CLI.
#
# Requirements:
#   - macOS 12+ (CGEvent is stable across versions)
#   - Xcode Command Line Tools (for clang + frameworks)
#   - Go 1.22+
#
# The dylib is committed at internal/dylib/libinput_sync.dylib so
# downstream users get a working `go get` without running this Makefile.
# Contributors rerun `make dylib` after editing objc/input_events.m.

EMBEDDED_DYLIB := internal/dylib/libinput_sync.dylib
OBJC_SRC       := objc/input_events.m

ARCHES  ?= arm64 x86_64
ARCH_FLAGS := $(foreach a,$(ARCHES),-arch $(a))

CLANG_FLAGS := -dynamiclib -fobjc-arc -O2
FRAMEWORKS  := -framework Foundation \
               -framework CoreGraphics \
               -framework Carbon \
               -framework ApplicationServices \
               -framework AppKit

.PHONY: all dylib build test vet lint clean cli install-cli help

all: dylib build

$(EMBEDDED_DYLIB): $(OBJC_SRC)
	@echo "→ Building universal dylib ($(ARCHES))"
	@mkdir -p $(@D)
	clang $(CLANG_FLAGS) $(ARCH_FLAGS) $(OBJC_SRC) $(FRAMEWORKS) -o $@
	@echo "→ Verifying architectures"
	@file $@

dylib: $(EMBEDDED_DYLIB)   ## Build + commit the embedded universal dylib

build: dylib               ## Build all Go packages
	go build ./...

test: dylib                ## Run Go tests (unit only — integration needs Accessibility permission)
	go test ./...

test-integration: dylib    ## Run integration tests (requires Accessibility permission — NOT automated)
	go test -tags integration ./...

vet:                       ## go vet ./...
	go vet ./...

lint: vet                  ## vet + staticcheck + golangci-lint if available
	@command -v staticcheck >/dev/null 2>&1 && staticcheck ./... || echo "skip staticcheck"
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run || echo "skip golangci-lint"

cli: dylib                 ## Build the input CLI binary
	go build -o input ./cmd/input
	@echo "→ built ./input ($(shell du -h input | cut -f1))"

install-cli: dylib         ## Install input CLI to $$GOBIN
	go install ./cmd/input
	@echo "→ installed input to $$(go env GOBIN || echo \"$$(go env GOPATH)/bin\")"

clean:                     ## Remove build artifacts (keeps committed embedded dylib)
	rm -f libinput_sync.dylib input
	rm -rf ~/Library/Caches/input-go
	go clean ./...

clean-all: clean           ## Also delete committed embedded dylib
	rm -f $(EMBEDDED_DYLIB)

help:                      ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?##' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?##"}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'
