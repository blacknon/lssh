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

	"github.com/blacknon/lssh/internal/common"
	conf "github.com/blacknon/lssh/internal/config"
	"github.com/vbauerster/mpb"
	"github.com/vbauerster/mpb/decor"
)

type syncedPipeWriter struct {
	writer *io.PipeWriter
	done   <-chan struct{}
}

func (w *syncedPipeWriter) Write(p []byte) (n int, err error) {
	return w.writer.Write(p)
}

func (w *syncedPipeWriter) Close() error {
	err := w.writer.Close()
	<-w.done
	return err
}

func (w *syncedPipeWriter) CloseWithError(err error) error {
	closeErr := w.writer.CloseWithError(err)
	<-w.done
	return closeErr
}

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
	colorServerName := server
	if o.AutoColor && isTerminal(os.Stdout) {
		colorServerName = OutColorStrings(n, server)
	}

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

// NewWriter returns a writer that blocks on Close until the printer drains.
func (o *Output) NewWriter() *syncedPipeWriter {
	// create io.PipeReader, io.PipeWriter
	r, w := io.Pipe()
	done := make(chan struct{})

	// run output.Printer()
	go func() {
		o.Printer(r)
		close(done)
	}()

	// return writer
	return &syncedPipeWriter{writer: w, done: done}
}

// Printer output stdout from reader.
func (o *Output) Printer(reader io.ReadCloser) {
	sc := bufio.NewScanner(reader)
	for sc.Scan() {
		text := sc.Text()
		if (len(o.ServerList) > 1 && !o.DisableHeader) || o.EnableHeader {
			oPrompt := o.GetPrompt()
			fmt.Printf("%s %s\n", oPrompt, text)
		} else {
			fmt.Printf("%s\n", text)
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
		if err != nil {
			if err == io.EOF {
				bar.SetTotal(size, true)
			} else {
				bar.SetTotal(int64(sum), true)
			}
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

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}

	return (info.Mode() & os.ModeCharDevice) != 0
}

// PushInput is Reader([io.PipeReader, os.Stdin]) to []io.WriteCloser.
func PushInput(isExit <-chan bool, output []io.WriteCloser, input io.Reader) {
	type readResult struct {
		buf  []byte
		size int
		err  error
	}

	results := make(chan readResult)
	go func() {
		rd := bufio.NewReader(input)
		for {
			buf := make([]byte, 1024)
			size, err := rd.Read(buf)
			results <- readResult{buf: buf, size: size, err: err}
			if err != nil {
				return
			}
		}
	}()

loop:
	for {
		select {
		case <-isExit:
			break loop
		case result := <-results:
			if result.size > 0 {
				d := result.buf[:result.size]
				for _, w := range output {
					_, _ = w.Write(d)
				}
			}

			if input != os.Stdin {
				switch result.err {
				case io.ErrClosedPipe, io.EOF:
					break loop
				}
			}
		}
	}

	// close output
	for _, w := range output {
		_ = w.Close()
	}
}
