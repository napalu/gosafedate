VERSION := $(shell git describe --tags --abbrev=0 | sed 's/^v//')
BINARY  := gosafedate

build:
	go build -trimpath -ldflags="-s -w -X github.com/napalu/gosafedate/version.Version=$(VERSION)" -o bin/$(BINARY) ./cmd/gosafedate

test:
	go test ./...

clean:
	rm -rf bin/

release: clean build
	tar czf $(BINARY)-$(VERSION).tar.gz -C bin $(BINARY)
