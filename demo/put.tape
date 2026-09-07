Output demo/put.gif

Set Framerate 60
Set Height 500

Hide
Type "./demo/setup.sh"
Enter
Type "export BAO_ADDR=http://localhost:8200"
Enter
Type "export CLICOLOR_FORCE=1"
Enter
Type "alias awoolt='./awoolt' && clear"
Enter
Show

Type "awoolt put -e kv servers/vm01/os/user04 -f username,password -m username"
Enter
Sleep 1.5
# username
Type "root"
Sleep 0.5
Enter
Sleep 0.5
# password
Type "super6secret7"
Sleep 0.5
Enter
Sleep 1.5
# custom metadata: username is preselected via -m
Enter
Sleep 1.5
# confirm write
Enter
Sleep 2

# the new secret shows up with its metadata
Type "awoolt -e kv -p servers/vm01/os"
Enter
Sleep 1.5
Down 3
Sleep 3
Ctrl+C
Sleep 1

Hide
Type "./demo/teardown.sh"
Enter
# give teardown time to finish before vhs closes the shell
Sleep 2
