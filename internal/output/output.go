// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package output

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/blacknon/lssh/internal/common"
	conf "github.com/blacknon/lssh/internal/config"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*m`)

type synchronizedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (w *synchronizedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.w.Write(p)
}

var terminalWriter = &synchronizedWriter{w: os.Stdout}

func TerminalWriter() io.Writer {
	return terminalWriter
}

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
		writer := TerminalWriter()
		if o.Progress != nil {
			writer = o.Progress
		}
		if (len(o.ServerList) > 1 && !o.DisableHeader) || o.EnableHeader {
			oPrompt := o.GetPrompt()
			fmt.Fprintf(writer, "%s %s\n", oPrompt, text)
		} else {
			fmt.Fprintf(writer, "%s\n", text)
		}
	}
}

// ProgressPrinter return print out progress bar
func (o *Output) ProgressPrinter(size int64, reader io.Reader, path string) {
	if o != nil && o.ProgressWG != nil {
		defer o.ProgressWG.Done()
	}

	bar := o.NewProgressBar(size, path)
	if bar == nil {
		_, _ = io.Copy(io.Discard, reader)
		return
	}

	proxy := bar.ProxyReader(reader)
	defer proxy.Close()

	_, _ = io.Copy(io.Discard, proxy)
}

func (o *Output) NewProgressBar(size int64, path string) *mpb.Bar {
	if o == nil || o.Progress == nil {
		return nil
	}

	// print header
	oPrompt := ""
	if len(o.ServerList) > 1 {
		oPrompt = o.GetPrompt()
	}

	// trim space
	path = strings.TrimSpace(path)
	barWidth, pathWidth := progressLayout(oPrompt)
	name := decor.Name(oPrompt, decor.WC{W: visibleStringWidth(oPrompt), C: decor.DindentRight})
	pathDecorator := decor.Name(trimProgressLabel(path, pathWidth), decor.WC{C: decor.DSyncSpaceR})

	return o.Progress.New(
		// size
		size,
		mpb.BarStyle().
			Lbound("[").
			Refiller("+").
			Filler("=").
			Tip(">").
			Padding("-").
			Rbound("]"),
		mpb.BarFillerClearOnComplete(),

		// prepend bar
		mpb.PrependDecorators(
			// name
			name,
			// path
			pathDecorator,
			// size
			decor.CountersKiloByte(" %.1f/%.1f", decor.WC{W: 16, C: decor.DindentRight}),
		),

		// append bar
		mpb.AppendDecorators(
			decor.OnComplete(decor.Percentage(decor.WC{W: 5, C: decor.DindentRight}), progressDoneLabel()),
		),

		mpb.BarWidth(barWidth),
	)
}

func progressDoneLabel() string {
	if isTerminal(os.Stdout) {
		return "\x1b[31mdone\x1b[0m"
	}
	return "done"
}

func progressLayout(prompt string) (barWidth, pathWidth int) {
	const (
		defaultTerminalWidth = 120
		minBarWidth          = 12
		maxBarWidth          = 28
		minPathWidth         = 18
	)

	terminalWidth := defaultTerminalWidth
	if columns := os.Getenv("COLUMNS"); columns != "" {
		if value, err := strconv.Atoi(columns); err == nil && value > 0 {
			terminalWidth = value
		}
	}

	promptWidth := visibleStringWidth(prompt)

	// Reserve enough room for counters, percentage, bar brackets, and spacing.
	remaining := terminalWidth - promptWidth - 24
	if remaining < minBarWidth+minPathWidth {
		return minBarWidth, minPathWidth
	}

	barWidth = remaining / 2
	if barWidth < minBarWidth {
		barWidth = minBarWidth
	}
	if barWidth > maxBarWidth {
		barWidth = maxBarWidth
	}

	pathWidth = remaining - barWidth
	if pathWidth < minPathWidth {
		pathWidth = minPathWidth
	}

	return barWidth, pathWidth
}

func trimProgressLabel(label string, width int) string {
	if width <= 0 {
		return ""
	}
	if visibleStringWidth(label) <= width {
		return label
	}
	if width <= 3 {
		return strings.Repeat(".", width)
	}

	suffixWidth := width - 3
	runes := []rune(label)
	if suffixWidth >= len(runes) {
		return label
	}
	return "..." + string(runes[len(runes)-suffixWidth:])
}

func visibleStringWidth(value string) int {
	clean := ansiRegexp.ReplaceAllString(value, "")
	return len([]rune(clean))
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
