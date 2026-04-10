complete -c lsmon -l host -s H -d "Connect to server by name" -r
complete -c lsmon -l file -s F -d "Specify config file path" -r -a "(__fish_complete_path)"
complete -c lsmon -l generate-lssh-conf -d "Print generated lssh config from OpenSSH config" -r -a "(__fish_complete_path)"
complete -c lsmon -l logfile -s L -d "Set log file path" -r -a "(__fish_complete_path)"
complete -c lsmon -l share-connect -s s -d "Reuse the monitor SSH connection for terminals"
complete -c lsmon -l list -s l -d "Print server list from config"
complete -c lsmon -l debug -d "Enable pprof on localhost:6060"
complete -c lsmon -l help -s h -d "Print help"
complete -c lsmon -l version -s v -d "Print version"
