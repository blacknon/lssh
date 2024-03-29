// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package output

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/blacknon/lssh/common"
	"github.com/blacknon/lssh/conf"
	"github.com/vbauerster/mpb"
	"github.com/vbauerster/mpb/decor"
)

// Output struct. command execute and lssh-shell mode output data.
type Output struct {
	// Template variable value (in unimplemented).
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
	Prompt string

	// target server name. ${SERVER}
	Server string

	// Count value. ${COUNT}
	Count int

	// Selected Server list
	ServerList []string

	// ServerConfig
	Conf conf.ServerConfig

	// Progress bar
	Progress   *mpb.Progress
	ProgressWG *sync.WaitGroup

	// Enable/Disable print header
	EnableHeader  bool
	DisableHeader bool

	// Auto Colorize flag
	// TODO(blacknon): colormodeに応じて、パイプ経由だった場合は色分けしないなどの対応ができるように条件分岐する(v0.6.2)
	AutoColor bool
}

// Create template, set variable value.
func (o *Output) Create(server string) {
	o.Server = server

	// get max length at server name
	length := common.GetMaxLength(o.ServerList)
	addL := length - len(server)

	// get color num
	n := common.GetOrderNumber(server, o.ServerList)
	colorServerName := OutColorStrings(n, server)

	// set templete
	p := o.Templete

	// server info
	p = strings.Replace(p, "${SERVER}", fmt.Sprintf("%-*s", len(colorServerName)+addL, colorServerName), -1)
	p = strings.Replace(p, "${ADDR}", o.Conf.Addr, -1)
	p = strings.Replace(p, "${USER}", o.Conf.User, -1)
	p = strings.Replace(p, "${PORT}", o.Conf.Port, -1)

	o.Prompt = p
}

// GetPrompt update variable value
func (o *Output) GetPrompt() (p string) {
	// Get time

	// replace variable value
	p = strings.Replace(o.Prompt, "${COUNT}", strconv.Itoa(o.Count), -1)
	return
}

// NewWriter return io.WriteCloser at Output printer.
func (o *Output) NewWriter() (writer *io.PipeWriter) {
	// create io.PipeReader, io.PipeWriter
	r, w := io.Pipe()

	// run output.Printer()
	go o.Printer(r)

	// return writer
	return w
}

// Printer output stdout from reader.
func (o *Output) Printer(reader io.ReadCloser) {
	sc := bufio.NewScanner(reader)
loop:
	for {
		for sc.Scan() {
			text := sc.Text()
			if (len(o.ServerList) > 1 && !o.DisableHeader) || o.EnableHeader {
				oPrompt := o.GetPrompt()
				fmt.Printf("%s %s\n", oPrompt, text)
			} else {
				fmt.Printf("%s\n", text)
			}
		}

		if sc.Err() == io.ErrClosedPipe {
			break loop
		}

		select {
		case <-time.After(50 * time.Millisecond):
			continue
		}
	}
}

// ProgressPrinter return print out progress bar
func (o *Output) ProgressPrinter(size int64, reader io.Reader, path string) {
	// print header
	oPrompt := ""
	if len(o.ServerList) > 1 {
		oPrompt = o.GetPrompt()
	}
	name := decor.Name(oPrompt)

	// trim space
	path = strings.TrimSpace(path)

	// set progress
	bar := o.Progress.AddBar(
		// size
		size,

		// bar clear at complete
		mpb.BarClearOnComplete(),

		// prepend bar
		mpb.PrependDecorators(
			// name
			name,
			// path and complete message
			decor.OnComplete(decor.Name(path), fmt.Sprintf("%s done!", path)),
			// size
			decor.OnComplete(decor.CountersKiloByte(" %.1f/%.1f", decor.WC{W: 5}), ""),
		),

		// append bar
		mpb.AppendDecorators(
			decor.OnComplete(decor.Percentage(decor.WC{W: 5}), ""),
		),

		// bar style
		mpb.BarStyle("[=>-]<+"),
	)

	// set start, and max time
	start := time.Now()
	max := 10 * time.Millisecond

	var sum int

	// print out progress
	defer o.ProgressWG.Done()
	for {
		// sleep
		time.Sleep(time.Duration(rand.Intn(10)+1) * max / 10)

		// read byte (1mb)
		b := make([]byte, 1048576)
		s, err := reader.Read(b)

		sum += s

		// add size
		bar.IncrBy(s, time.Since(start))

		// check exit
		if err == io.EOF {
			bar.SetTotal(size, true)
			break
		}
	}

	return
}

// OutColorStrings return color code
func OutColorStrings(num int, inStrings string) (str string) {
	// 1=Red,2=Yellow,3=Blue,4=Magenta,0=Cyan
	color := 31 + num%5

	str = fmt.Sprintf("\x1b[%dm%s\x1b[0m", color, inStrings)
	return
}

// PushInput is Reader([io.PipeReader, os.Stdin]) to []io.WriteCloser.
func PushInput(isExit <-chan bool, output []io.WriteCloser, input io.Reader) {
	rd := bufio.NewReader(input)

loop:
	for {
		buf := make([]byte, 1024)
		size, err := rd.Read(buf)

		select {
		case <-isExit:
			break loop
		case <-time.After(10 * time.Millisecond):
			if size > 0 {
				d := buf[:size]

				// write
				for _, w := range output {
					w.Write(d)
				}
			}

			if input != os.Stdin {
				switch err {
				case io.ErrClosedPipe, io.EOF:
					break loop
				}
			}

		}
	}

	// close output
	for _, w := range output {
		w.Close()
	}
}
