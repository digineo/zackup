TARGET    = zackup
DEPS      = $(shell find . -type f -name '*.go')

-include $(GOPATH)/src/github.com/digineo/goldflags/goldflags.mk

.PHONY: all
all: zackup.freebsd zackup.linux

zackup.freebsd: $(DEPS)
	GOOS=freebsd GOARCH=amd64 go build -ldflags "$(GOLDFLAGS)" -o $@

zackup.linux: $(DEPS)
	GOOS=linux GOARCH=amd64 go build -ldflags "$(GOLDFLAGS)" -o $@

.PHONY: release
release:
	goreleaser --rm-dist --auto-snapshot

.PHONY: clean
clean:
	rm -rf zackup.freebsd zackup.linux dist
