complete -c lssh -l host -s H -d "Connect to server by name" -r
complete -c lssh -l file -s F -d "Specify config file path" -r -a "(__fish_complete_path)"
complete -c lssh -l generate-lssh-conf -d "Print generated lssh config from OpenSSH config" -r -a "(__fish_complete_path)"
complete -c lssh -s L -d "Local port forward mode" -r
complete -c lssh -s R -d "Remote port forward mode" -r
complete -c lssh -s D -d "Dynamic port forward mode" -r
complete -c lssh -s d -d "HTTP dynamic port forward mode" -r
complete -c lssh -s r -d "HTTP reverse dynamic port forward mode" -r
complete -c lssh -s M -d "NFS dynamic forward mode" -r
complete -c lssh -s m -d "NFS reverse dynamic forward mode" -r
complete -c lssh -s S -d "SMB dynamic forward mode" -r
complete -c lssh -s s -d "SMB reverse dynamic forward mode" -r
complete -c lssh -l tunnel -d "Enable tunnel device" -r
complete -c lssh -s w -d "Display server header in command execution mode"
complete -c lssh -s W -d "Do not display server header in command execution mode"
complete -c lssh -l not-execute -s N -d "Do not execute remote command or shell"
complete -c lssh -l X11 -s X -d "Enable X11 forwarding"
complete -c lssh -s Y -d "Enable trusted X11 forwarding"
complete -c lssh -l term -s t -d "Run specified command in terminal"
complete -c lssh -l parallel -s p -d "Run command in parallel mode"
complete -c lssh -s P -d "Run shell or command in mux UI"
complete -c lssh -l hold -d "Keep command panes after remote command exits"
complete -c lssh -l allow-layout-change -d "Allow opening new pages or panes even in command mode"
complete -c lssh -l localrc -d "Use local bashrc shell"
complete -c lssh -l not-localrc -d "Do not use local bashrc shell"
complete -c lssh -l list -s l -d "Print server list from config"
complete -c lssh -l enable-control-master -d "Temporarily enable ControlMaster for this command execution"
complete -c lssh -l disable-control-master -d "Temporarily disable ControlMaster for this command execution"
complete -c lssh -l help -s h -d "Print help"
complete -c lssh -l version -s v -d "Print version"
complete -c lssh -s f -d "Run in background after forwarding or connection"
