package ssh

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/c-bata/go-prompt"
)

func (r *Run) shell() {
	shellConf := r.Conf.Shell //shell config

	// create new shell struct
	s := new(shell)

	// create ssh shell connects
	conns := r.createConn()
	s.CreateConn(conns)

	// connect ssh

	// set prompt templete
	s.PROMPT = shellConf.Prompt
	s.OPROMPT = shellConf.OPrompt

	// set signal
	s.Signal = make(chan os.Signal)
	signal.Notify(s.Signal, syscall.SIGTERM, syscall.SIGINT)

	// create prompt
	shellPrompt, _ := s.CreatePrompt()

	// create new go-prompt
	p := prompt.New(
		s.Executor,
		s.Completer,
		prompt.OptionPrefix(shellPrompt),
		prompt.OptionLivePrefix(s.CreatePrompt),
		prompt.OptionInputTextColor(prompt.Green),
		prompt.OptionPrefixTextColor(prompt.Blue),
	)

	// run go-prompt
	p.Run()
}
