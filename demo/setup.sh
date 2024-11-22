#!/usr/bin/env bash

set -e

# backup vault token if exists
if [ -f ~/.vault-token ]; then
  mv ~/.vault-token ~/.vault-token.bak
fi

# start vault server in backgroud and save pid
vault server -dev > /dev/null 2>&1 &
VAULT_PID=$!

# write pid to file
echo $VAULT_PID > ./demo/vault.pid

export VAULT_ADDR=http://localhost:8200

# wait for vault server to start
sleep 3

vault secrets enable kv > /dev/null
vault kv enable-versioning kv > /dev/null
vault kv put kv/servers/vm01/os/user01 username=root password=toor > /dev/null
vault kv put kv/servers/vm01/os/user02 username=root password=toor > /dev/null
vault kv put kv/servers/vm01/os/user03 username=root password=toor > /dev/null
vault kv put kv/servers/vm01/web/user01 username=root password=toor > /dev/null
vault kv put kv/servers/vm02/os/user01 username=root password=toor > /dev/null
vault kv put kv/servers/vm03/os/user01 username=root password=toor > /dev/null
vault kv put kv/servers/vm04/os/user01 username=root password=toor > /dev/null
vault kv put kv/servers/vm05/os/user01 username=root password=toor > /dev/null
vault kv put kv/servers/vm06/os/user01 username=root password=toor > /dev/null

# disown the process
disown $VAULT_PID

# build awoolt
go build .
