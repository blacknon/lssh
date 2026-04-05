package sshlib

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync/atomic"
	"time"
)

const controlPersistEnv = "GO_SSHLIB_CONTROL_PERSIST_PAYLOAD"

type controlPersistPayload struct {
	Host                string
	Port                string
	User                string
	ControlPath         string
	ControlPersistNanos int64
	CheckKnownHosts     bool
	OverwriteKnownHosts bool
	KnownHostsFiles     []string
	Auth                []controlPersistAuthMethodDefinition
	ProxyRoute          []controlPersistProxyRoute
}

type detachedControlMaster struct {
	*controlMaster
	persist   time.Duration
	lastUsed  atomic.Int64
	closeFlag atomic.Bool
}

func init() {
	payload := os.Getenv(controlPersistEnv)
	if payload == "" {
		return
	}

	debugln("sshlib: control persist helper starting")
	err := runDetachedControlMaster(payload)
	if err != nil {
		debugln("sshlib: control persist helper failed:", err)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	debugln("sshlib: control persist helper exited cleanly")
	os.Exit(0)
}

func (c *Connect) startDetachedControlMaster(host, port, user string) error {
	debugf("sshlib: startDetachedControlMaster host=%s port=%s user=%s control_path=%s\n", host, port, user, c.ControlPath)
	if c.ControlPersistAuth == nil {
		return errors.New("sshlib: ControlPersistAuth is required when ControlPersist is enabled")
	}
	if c.ProxyDialer != nil {
		return errors.New("sshlib: detached ControlPersist does not support ProxyDialer yet")
	}
	if c.HostKeyCallback != nil && !c.CheckKnownHosts {
		return errors.New("sshlib: detached ControlPersist supports CheckKnownHosts or default insecure host key validation only")
	}

	resolvedAuths, err := c.ControlPersistAuth.resolved()
	if err != nil {
		return err
	}

	serializedProxyRoute, err := serializeControlPersistProxyRoutes(c.ProxyRoute)
	if err != nil {
		return err
	}

	payload := controlPersistPayload{
		Host:                host,
		Port:                port,
		User:                user,
		ControlPath:         c.ControlPath,
		ControlPersistNanos: int64(c.ControlPersist),
		CheckKnownHosts:     c.CheckKnownHosts,
		OverwriteKnownHosts: c.OverwriteKnownHosts,
		KnownHostsFiles:     append([]string(nil), c.KnownHostsFiles...),
		Auth:                resolvedAuths,
		ProxyRoute:          serializedProxyRoute,
	}

	encoded, err := encodeControlPersistPayload(payload)
	if err != nil {
		return err
	}
	debugf("sshlib: detached helper payload encoded bytes=%d proxy_hops=%d\n", len(encoded), len(payload.ProxyRoute))

	exe, err := os.Executable()
	if err != nil {
		return err
	}

	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Env = append(os.Environ(), controlPersistEnv+"="+encoded)

	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer devNull.Close()

	cmd.Stdin = devNull
	cmd.Stdout = devNull
	cmd.Stderr = devNull

	promptBridge, cleanupPromptIPC, err := setupControlPersistPromptIPC(cmd)
	if err != nil {
		return err
	}

	setDetachedSysProcAttr(cmd)
	if err := cmd.Start(); err != nil {
		cleanupPromptIPC()
		debugln("sshlib: failed to start detached control master:", err)
		return err
	}

	debugf("sshlib: detached control master started pid=%d\n", cmd.Process.Pid)
	startControlPersistPromptIPC(promptBridge)
	return nil
}

func encodeControlPersistPayload(payload controlPersistPayload) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

func decodeControlPersistPayload(encoded string) (controlPersistPayload, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return controlPersistPayload{}, err
	}

	var payload controlPersistPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return controlPersistPayload{}, err
	}
	return payload, nil
}

