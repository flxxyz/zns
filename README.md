# Simple DoH server in Go

## QuickStart

[![Deploy on Railway](https://railway.com/button.svg)](https://railway.app/template/gUgRJT?referralCode=rLqwJq)

## Self-Hosts

```bash
git clone --depth=0 github.com/flxxyz/doh/cmd/doh && cd doh
go build ./cmd/doh
./doh -free -tls-hosts doh.example.org -root ./web
```
