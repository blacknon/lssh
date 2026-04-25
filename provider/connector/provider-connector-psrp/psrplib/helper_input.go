package psrplib

import (
	"bufio"
	"context"
	"io"
	"strings"
)

type lineBroker struct {
	lines chan string
	err   chan error
}

func newLineBroker(stdin io.Reader) *lineBroker {
	broker := &lineBroker{
		lines: make(chan string),
		err:   make(chan error, 1),
	}

	if stdin == nil {
		close(broker.lines)
		broker.err <- io.EOF
		close(broker.err)
		return broker
	}

	go func() {
		defer close(broker.lines)
		defer close(broker.err)

		reader := bufio.NewReader(stdin)
		for {
			line, err := reader.ReadString('\n')
			if err == nil {
				broker.lines <- strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
				continue
			}

			if len(line) > 0 {
				broker.lines <- strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
			}
			broker.err <- err
			return
		}
	}()

	return broker
}

func (b *lineBroker) Next(ctx context.Context) (string, bool, error) {
	select {
	case <-ctx.Done():
		return "", false, ctx.Err()
	case line, ok := <-b.lines:
		if ok {
			return line, false, nil
		}

		err, ok := <-b.err
		if !ok {
			return "", true, io.EOF
		}
		if err == io.EOF {
			return "", true, nil
		}
		return "", false, err
	}
}
