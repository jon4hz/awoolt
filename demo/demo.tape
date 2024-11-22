Output demo/demo.gif

Set Framerate 60
Set Height 350

Hide
Type "./demo/setup.sh"
Enter
Type "export VAULT_ADDR=http://localhost:8200"
Enter
Type "export CLICOLOR_FORCE=1"
Enter
Type "alias awoolt='./awoolt > /dev/null' && clear"
Enter
Show

Type "awoolt -e kv"
Enter
Sleep 1
# select servers
Enter
Sleep 1
# select vm01
Enter
Sleep 1
# select os
Enter
Sleep 1
# select user02
Down
Sleep 1
Enter
Sleep 0.5
Type "# Values should be printed here. No idea why the demo isn't working :("
Enter
Sleep 3

Hide
Type "./demo/teardown.sh"
Enter
