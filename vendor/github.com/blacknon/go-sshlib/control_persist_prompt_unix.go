//go:build !windows

package sshlib

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

const (
	controlPersistPromptReqFDEnv  = "GO_SSHLIB_CONTROL_PERSIST_PROMPT_REQ_FD"
	controlPersistPromptRespFDEnv = "GO_SSHLIB_CONTROL_PERSIST_PROMPT_RESP_FD"
)

type controlPersistPromptBridge struct {
	reqReader  *os.File
	respWriter *os.File
	childReq   *os.File
	childResp  *os.File
}

type controlPersistPromptRequest struct {
	Prompt string `json:"prompt"`
}

type controlPersistPromptResponse struct {
	Value string `json:"value,omitempty"`
	Error string `json:"error,omitempty"`
}

type controlPersistPromptClient struct {
	reqWriter  *os.File
	respReader *os.File
	mu         sync.Mutex
}

func setupControlPersistPromptIPC(cmd *exec.Cmd) (*controlPersistPromptBridge, func(), error) {
	reqReader, reqWriter, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	respReader, respWriter, err := os.Pipe()
	if err != nil {
		_ = reqReader.Close()
		_ = reqWriter.Close()
		return nil, nil, err
	}

	cmd.ExtraFiles = append(cmd.ExtraFiles, reqWriter, respReader)
	cmd.Env = append(cmd.Env,
		controlPersistPromptReqFDEnv+"="+strconv.Itoa(3),
		controlPersistPromptRespFDEnv+"="+strconv.Itoa(4),
	)

	bridge := &controlPersistPromptBridge{
		reqReader:  reqReader,
		respWriter: respWriter,
		childReq:   reqWriter,
		childResp:  respReader,
	}

	cleanup := func() {
		_ = reqReader.Close()
		_ = reqWriter.Close()
		_ = respReader.Close()
		_ = respWriter.Close()
	}

	return bridge, cleanup, nil
}

func startControlPersistPromptIPC(bridge *controlPersistPromptBridge) {
	if bridge == nil {
		return
	}

	if bridge.childReq != nil {
		_ = bridge.childReq.Close()
		bridge.childReq = nil
	}
	if bridge.childResp != nil {
		_ = bridge.childResp.Close()
		bridge.childResp = nil
	}

	go func() {
		defer bridge.reqReader.Close()
		defer bridge.respWriter.Close()

		reader := bufio.NewReader(bridge.reqReader)
		writer := bufio.NewWriter(bridge.respWriter)
		decoder := json.NewDecoder(reader)
		encoder := json.NewEncoder(writer)

		for {
			var req controlPersistPromptRequest
			if err := decoder.Decode(&req); err != nil {
				return
			}

			value, err := getPassphrase(req.Prompt)
			resp := controlPersistPromptResponse{Value: strings.TrimRight(value, "\n")}
			if err != nil {
				resp.Error = err.Error()
				resp.Value = ""
			}

			if err := encoder.Encode(resp); err != nil {
				return
			}
			if err := writer.Flush(); err != nil {
				return
			}
		}
	}()
}

func loadControlPersistPrompt() (PromptFunc, func(), error) {
	reqFD := os.Getenv(controlPersistPromptReqFDEnv)
	respFD := os.Getenv(controlPersistPromptRespFDEnv)
	if reqFD == "" || respFD == "" {
		return nil, func() {}, nil
	}

	reqInt, err := strconv.Atoi(reqFD)
	if err != nil {
		return nil, nil, err
	}
	respInt, err := strconv.Atoi(respFD)
	if err != nil {
		return nil, nil, err
	}

	client := &controlPersistPromptClient{
		reqWriter:  os.NewFile(uintptr(reqInt), "control-persist-prompt-req"),
		respReader: os.NewFile(uintptr(respInt), "control-persist-prompt-resp"),
	}

	prompt := func(message string) (string, error) {
		client.mu.Lock()
		defer client.mu.Unlock()

		if err := json.NewEncoder(client.reqWriter).Encode(controlPersistPromptRequest{Prompt: message}); err != nil {
			return "", err
		}

		var resp controlPersistPromptResponse
		if err := json.NewDecoder(client.respReader).Decode(&resp); err != nil {
			return "", err
		}
		if resp.Error != "" {
			return "", errors.New(resp.Error)
		}
		return resp.Value, nil
	}

	cleanup := func() {
		_ = client.reqWriter.Close()
		_ = client.respReader.Close()
	}

	return prompt, cleanup, nil
}
