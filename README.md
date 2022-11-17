# columbus-scanner

This program is used to parse the certificates from a CT log and insert into the Columbus database. 

## Build

- `go 1.19` required!

```bash
make build
```

## Install

Download and verify:
```bash 
wget -q 'https://github.com/elmasy-com/columbus-scanner/releases/latest/download/columbus-scanner' -O columbus-scanner && \
wget -q 'https://github.com/elmasy-com/columbus-scanner/releases/latest/download/columbus-scanner.sha' -O columbus-scanner.sha && \
sha512sum -c columbus-scanner.sha && rm columbus-scanner.sha && chmod +x columbus-scanner
```

0. Verify the binary:
```bash
sha512sum -c columbus-scanner.sha
```
1. Place the binary somwhere
2. Update and place the config file somewhere
3. Update and install `columbus-scanner.service` somewhere

## TODO

- Add option to modify the server