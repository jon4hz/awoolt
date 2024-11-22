#!/usr/bin/env bash

set -e

# read pid from file
VAULT_PID=$(cat ./demo/vault.pid)

# restore vault token
if [ -f ~/.vault-token.bak ]; then
  mv ~/.vault-token.bak ~/.vault-token
fi

# stop vault server
kill "$VAULT_PID"

# remove files
rm ./demo/vault.pid
rm awoolt
