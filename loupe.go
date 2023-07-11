package main

import (
        "fmt"
        "os"
        "os/exec"
        "strings"
        "bufio"
        tea "github.com/charmbracelet/bubbletea"
        lg "github.com/charmbracelet/lipgloss"
        "github.com/charmbracelet/bubbles/viewport"
        "github.com/charmbracelet/bubbles/textinput"
)

// this model will be used for BubbleTea state
type model struct {
    cmd *exec.Cmd
    stdout_scanner, stderr_scanner *bufio.Scanner
    stdout_lines, stderr_lines []string
    strace_lines string
    opened_files, connect_lines string
    selected_tab uint
    exit_code int
    ready bool
    vp viewport.Model
    stdin_ti textinput.Model
}

type StdOutMsg string
type StdErrMsg string
type ExitMsg int

func inhaler(pipeScanner *bufio.Scanner) string {
    if pipeScanner.Scan() {
        return pipeScanner.Text()
    }
    // FIXME: improve handling this case
    return ""
}

func inhaleStdOut(pipeScanner *bufio.Scanner) tea.Cmd {
    return func() tea.Msg {
        return StdOutMsg(inhaler(pipeScanner))
    }
}

func inhaleStdErr (pipeScanner *bufio.Scanner) tea.Cmd {
    return func() tea.Msg {
        return StdErrMsg(inhaler(pipeScanner))
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
            inhaleStdOut(m.stdout_scanner),
            inhaleStdErr(m.stderr_scanner),
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
        seba_cmd = inhaleStdOut(m.stdout_scanner)
    case StdErrMsg:
        m.stderr_lines = append(m.stderr_lines, string(msg))
        seba_cmd = inhaleStdErr(m.stderr_scanner)
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
        content = m.strace_lines
    case 3:
        content = m.opened_files
    case 4:
        content = m.connect_lines
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
    // FIXME: improve strace output handling
    // maybe with a fifo
    strace_file_path := "/tmp/loupe_strace"
    _args := []string{"strace", "--output", strace_file_path}

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
    // FIXME: find a better way to do this
    // strace_data, _ := os.ReadFile(strace_file_path)

    // openat, _ := exec.Command("sh", "-c", "awk '/openat/ {print $2}' /tmp/loupe_strace | sed 's/^\"//; s/\",$//' ").Output()
    // connects, _ := exec.Command("sh", "-c", "awk '/connect/ {print $0}' /tmp/loupe_strace").Output()

    // m.strace_lines = string(strace_data)
    // m.opened_files = string(openat)
    // m.connect_lines = string(connects)

