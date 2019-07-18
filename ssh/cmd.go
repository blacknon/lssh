package ssh

func (r *Run) cmd() {
	// make channel
	finished := make(chan bool)

	// print header
	r.printSelectServer()
	r.printRunCommand()
	if len(r.ServerList) == 1 {
		r.printProxy(r.ServerList[0])
	}

	// for run loop
	for _, server := range r.ServerList {

	}
}
