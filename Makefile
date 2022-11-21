LDFLAGS = -s
LDFLAGS += -w
LDFLAGS += -X 'main.Version=$(shell git describe --tags --abbrev=0)'
LDFLAGS += -X 'main.Commit=$(shell git rev-list -1 HEAD)'

build:
	go build -o columbus-scanner -ldflags="$(LDFLAGS)" .

build-dev:
	go build -o columbus-scanner-dev --race .

release: build
	sha512sum columbus-scanner | gpg --clearsign -u daniel@elmasy.com > columbus-scanner.sha