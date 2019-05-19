package ssh

import (
	"os"
	"os/signal"
	"syscall"
)

func (r *Run) shell() {

	shellConf := r.Conf.Shell //shell config

	// create new shell struct
	s := new(shell)

	// create ssh connects
	// s.CreateConn() // 一時的にコメントアウト

	// set prompt templete
	s.PROMPT = shellConf.Prompt
	s.OPROMPT = shellConf.OPrompt

	// set signal
	s.Signal = make(chan os.Signal)
	signal.Notify(s.Signal, syscall.SIGTERM, syscall.SIGINT)

	// create prompt
	// shellPrompt, _ := s.CreatePrompt() // 一時的にコメントアウト

	// create new go-prompt
	// p := prompt.New(
	// 	s.Executor,
	// 	s.Completer,
	// 	prompt.OptionPrefix(shellPrompt),
	// 	prompt.OptionLivePrefix(s.CreatePrompt),
	// 	prompt.OptionInputTextColor(prompt.Green),
	// 	prompt.OptionPrefixTextColor(prompt.Blue),
	// )

	// run go-prompt
	// p.Run()
}