func runDetachedControlMaster(encoded string) error {
	debugln("sshlib: decoding control persist payload")
	payload, err := decodeControlPersistPayload(encoded)
	if err != nil {
		return err
	}

	debugln("sshlib: loading control persist prompt bridge")
	prompt, cleanupPrompt, err := loadControlPersistPrompt()
	if err != nil {
		return err
	}
	defer cleanupPrompt()

	debugln("sshlib: rebuilding auth methods")
	authMethods, err := createControlPersistAuthMethodsWithPrompt(payload.Auth, prompt)
	if err != nil {
		return err
	}

	con := &Connect{
		CheckKnownHosts:     payload.CheckKnownHosts,
		OverwriteKnownHosts: payload.OverwriteKnownHosts,
		KnownHostsFiles:     append([]string(nil), payload.KnownHostsFiles...),
		ControlPath:         payload.ControlPath,
		ControlPersist:      time.Duration(payload.ControlPersistNanos),
	}

	if len(payload.ProxyRoute) > 0 {
		debugf("sshlib: rebuilding proxy route hops=%d\n", len(payload.ProxyRoute))
		dialer, proxyConnects, err := buildControlPersistProxyRouteDialer(payload.ProxyRoute, prompt)
		if err != nil {
			return err
		}
		con.ProxyDialer = dialer
		con.proxyConnects = proxyConnects
	}

	debugf("sshlib: connecting target %s:%s as %s\n", payload.Host, payload.Port, payload.User)
	if err := con.createDirectClient(payload.Host, payload.Port, payload.User, authMethods); err != nil {
		_ = con.closeProxyConnects()
		return err
	}

	debugln("sshlib: starting detached control master listener")
	master, err := newDetachedControlMaster(con, payload.ControlPath, time.Duration(payload.ControlPersistNanos))
	if err != nil {
		_ = con.Client.Close()
		return err
	}
	con.controlMaster = master.controlMaster
	return master.serve(context.Background())
}

func newDetachedControlMaster(c *Connect, path string, persist time.Duration) (*detachedControlMaster, error) {
	base, err := newControlMaster(c, path)
	if err != nil {
		return nil, err
	}

	d := &detachedControlMaster{
		controlMaster: base,
		persist:       persist,
	}
	base.onActivity = d.touch
	d.touch()
	return d, nil
}

func (m *detachedControlMaster) serve(ctx context.Context) error {
	if m.persist <= 0 {
		<-ctx.Done()
		return nil
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if m.isIdleExpired() {
				m.connect.ControlPersist = 0
				return m.Close()
			}
		}
	}
}

func (m *detachedControlMaster) isIdleExpired() bool {
	m.sessionMu.Lock()
	active := m.activeSessions
	m.sessionMu.Unlock()
	if active > 0 {
		return false
	}

	last := time.Unix(0, m.lastUsed.Load())
	return time.Since(last) >= m.persist
}

func (m *detachedControlMaster) touch() {
	m.lastUsed.Store(time.Now().UnixNano())
}

func waitForControlClient(path string, timeout time.Duration) (*controlClient, error) {
	deadline := time.Now().Add(timeout)
	for {
		client, err := dialControlClient(path)
		if err == nil {
			return client, nil
		}
		if time.Now().After(deadline) {
			return nil, err
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (c *Connect) ensureControlClient() error {
	if c.controlClient == nil {
		return errors.New("sshlib: control client is not initialized")
	}

	if err := c.controlClient.Ping(); err == nil {
		return nil
	}

	if c.ControlPersist <= 0 {
		return errors.New("sshlib: control master is unavailable")
	}

	if err := c.startDetachedControlMaster(c.controlHost, c.controlPort, c.controlUser); err != nil {
		return err
	}
	c.controlSpawned = true

	client, err := waitForControlClient(c.ControlPath, 5*time.Second)
	if err != nil {
		return err
	}
	c.controlClient = client
	return nil
}

func (c *Connect) requestControl(req controlRequest) (controlResponse, error) {
	if err := c.ensureControlClient(); err != nil {
		return controlResponse{}, err
	}

	resp, err := c.controlClient.request(req)
	if err == nil {
		return resp, nil
	}

	if c.ControlPersist <= 0 {
		return controlResponse{}, err
	}

	if err := c.startDetachedControlMaster(c.controlHost, c.controlPort, c.controlUser); err != nil {
		return controlResponse{}, err
	}
	c.controlSpawned = true

	client, reacquireErr := waitForControlClient(c.ControlPath, 5*time.Second)
	if reacquireErr != nil {
		return controlResponse{}, reacquireErr
	}
	c.controlClient = client
	return c.controlClient.request(req)
}
