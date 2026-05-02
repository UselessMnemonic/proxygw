GO ?= go
BINDIR ?= build
GOOS ?= linux
GOARCH ?= amd64
GOFLAGS ?= -trimpath
LDFLAGS ?= -s -w

.PHONY: all proxygw proxygwctl test clean

all: plugins proxygw proxygwctl

plugins:
	$(GO) generate ./cmd/proxygw
	go mod tidy

proxygw:
	mkdir -p $(BINDIR)
	$(GO) generate ./cmd/proxygw
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BINDIR)/proxygw ./cmd/proxygw

proxygwctl:
	mkdir -p $(BINDIR)
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BINDIR)/proxygwctl ./cmd/proxygwctl

test:
	$(GO) test ./...

clean:
	go clean -testcache
	rm -f $(BINDIR)/proxygw $(BINDIR)/proxygwctl cmd/proxygw/plugin.go
