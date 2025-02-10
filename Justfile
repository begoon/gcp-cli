default: build install

build: test
    CGO_ENABLED=0 go build -buildvcs=true -o ./ ./...

test:
    go test ./...

build-amd64:
    mkdir -p ./amd64
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -buildvcs=true -trimpath -ldflags="-s -w" -o ./amd64 ./...
    upx -9 ./amd64/*

install:
    cp cr ~/bin
    cp vm ~/bin

VM := env("VM", "vmi")

install-cr:
    scp amd64/cr {{ VM }}:.local/bin/cr
