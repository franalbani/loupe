package main

import (
        "fmt"
        "os"
        "os/exec"
        "strings"
        "bufio"
        "path/filepath"
        "golang.org/x/sys/unix"
        "regexp"
        tea "github.com/charmbracelet/bubbletea"
        lg "github.com/charmbracelet/lipgloss"
        "github.com/charmbracelet/bubbles/viewport"
        "github.com/charmbracelet/bubbles/textinput"
        "github.com/charmbracelet/log"
)

// this model will be used for BubbleTea state
type model struct {
    cmd *exec.Cmd
    stdout_scanner, stderr_scanner, strace_scanner *bufio.Scanner
    strace_fifo_path string
    stdout_lines, stderr_lines, strace_lines, opened_files, connect_lines []string
    selected_tab uint
    exit_code int
    ready bool
    vp viewport.Model
    stdin_ti textinput.Model
}

type StdOutMsg string
type StdErrMsg string
type StraceMsg string
type StraceScannerMsg *bufio.Scanner
type BastaMsg string
type ExitMsg int

var OpenatRegExp = regexp.MustCompile(`openat\((.*?), "(.*?)", (.*?)\).*`)
var ConnectRegExp = regexp.MustCompile(`connect\(\d+, {.*?sin_addr=inet_addr\("([^"]+)"\)}, \d+\).*`)

func inhale(pipeScanner *bufio.Scanner, wrapper func(string) tea.Msg, name string) tea.Cmd {
    return func() tea.Msg {
        if pipeScanner.Scan() {
            return wrapper(pipeScanner.Text())
        }
        if err := pipeScanner.Err(); err != nil {
            log.Info(name, "err", err)
            return BastaMsg("Error")
        } else {
            // log.Info(name + " EOF")
            return BastaMsg("EOF")
        }
    }
}

// This is needed to avoid getting EOF
func openStraceFifo(path string) tea.Cmd {
    return func() tea.Msg {
        strace_pipe, _ := os.Open(path)
	    return StraceScannerMsg(bufio.NewScanner(strace_pipe))
    }
}

// This command launchs the process,
// waits for it and then emits a message with its exit code
func launchAndWait(cmd *exec.Cmd) tea.Cmd {
    cmd.Start()
    return func() tea.Msg {
        cmd.Wait()
        return ExitMsg(cmd.ProcessState.ExitCode())
    }
}

// this method is required by BubbleTea
func (m model) Init() tea.Cmd {
    return tea.Batch(
            inhale(m.stdout_scanner, func(x string) tea.Msg {return StdOutMsg(x)}, "stdout"),
            inhale(m.stderr_scanner, func(x string) tea.Msg {return StdErrMsg(x)}, "stderr"),
            openStraceFifo(m.strace_fifo_path),
            launchAndWait(m.cmd))
}

// this method is required by BubbleTea
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var tiCmd, vp_cmd, seba_cmd tea.Cmd

    switch msg := msg.(type) {

    case tea.KeyMsg:
        switch msg.String() {
            case "tab":
                m.selected_tab = (m.selected_tab + 1) % 5
            case "shift+tab":
                m.selected_tab = (m.selected_tab + 4) % 5
            case "ctrl+c":
                return m, tea.Quit
        }

    case tea.WindowSizeMsg:
        if !m.ready {
            m.vp = viewport.New(1, 1)
            m.ready = true
        }
        m.vp.Width = msg.Width
        m.vp.Height = msg.Height - 9 // FIXME --> softcode the 9

        // FIXME: force a minimum Width for borders
        m.vp.Style = content_style

    // TODO: check if its really reading all output
    case StdOutMsg:
        m.stdout_lines = append(m.stdout_lines, string(msg))
        seba_cmd = inhale(m.stdout_scanner, func(x string) tea.Msg {return StdOutMsg(x)}, "stdout")
    case StdErrMsg:
        m.stderr_lines = append(m.stderr_lines, string(msg))
        seba_cmd = inhale(m.stderr_scanner, func(x string) tea.Msg {return StdErrMsg(x)}, "stderr")
    case StraceMsg:
        m.strace_lines = append(m.strace_lines, string(msg))
        seba_cmd = inhale(m.strace_scanner, func(x string) tea.Msg {return StraceMsg(x)}, "strace")
        openat_match := OpenatRegExp.FindStringSubmatch(string(msg))
        if len(openat_match) > 0 {
            m.opened_files = append(m.opened_files, openat_match[2])
        }
        connect_match := ConnectRegExp.FindStringSubmatch(string(msg))
        if len(connect_match) > 0 {
            m.connect_lines = append(m.connect_lines, connect_match[1])
        }
    case StraceScannerMsg:
        m.strace_scanner = msg
        seba_cmd = inhale(m.strace_scanner, func(x string) tea.Msg {return StraceMsg(x)}, "strace")
    case ExitMsg:
        m.exit_code = int(msg)
    }

    // FIXME: improve this!
    content := ""
    switch m.selected_tab {
    case 0:
        content = strings.Join(m.stdout_lines, "\n")
    case 1:
        content = strings.Join(m.stderr_lines, "\n")
    case 2:
        content = strings.Join(m.strace_lines, "\n")
    case 3:
        content = strings.Join(m.opened_files, "\n")
    case 4:
        content = strings.Join(m.connect_lines, "\n")
    default:
        content = "soon"
    }
    m.vp.SetContent(content)

    m.vp, vp_cmd = m.vp.Update(msg)
    m.stdin_ti, tiCmd = m.stdin_ti.Update(msg)

    return m, tea.Batch(tiCmd, vp_cmd, seba_cmd)
}

