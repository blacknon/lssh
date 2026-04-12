complete -c lspipe -l name -d "Session name" -r
complete -c lspipe -l fifo-name -d "Named pipe set name" -r
complete -c lspipe -l create-host -s H -d "Add servername when creating or replacing a session" -r
complete -c lspipe -l host -s h -d "Limit command execution to servername inside the session" -r
complete -c lspipe -l file -s F -d "Config filepath" -r -a "(__fish_complete_path)"
complete -c lspipe -l generate-lssh-conf -d "Print generated lssh config from OpenSSH config" -r -a "(__fish_complete_path)"
complete -c lspipe -l replace -d "Replace the named session if it already exists"
complete -c lspipe -l list -d "List known lspipe sessions"
complete -c lspipe -l mkfifo -d "Create a named pipe bridge for the named session"
complete -c lspipe -l list-fifos -d "List named pipe bridges"
complete -c lspipe -l rmfifo -d "Remove the named pipe bridge for the named session"
complete -c lspipe -l info -d "Show information for the named session"
complete -c lspipe -l close -d "Close the named session"
complete -c lspipe -l raw -d "Write pure stdout for exactly one resolved host"
complete -c lspipe -l enable-control-master -d "Temporarily enable ControlMaster for this command execution"
complete -c lspipe -l disable-control-master -d "Temporarily disable ControlMaster for this command execution"
complete -c lspipe -l help -d "Print help"
complete -c lspipe -l version -s v -d "Print version"
