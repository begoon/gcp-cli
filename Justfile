default: build install

build:
    CGO_ENABLED=0 go build -buildvcs=true -trimpath -o cr main.go

build-amd64:
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -buildvcs=true -trimpath -ldflags="-s -w" -o ./cr-amd64 .
    upx --best cr-amd64

install:
	cp cr ~/bin

install-vm: build-amd64
    scp cr-amd64 ${VM}:.local/bin/cr
