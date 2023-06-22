package main

import (
        "fmt"
        "bytes"
        "os"
        "os/exec"
)

func main() {
    args := os.Args[1:]

    cmd := exec.Command(args[0], args[1:]...)

    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
	cmd.Stderr = &stderr

    err := cmd.Run()
    if err != nil {
        fmt.Println(err)
    }

    fmt.Println("stdout:")
    fmt.Println(stdout.String())
    fmt.Println("stderr:")
    fmt.Println(stderr.String())
}
