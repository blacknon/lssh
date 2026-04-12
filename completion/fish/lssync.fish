complete -c lssync -l host -s H -d "Connect to server by name" -r
complete -c lssync -l list -s l -d "Print server list from config"
complete -c lssync -l file -s F -d "Specify config file path" -r -a "(__fish_complete_path)"
complete -c lssync -l generate-lssh-conf -d "Print generated lssh config from OpenSSH config" -r -a "(__fish_complete_path)"
complete -c lssync -l daemon -s D -d "Run as a daemon and repeat sync at each interval"
complete -c lssync -l daemon-interval -d "Set daemon sync interval" -r
complete -c lssync -l bidirectional -s B -d "Sync both sides"
complete -c lssync -l parallel -s P -d "Parallel file sync count per host" -r
complete -c lssync -l permission -s p -d "Copy file permission"
complete -c lssync -l dry-run -d "Show sync actions without modifying files"
complete -c lssync -l delete -d "Delete destination entries missing in source"
complete -c lssync -l enable-control-master -d "Temporarily enable ControlMaster for this command execution"
complete -c lssync -l disable-control-master -d "Temporarily disable ControlMaster for this command execution"
complete -c lssync -l help -s h -d "Print help"
complete -c lssync -l version -s v -d "Print version"
complete -c lssync -f -a "local: remote:"
