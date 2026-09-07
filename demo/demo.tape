Output demo/demo.gif

Set Framerate 60
Set Height 450

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
Sleep 1.5
# reveal the password
Enter
Sleep 1.5
# copy it
Type "c"
Sleep 1.5
# edit it
Type "e"
Sleep 1
Backspace 4
Type "password6767"
Sleep 1
Enter
Sleep 2
Type "y"
Sleep 2
# print and exit
Type "p"
Sleep 3
Hide
Type "./demo/teardown.sh"
Enter
# give teardown time to finish before vhs closes the shell
Sleep 2
