# 🐺 Awoolt
[![Go Report Card](https://goreportcard.com/badge/github.com/jon4hz/awoolt)](https://goreportcard.com/report/github.com/jon4hz/awoolt)
[![lint](https://github.com/jon4hz/awoolt/actions/workflows/lint.yml/badge.svg)](https://github.com/jon4hz/awoolt/actions/workflows/lint.yml)
[![goreleaser](https://github.com/jon4hz/awoolt/actions/workflows/release.yml/badge.svg)](https://github.com/jon4hz/awoolt/actions/workflows/release.yml)

A simple TUI for your openbao KV engines.

![demo](demo/demo.gif)

## 🚀 Installation

```bash
# using go directly
$ go install github.com/jon4hz/awoolt@latest

# from aur (btw)
$ yay -S awoolt-bin

# local pkg manager
$ export VERSION=v0.0.0 # -> replace with actual version

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
engine: my-bao-kv
```

## 🔑 Authentication
Make sure you have a valid openbao token on your system. Try `bao login`.

## ✨ Usage
```
$ awoolt --help
```

### 📥 Creating secrets
`awoolt put` creates secrets using an interactive form. Fields passed with `-m` are
also stored as custom metadata.

![put demo](demo/put.gif)
