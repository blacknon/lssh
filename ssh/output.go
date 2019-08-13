package ssh

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/blacknon/lssh/common"
	"github.com/blacknon/lssh/conf"
)

// Output struct. command execute and lssh-shell mode output data.
type Output struct {
	// Template variable value.
	//     - ${COUNT}  ... Count value(int)
	//     - ${SERVER} ... Server Name
	//     - ${ADDR}   ... Address
	//     - ${USER}   ... User Name
	//     - ${PORT}   ... Port
	//     - ${DATE}   ... Date(YYYY/mm/dd)
	//     - ${YEAR}   ... Year(YYYY)
	//     - ${MONTH}  ... Month(mm)
	//     - ${DAY}    ... Day(dd)
	//     - ${TIME}   ... Time(HH:MM:SS)
	//     - ${HOUR}   ... Hour(HH)
	//     - ${MINUTE} ... Minute(MM)
	//     - ${SECOND} ... Second(SS)
	Templete string

	// prompt is Output prompt.
	prompt string

	// target server name. ${SERVER}
	server string

	// Count value. ${COUNT}
	Count int

	//
	ServerList []string

	//
	Conf conf.ServerConfig

	//
	AutoColor bool

	//
	OutputWriter io.Writer
}

// Create template, set variable value.
func (o *Output) Create(server string) {
	// TODO(blacknon): Replaceでの処理ではなく、Text templateを作ってそちらで処理させる(置換処理だと脆弱性がありそうなので)
	o.server = server

	// get max length at server name
	length := common.GetMaxLength(o.ServerList)
	addL := length - len(server)

	// get color num
	n := common.GetOrderNumber(server, o.ServerList)
	colorServerName := outColorStrings(n, server)

	// set templete
	p := o.Templete

	// server info
	p = strings.Replace(p, "${SERVER}", fmt.Sprintf("%-*s", len(colorServerName)+addL, colorServerName), -1)
	p = strings.Replace(p, "${ADDR}", o.Conf.Addr, -1)
	p = strings.Replace(p, "${USER}", o.Conf.User, -1)
	p = strings.Replace(p, "${PORT}", o.Conf.Port, -1)

	o.prompt = p
}

// GetPrompt update variable value
func (o *Output) GetPrompt() (p string) {
	// Get time

	// replace variable value
	p = strings.Replace(o.prompt, "${COUNT}", strconv.Itoa(o.Count), -1)
	return
}

func printOutput(o *Output, output chan []byte) {
	// check o.OutputWriter.
	// default is os.Stdout.
	if o.OutputWriter == nil {
		o.OutputWriter = os.Stdout
	}

	// print output
	for data := range output {
		str := strings.TrimRight(string(data), "\n")
		for _, s := range strings.Split(str, "\n") {
			if len(o.ServerList) > 1 {
				oPrompt := o.GetPrompt()
				fmt.Fprintf(o.OutputWriter, "%s %s\n", oPrompt, s)
			} else {
				fmt.Fprintf(o.OutputWriter, "%s\n", s)
			}
		}
	}
}

func outColorStrings(num int, inStrings string) (str string) {
	// 1=Red,2=Yellow,3=Blue,4=Magenta,0=Cyan
	color := 31 + num%5

	str = fmt.Sprintf("\x1b[%dm%s\x1b[0m", color, inStrings)
	return
}

// multiPipeReadWriter is PipeReader to []io.WriteCloser.
func multiPipeReadWriter(isExit <-chan bool, output []io.WriteCloser, input io.ReadCloser) {
	rd := bufio.NewReader(input)

loop:
	for {
		buf := make([]byte, 1024)
		size, err := rd.Read(buf)

		if size > 0 {
			d := buf[:size]

			// write
			for _, w := range output {
				w.Write(d)
			}
		}

		switch err {
		case io.EOF, nil:
			continue
		case io.ErrClosedPipe:
			break loop
		}

		select {
		case <-isExit:
			break loop
		case <-time.After(10 * time.Millisecond):
			continue
		}
	}

	// close output
	for _, w := range output {
		w.Close()
	}
}

// send input to ssh Session Stdin
// TODO(blacknon): multiStdinWriterにして記述する
func pushInput(isExit <-chan bool, writer io.Writer) {
	rd := bufio.NewReader(os.Stdin)
loop:
	for {
		data, _ := rd.ReadBytes('\n')
		if len(data) > 0 {
			writer.Write(data)
		}

		select {
		case <-isExit:
			break loop
		case <-time.After(10 * time.Millisecond):
			continue
		}
	}
}
