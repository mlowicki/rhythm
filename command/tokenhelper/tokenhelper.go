package tokenhelper

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/mitchellh/go-homedir"
)

// Helper is an interface that contains basic operations that must be
// implemented by a token helper.
type Helper interface {
	Read() (string, error)
	Update(string) error
}

// Internal retrieves token from file in home directory
// and doesn't rely on executing external binary.
type Internal struct {
}

func (i *Internal) getTokenPath() (string, error) {
	homePath, err := homedir.Dir()
	if err != nil {
		return "", err
	}
	return homePath + "/.rhythm-token", nil
}

// Read retrieves token.
func (i *Internal) Read() (string, error) {
	path, err := i.getTokenPath()
	if err != nil {
		return "", err
	}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	defer f.Close()
	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, f); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

// Update either modify or set token.
func (i *Internal) Update(token string) error {
	path, err := i.getTokenPath()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	buf := bytes.NewBufferString(token)
	if _, err := io.Copy(f, buf); err != nil {
		return err
	}
	return nil
}

// External retrieves token using external binary.
type External struct {
	BinaryPath string
}

func (e *External) cmd(op string) *exec.Cmd {
	script := e.BinaryPath + " " + op
	shell := "/bin/sh"
	flag := "-c"
	if other := os.Getenv("SHELL"); other != "" {
		shell = other
	}
	cmd := exec.Command(shell, flag, script)
	return cmd
}

// Read retrieves token.
func (e *External) Read() (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := e.cmd("read")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s: %q", err, stderr.String())
	}
	return stdout.String(), nil
}

// Update either modify or set token.
func (e *External) Update(token string) error {
	buf := bytes.NewBufferString(token)
	cmd := e.cmd("update")
	cmd.Stdin = buf
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %q", err, string(output))
	}
	return nil
}
