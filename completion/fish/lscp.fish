complete -c lscp -l host -s H -d "Connect to server by name" -r
complete -c lscp -l list -s l -d "Print server list from config"
complete -c lscp -l file -s F -d "Specify config file path" -r -a "(__fish_complete_path)"
complete -c lscp -l generate-lssh-conf -d "Print generated lssh config from OpenSSH config" -r -a "(__fish_complete_path)"
complete -c lscp -l parallel -s P -d "Parallel file copy count per host" -r
complete -c lscp -l permission -s p -d "Copy file permission"
complete -c lscp -l dry-run -d "Show copy actions without modifying files"
complete -c lscp -l enable-control-master -d "Temporarily enable ControlMaster for this command execution"
complete -c lscp -l disable-control-master -d "Temporarily disable ControlMaster for this command execution"
complete -c lscp -l help -s h -d "Print help"
complete -c lscp -l version -s v -d "Print version"
complete -c lscp -f -a "local: remote:"
