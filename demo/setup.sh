#!/usr/bin/env bash

set -e

# backup bao token if exists
if [ -f ~/.vault-token ]; then
  mv ~/.vault-token ~/.vault-token.bak
fi

# start bao server in backgroud and save pid
bao server -dev > /dev/null 2>&1 &
BAO_PID=$!

# write pid to file
echo $BAO_PID > ./demo/bao.pid

export BAO_ADDR=http://localhost:8200

# wait for bao server to start
sleep 3

bao secrets enable kv > /dev/null
bao kv enable-versioning kv > /dev/null
bao kv put kv/servers/vm01/os/user01 username=root password=toor > /dev/null
bao kv put kv/servers/vm01/os/user02 username=root password=toor > /dev/null
bao kv put kv/servers/vm01/os/user03 username=root password=toor > /dev/null
bao kv put kv/servers/vm01/web/user01 username=root password=toor > /dev/null
bao kv put kv/servers/vm02/os/user01 username=root password=toor > /dev/null
bao kv put kv/servers/vm03/os/user01 username=root password=toor > /dev/null
bao kv put kv/servers/vm04/os/user01 username=root password=toor > /dev/null
bao kv put kv/servers/vm05/os/user01 username=root password=toor > /dev/null
bao kv put kv/servers/vm06/os/user01 username=root password=toor > /dev/null

# disown the process
disown $BAO_PID

# build awoolt
go build .
