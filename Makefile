GO ?= go
BINDIR ?= build
GOOS ?= linux
GOARCH ?= amd64

.PHONY: all proxygw proxygwctl test clean

all: proxygw proxygwctl

proxygw:
	mkdir -p $(BINDIR)
	$(GO) generate ./cmd/proxygw
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build -o $(BINDIR)/proxygw ./cmd/proxygw

proxygwctl:
	mkdir -p $(BINDIR)
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build -o $(BINDIR)/proxygwctl ./cmd/proxygwctl

test:
	$(GO) test ./...

clean:
	rm -f $(BINDIR)/proxygw $(BINDIR)/proxygwctl cmd/proxygw/plugin.go
