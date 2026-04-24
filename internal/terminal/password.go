package terminal

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"strings"
)

func ReadPassword(r io.Reader) (string, bool, error) {
	file, interactive := r.(*os.File)
	var restore func()
	hidden := false
	if interactive && isTerminal(file) {
		if cleanup, ok := disableEcho(file); ok {
			restore = cleanup
			hidden = true
		}
	}
	if restore != nil {
		defer restore()
	}

	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", hidden, err
	}
	return strings.TrimSpace(line), hidden, nil
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && (info.Mode()&os.ModeCharDevice) != 0
}

func disableEcho(file *os.File) (func(), bool) {
	if err := runStty(file, "-echo"); err != nil {
		return nil, false
	}
	return func() {
		_ = runStty(file, "echo")
	}, true
}

func runStty(file *os.File, arg string) error {
	cmd := exec.Command("stty", arg)
	cmd.Stdin = file
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}
