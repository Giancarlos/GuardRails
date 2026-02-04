.PHONY: build build-all clean test release snapshot

# Default build for current platform
build:
	go build -o gur .

# Build for all platforms (without goreleaser)
build-all:
	GOOS=darwin GOARCH=amd64 go build -o dist/gur-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build -o dist/gur-darwin-arm64 .
	GOOS=linux GOARCH=amd64 go build -o dist/gur-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build -o dist/gur-linux-arm64 .
	GOOS=windows GOARCH=amd64 go build -o dist/gur-windows-amd64.exe .

# Clean build artifacts
clean:
	rm -rf dist/
	rm -f gur

# Run tests
test:
	go test ./...

# Local snapshot build with goreleaser (no publish)
snapshot:
	goreleaser build --snapshot --clean

# Full local release with goreleaser (no publish)
release:
	goreleaser release --snapshot --clean --skip=publish
