# 🐺 Awoolt
[![Go Report Card](https://goreportcard.com/badge/github.com/jon4hz/awoolt)](https://goreportcard.com/report/github.com/jon4hz/awoolt)
[![lint](https://github.com/jon4hz/awoolt/actions/workflows/lint.yml/badge.svg)](https://github.com/jon4hz/awoolt/actions/workflows/lint.yml)
[![goreleaser](https://github.com/jon4hz/awoolt/actions/workflows/release.yml/badge.svg)](https://github.com/jon4hz/awoolt/actions/workflows/release.yml)

Interactively browse vault/openbao in the terminal.

## 🚀 Installation

```bash
# using go directly
$ go install github.com/jon4hz/awoolt@latest

# from aur (btw)
$ yay -S awoolt-bin

# local pkg manager
$ export VERSION=v0.1.0

## debian / ubuntu
$ dpkg -i awoolt-$VERSION-linux-amd64.deb

## rhel / sles
$ rpm -i awoolt-$VERSION-linux-amd64.rpm

## alpine
$ apk add --allow-untrusted awoolt-$VERSION-linux-amd64.apk
```
All releases can be found [here](https://github.com/jon4hz/awoolt/releases)

## 📝 Config

`awoolt` searches for a config file in the following locations:
1. `./awoolt.yml`
2. `~/.config/awoolt/awoolt.yml`
3. `/etc/awoolt/awoolt.yml`

### 🥁 Example
```yaml
# ~/.config/awoolt/awoolt.yml
---
engine: my-vault-kv
```

## 🔑 Authentication
Make sure you have a valid vault token on your system. Try `vault login`.

## ✨ Usage
```
$ awoolt --help
interactively browse vault/openbao in the terminal.

Usage:
  awoolt [flags]

Flags:
  -e, --engine string   secret engine to use
  -h, --help            help for awoolt
  -p, --path string     secret path
```
