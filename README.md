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
wget 'https://github.com/elmasy-com/columbus-scanner/releases/latest/download/columbus-scanner' && wget 'https://github.com/elmasy-com/columbus-scanner/releases/latest/download/columbus-scanner.sha' && sha512sum -c columbus-scanner.sha 
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