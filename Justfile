default: build install-links

build: test
    CGO_ENABLED=0 go build -buildvcs=true -o xt ./cmd/xt

test:
    go test ./...

build-amd64:
    mkdir -p ./amd64
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -buildvcs=true -trimpath -ldflags="-s -w" -o ./amd64/xt ./cmd/xt
    upx -9 ./amd64/xt

BINS := "cr vm path ver vs"

install:
    mkdir -p ~/bin
    cp xt ~/bin/xt

install-links: install
    for name in {{ BINS }}; do ln -sfn xt ~/bin/"$name"; done

VM := env("VM", "vmi")

install-remote:
    ssh {{ VM }} 'mkdir -p ~/.local/bin'
    scp amd64/xt {{ VM }}:.local/bin/xt.new
    ssh {{ VM }} 'chmod +x ~/.local/bin/xt.new && mv -f ~/.local/bin/xt.new ~/.local/bin/xt'

install-remote-links: install-remote
    ssh {{ VM }} 'cd ~/.local/bin && for name in {{ BINS }}; do ln -sfn xt "$name"; done'

clean:
    -rm -f xt {{ BINS }}
    -rm -rf amd64
