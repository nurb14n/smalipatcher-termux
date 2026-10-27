# smalipatcher-termux: built to run ONLY on Termux (Android/aarch64).
# Delegates to Termux packages: adb (android-tools), apktool, java.
BINARY      := smalipatcher
PKG         := ./cmd/smalipatcher
PREFIX      ?= /data/data/com.termux/files/usr

.PHONY: all install test vet clean

# Default: native Android binary (runs directly in Termux, no proot).
all:
	GOOS=android GOARCH=arm64 go build -o $(BINARY) $(PKG)

# Install as the `smalipatcher` command in Termux.
install: all
	install -m755 $(BINARY) $(PREFIX)/bin/$(BINARY)

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f $(BINARY) smalipatcher-android-arm64
