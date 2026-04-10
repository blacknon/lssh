complete -c lsmux -l host -s H -d "Connect to server by name" -r
complete -c lsmux -l file -s F -d "Specify config file path" -r -a "(__fish_complete_path)"
complete -c lsmux -l generate-lssh-conf -d "Print generated lssh config from OpenSSH config" -r -a "(__fish_complete_path)"
complete -c lsmux -s R -d "Remote port forward mode" -r
complete -c lsmux -s r -d "HTTP reverse dynamic port forward mode" -r
complete -c lsmux -s m -d "NFS reverse dynamic forward mode" -r
complete -c lsmux -l hold -d "Keep command panes after remote command exits"
complete -c lsmux -l allow-layout-change -d "Allow opening new pages or panes even in command mode"
complete -c lsmux -l list -s l -d "Print server list from config"
complete -c lsmux -l help -s h -d "Print help"
complete -c lsmux -l version -s v -d "Print version"