var tab_styles = map[bool]lg.Style{
    false: lg.NewStyle().
               BorderStyle(lg.RoundedBorder()).
               BorderForeground(lg.Color("63")).
               Foreground(lg.Color("5")),
    true: lg.NewStyle().
             BorderStyle(lg.RoundedBorder()).
             BorderForeground(lg.Color("63")).
             Foreground(lg.Color("86")),
}

func tab_header(selected_tab uint) string {
    return lg.JoinHorizontal(lg.Top,
                             tab_styles[selected_tab == 0].Render("stdout"),
                             tab_styles[selected_tab == 1].Render("stderr"),
                             tab_styles[selected_tab == 2].Render("strace"),
                             tab_styles[selected_tab == 3].Render("files"),
                             tab_styles[selected_tab == 4].Render("connect"),
                             )
}

var content_style = lg.NewStyle().
                       BorderStyle(lg.RoundedBorder()).
                       BorderForeground(lg.Color("63"))

var help_footer = lg.NewStyle().
                     Foreground(lg.Color("#5C5C5C")).
                     SetString("TAB for switching, CTRL + C to quit.")

// this method is required by BubbleTea
func (m model) View() string {

    proc_state := "Running..."
    if m.cmd.ProcessState != nil {
        ec_color := "3"
        if m.exit_code != 0 {
            ec_color = "9"
        }
        ec_style := lg.NewStyle().Foreground(lg.Color(ec_color))
        proc_state = "Exit code: " + ec_style.Render(fmt.Sprintf("%d", m.exit_code))
    }
    s := lg.JoinVertical(lg.Left,
            tab_styles[false].Render(strings.Join(m.cmd.Args[3:], " ")),
            proc_state,
            tab_header(m.selected_tab),
            m.vp.View(),
            m.stdin_ti.View(),
            help_footer.Render())
    return s
}

func main() {
    log.Info("main")
    tempdir, _ := os.MkdirTemp("", "loupe_")
    defer os.RemoveAll(tempdir)

    strace_fifo_path := filepath.Join(tempdir, "strace")
    unix.Mkfifo(strace_fifo_path, 0666)
    log.Info("created strace_fifo", "at", strace_fifo_path)

    _args := []string{"strace", "--output", strace_fifo_path}

    args := os.Args[1:]
    _args = append(_args, args...)

    cmd := exec.Command(_args[0], _args[1:]...)
    stdout_pipe, _ := cmd.StdoutPipe()
    stderr_pipe, _ := cmd.StderrPipe()
    stdout_scanner := bufio.NewScanner(stdout_pipe)
	stderr_scanner := bufio.NewScanner(stderr_pipe)

    ti := textinput.New()
    ti.Placeholder = "stdin"
    ti.Prompt = "$ "

    initial_model := model{
        cmd: cmd,
        stdout_scanner: stdout_scanner,
        stderr_scanner: stderr_scanner,
        strace_fifo_path: strace_fifo_path,
        selected_tab: 0,
        stdin_ti: ti,
    }

    p := tea.NewProgram(initial_model,
                        tea.WithAltScreen(),
                        tea.WithMouseCellMotion())

    if _, err := p.Run(); err != nil {
        fmt.Printf("tea Error: %v", err)
        os.Exit(1)
    }
}
