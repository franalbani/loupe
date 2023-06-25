# loupe

Linux process monitor wrapper, with a TUI built with [BubbleTea](https://github.com/charmbracelet/bubbletea).

![Screenshot](./2023.06.25.screenshot.png)


## Usage

```console
$ go mod tidy
$ go build loupe
# ./loupe command arg0 arg1 etc
```

## Pending

### Info

* [x] `stdout` tab
* [x] `stderr` tab
* [x] `exit code`
* [ ] `stdin` redirection from text input
* [ ] syscalls and signals tab (`strace`)
* [ ] opened files (`lsof -p $PID`)
* [ ] opened connections (`netstat`)
* [ ] CPU and MEM stats
* [ ] *please suggest more!*

### visual

* [x] AltScreen
* [x] `stdin` textinput
* [x] scrollable fixed-height viewports

