# Regenerate with: vhs docs/demo/demo.tape   (from the repo root)
Output docs/demo/demo.gif

Require go
Require vhs

Set Shell bash
Set FontSize 14
Set Width 1500
Set Height 560
Set Padding 20
Set TypingSpeed 40ms
Set Theme "Catppuccin Mocha"

Hide
Type "go build -o /tmp/wt-demo-bin/wt-cleanup . && docs/demo/setup.sh /tmp/wt-demo && export PATH=/tmp/wt-demo-bin:$PATH GIT_CONFIG_GLOBAL=/tmp/wt-demo/gitconfig && cd /tmp/wt-demo/myrepo && clear" Enter
Sleep 3s
Show

Type "wt-cleanup --no-gh" Sleep 500ms Enter
Sleep 3s

Type "wt-cleanup --no-gh --apply" Sleep 500ms Enter
Sleep 2s
# add-flag: yes
Right Sleep 600ms Enter
Sleep 1.5s
# fix-typo: no
Sleep 600ms Enter
Sleep 3s
