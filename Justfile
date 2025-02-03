default: build install

build:
    go build -o cr main.go

build-amd64:
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o ./cr-amd64 .
    upx cr-amd64

install:
	cp cr ~/bin

install-vm: build-amd64
    scp cr-amd64 $(VM):~/bin/cr
