package worker

import (
	"bufio"
	"os/exec"
)

type Worker struct {
	args    []string
}

func NewWorker(args []string) *Worker {
	return &Worker{
		args: args,
	}
}

func (w *Worker) Run(output chan string) {
    cmd := exec.Command(w.args[0], w.args[1:]...)
	pipe, _ := cmd.StdoutPipe()

	cmd.Start()

	scanner := bufio.NewScanner(pipe)
	// Unneeded because scanlines is the default but just in case
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		line := scanner.Text()
		output <- line
	}
    // FIXME: cerrar canal mandando exit_code
}
