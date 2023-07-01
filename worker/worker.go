package worker

import (
	"bufio"
	"os/exec"
)

type Worker struct {
	command string
	args    []string
}

func NewWorker(cmd string, args []string) *Worker {
	return &Worker{
		command: cmd,
		args:    args,
	}
}

func (w *Worker) Run(output chan string) {
	cmd := exec.Command(w.command, "")
	pipe, _ := cmd.StdoutPipe()

	cmd.Start()

	scanner := bufio.NewScanner(pipe)
	// Unneeded because scanlines is the default but just in case
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		line := scanner.Text()
		output <- line
	}
}
