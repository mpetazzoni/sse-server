build:
	go build
macos:
	GOOS=darwin GOARCH=arm64 go build
linux:
	GOOS=linux GOARCH=amd64 go build
