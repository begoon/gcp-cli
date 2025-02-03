default: build install

build:
    go build -o cr main.go

build-amd64:
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o ./cr .
    upx cr

install:
	cp cr ~/bin
