package psrplib

import (
	"context"
	"strings"
	"testing"
)

func TestLineBrokerNext(t *testing.T) {
	broker := newLineBroker(strings.NewReader("one\ntwo\n"))

	line, done, err := broker.Next(context.Background())
	if err != nil || done || line != "one" {
		t.Fatalf("first Next() = %q, %v, %v", line, done, err)
	}
	line, done, err = broker.Next(context.Background())
	if err != nil || done || line != "two" {
		t.Fatalf("second Next() = %q, %v, %v", line, done, err)
	}
	line, done, err = broker.Next(context.Background())
	if err != nil || !done || line != "" {
		t.Fatalf("final Next() = %q, %v, %v", line, done, err)
	}
}
