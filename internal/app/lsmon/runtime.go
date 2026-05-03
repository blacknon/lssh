//go:build !windows

package lsmon

import (
	"log"
	"net/http"
	"os"
)

func setupLogOutput(path string) (*os.File, error) {
	logfile, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	log.SetOutput(logfile)
	return logfile, nil
}

func startDebugServer(enabled bool) {
	if !enabled {
		return
	}
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
}
