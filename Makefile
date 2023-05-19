LDFLAGS = -s
LDFLAGS += -w
LDFLAGS += -X 'main.Version=$(shell git describe --tags --abbrev=0)'
LDFLAGS += -X 'main.Commit=$(shell git rev-list -1 HEAD)'
LDFLAGS += -extldflags "-static"'

clean:
	@if [ -e "./columbus-scanner" ];     then rm -rf "./columbus-scanner"    ; fi
	@if [ -e "./columbus-scanner-dev" ]; then rm -rf "./columbus-scanner-dev"; fi
	@if [ -e "./columbus-scanner.sha" ]; then rm -rf "./columbus-scanner.sha"; fi


build: clean
	CGO_ENABLED=0 go build -o columbus-scanner -tags netgo -ldflags="$(LDFLAGS)" .

build-dev: clean
	go build -o columbus-scanner --race .

release: build
	sha512sum columbus-scanner | gpg --clearsign -u daniel@elmasy.com > columbus-scanner.sha