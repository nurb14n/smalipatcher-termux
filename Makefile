BINARY      := smalipatcher
PKG         := ./cmd/smalipatcher
ANDROID_OUT := $(BINARY)-android-arm64

.PHONY: all linux android test vet clean

all: linux

linux:
	go build -o $(BINARY) $(PKG)

# Native Termux binary (Android/bionic, runs outside proot).
android:
	GOOS=android GOARCH=arm64 go build -o $(ANDROID_OUT) $(PKG)

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f $(BINARY) $(ANDROID_OUT)
