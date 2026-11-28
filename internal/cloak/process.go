package cloak

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

type obscuraProcess struct {
	cmd      *exec.Cmd
	endpoint string
	token    string
}

func startObscura(ctx context.Context, executable string, env []string, stderr io.Writer) (*obscuraProcess, error) {
	directory, err := os.MkdirTemp("", "solomon-obscura-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(directory)
	ready := filepath.Join(directory, "ready.json")
	token := hex.EncodeToString(randBytes())
	cmd := exec.Command(executable, "serve", "--host", "127.0.0.1", "--port", "0", "--ready-file", ready, "--allow-file-access")
	cmd.Env = append(env, "OBSCURA_CDP_TOKEN="+token)
	cmd.Stderr = stderr
	configureCloakProcess(cmd)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	success := false
	defer func() {
		if !success {
			_ = cmd.Process.Kill()
		}
	}()
	deadline, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	poll := time.NewTicker(25 * time.Millisecond)
	defer poll.Stop()
	for {
		select {
		case <-deadline.Done():
			return nil, deadline.Err()
		case err := <-exited:
			if err != nil {
				return nil, err
			}
			return nil, errors.New("Obscura exited before it was ready")
		case <-poll.C:
			data, err := os.ReadFile(ready)
			if err != nil {
				continue
			}
			var info struct {
				Host string
				Port uint16
				Path string `json:"websocket_path"`
			}
			if err := json.Unmarshal(data, &info); err != nil || info.Host != "127.0.0.1" || info.Port == 0 || info.Path != "/devtools/browser" {
				return nil, errors.New("Obscura returned an invalid CDP endpoint")
			}
			success = true
			return &obscuraProcess{cmd: cmd, endpoint: "ws://127.0.0.1:" + strconv.Itoa(int(info.Port)) + info.Path, token: token}, nil
		}
	}
}

func randBytes() []byte {
	token := make([]byte, 32)
	rand.Read(token)
	return token
}

func (c *Client) stopObscuraLocked() {
	if c.managed != nil {
		_ = c.managed.cmd.Process.Kill()
		c.managed = nil
	}
}
