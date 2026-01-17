package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	goruntime "runtime"
	"sync"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type PTYSession struct {
	ID      string
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	cancel  context.CancelFunc
	running bool
	mu      sync.Mutex
}

type PTYManager struct {
	ctx      context.Context
	sessions map[string]*PTYSession
	mu       sync.RWMutex
	counter  int
}

func NewPTYManager() *PTYManager {
	return &PTYManager{
		sessions: make(map[string]*PTYSession),
	}
}

func (m *PTYManager) SetContext(ctx context.Context) {
	m.ctx = ctx
}

func (m *PTYManager) getDefaultShell() string {
	if goruntime.GOOS == "windows" {
		if pwsh, err := exec.LookPath("pwsh"); err == nil {
			return pwsh
		}
		return "powershell.exe"
	}
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/sh"
}

func (m *PTYManager) Spawn(shell string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if shell == "" {
		shell = m.getDefaultShell()
	}

	m.counter++
	id := fmt.Sprintf("pty-%d", m.counter)

	ctx, cancel := context.WithCancel(m.ctx)
	cmd := exec.CommandContext(ctx, shell)
	cmd.Env = os.Environ()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return "", fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		stdin.Close()
		return "", fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	cmd.Stderr = cmd.Stdout

	session := &PTYSession{
		ID:      id,
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		cancel:  cancel,
		running: true,
	}

	if err := cmd.Start(); err != nil {
		cancel()
		stdin.Close()
		stdout.Close()
		return "", fmt.Errorf("failed to start shell: %w", err)
	}

	m.sessions[id] = session

	go m.readOutput(session)
	go m.waitForExit(session)

	return id, nil
}

func (m *PTYManager) readOutput(session *PTYSession) {
	buf := make([]byte, 4096)
	for {
		n, err := session.stdout.Read(buf)
		if n > 0 {
			data := string(buf[:n])
			wailsRuntime.EventsEmit(m.ctx, "pty:data", map[string]string{
				"id":   session.ID,
				"data": data,
			})
		}
		if err != nil {
			break
		}
	}
}

func (m *PTYManager) waitForExit(session *PTYSession) {
	_ = session.cmd.Wait()

	session.mu.Lock()
	session.running = false
	session.mu.Unlock()

	wailsRuntime.EventsEmit(m.ctx, "pty:exit", map[string]any{
		"id":       session.ID,
		"exitCode": session.cmd.ProcessState.ExitCode(),
	})
}

func (m *PTYManager) Write(id string, data string) error {
	m.mu.RLock()
	session, ok := m.sessions[id]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if !session.running {
		return fmt.Errorf("session not running: %s", id)
	}

	_, err := session.stdin.Write([]byte(data))
	return err
}

func (m *PTYManager) Kill(id string) error {
	m.mu.Lock()
	session, ok := m.sessions[id]
	if ok {
		delete(m.sessions, id)
	}
	m.mu.Unlock()

	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	session.cancel()
	session.stdin.Close()
	session.stdout.Close()

	return nil
}

func (m *PTYManager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	return ids
}

func (m *PTYManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, session := range m.sessions {
		session.cancel()
		session.stdin.Close()
		session.stdout.Close()
	}
	m.sessions = make(map[string]*PTYSession)
}
