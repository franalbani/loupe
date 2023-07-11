package worker

import (
	"bufio"
    "io"
    "os/exec"
)

func Inhale(pipe io.ReadCloser, output chan string) {

	scanner := bufio.NewScanner(pipe)
	// Unneeded because scanlines is the default but just in case
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		line := scanner.Text()
		output <- line
	}
}

func Waiter(cmd *exec.Cmd, exit_ch chan int) {
        cmd.Wait()
        exit_ch <- cmd.ProcessState.ExitCode()
}
