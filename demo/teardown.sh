#!/usr/bin/env bash

set -e

# read pid from file
BAO_PID=$(cat ./demo/bao.pid)

# restore bao token
if [ -f ~/.vault-token.bak ]; then
  mv ~/.vault-token.bak ~/.vault-token
fi

# stop bao server
kill "$BAO_PID"

# remove files
rm ./demo/bao.pid
rm awoolt
