complete -c lsshell -l host -s H -d "Connect to server by name" -r
complete -c lsshell -l file -s F -d "Specify config file path" -r -a "(__fish_complete_path)"
complete -c lsshell -l generate-lssh-conf -d "Print generated lssh config from OpenSSH config" -r -a "(__fish_complete_path)"
complete -c lsshell -s R -d "Remote port forward mode" -r
complete -c lsshell -s r -d "HTTP reverse dynamic port forward mode" -r
complete -c lsshell -s m -d "NFS reverse dynamic forward mode" -r
complete -c lsshell -l term -s t -d "Run specified command in terminal"
complete -c lsshell -l list -s l -d "Print server list from config"
complete -c lsshell -l enable-control-master -d "Temporarily enable ControlMaster for this command execution"
complete -c lsshell -l disable-control-master -d "Temporarily disable ControlMaster for this command execution"
complete -c lsshell -l help -s h -d "Print help"
complete -c lsshell -l version -s v -d "Print version"
