package main

import (
        "fmt"
        "os"
        "os/exec"
        "strings"
        tea "github.com/charmbracelet/bubbletea"
        lg "github.com/charmbracelet/lipgloss"
        "github.com/charmbracelet/bubbles/viewport"
        "github.com/charmbracelet/bubbles/textinput"

	    "github.com/franalbani/loupe/worker"
)

// this type serves as a Msg emitted
// when there are new lines in either stdout ot stderr
// err indicates which
type Notes struct {
    last_line string
    err bool
    stdout_ch, stderr_ch chan string
}

// this method watches for a new line in stdout or stderr
// and emits a new Note with it
func (n Notes) awaitNext() Notes {
    err := false
    last := ""
    select {
    case line := <- n.stdout_ch:
        last = line
    case line := <- n.stderr_ch:
        err = true
        last = line
    }
    return Notes{last_line: last,
                 err: err,
                 stdout_ch: n.stdout_ch,
                 stderr_ch: n.stderr_ch}
}

// this model will be used for BubbleTea state
type model struct {
    com string
    stdout_lines, stderr_lines []string
    strace_lines string
    opened_files, connect_lines string
    selected_tab uint
    exit_code int
    ready bool
    vp viewport.Model
    stdin_ti textinput.Model
}

// this method is required by BubbleTea
func (m *model) Init() tea.Cmd {

    // FIXME: improve strace output handling
    // maybe with a fifo
    strace_file_path := "/tmp/loupe_strace"
    _args := []string{"strace", "--output", strace_file_path}

    args := os.Args[1:]
    _args = append(_args, args...)

    stdout_ch := make(chan string)
    stderr_ch := make(chan string)

    cmd := exec.Command(_args[0], _args[1:]...)
    stdout_pipe, _ := cmd.StdoutPipe()
    stderr_pipe, _ := cmd.StderrPipe()
    go worker.Inhale(stdout_pipe, stdout_ch)
    go worker.Inhale(stderr_pipe, stderr_ch)
    cmd.Start()

    // TODO: go rutina para esperar el cierre y emitir exit_code

    ti := textinput.New()
    ti.Placeholder = "stdin"
    ti.Prompt = "$ "

    strace_data, _ := os.ReadFile(strace_file_path)

    openat, _ := exec.Command("sh", "-c", "awk '/openat/ {print $2}' /tmp/loupe_strace | sed 's/^\"//; s/\",$//' ").Output()
    connects, _ := exec.Command("sh", "-c", "awk '/connect/ {print $0}' /tmp/loupe_strace").Output()

    m.com = strings.Join(os.Args[1:], " ")
    m.strace_lines = string(strace_data)
    m.opened_files = string(openat)
    m.connect_lines = string(connects)
    m.selected_tab = 0
    m.stdin_ti = ti

    init_note := Notes{stdout_ch: stdout_ch, stderr_ch: stderr_ch}
	return func() tea.Msg { return init_note.awaitNext() }
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
                return &m, tea.Quit
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

	case Notes:
        switch msg.err {
        case false:
            m.stdout_lines = append(m.stdout_lines, msg.last_line)
        case true:
            m.stderr_lines = append(m.stderr_lines, msg.last_line)
        }
        seba_cmd = func() tea.Msg { return msg.awaitNext() }
	}

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

    return &m, tea.Batch(tiCmd, vp_cmd, seba_cmd)
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

    ec_color := "3"
    if m.exit_code != 0 {
        ec_color = "9"
    }
    ec_style := lg.NewStyle().Foreground(lg.Color(ec_color))
    s := lg.JoinVertical(lg.Left,
            tab_styles[false].Render(m.com),
            "| Exit code: " + ec_style.Render(fmt.Sprintf("%d", m.exit_code)),
            tab_header(m.selected_tab),
            m.vp.View(),
            m.stdin_ti.View(),
            help_footer.Render())
    return s
}

func main() {

    p := tea.NewProgram(&model{},
                        // tea.WithAltScreen(),
                        tea.WithMouseCellMotion())

    if _, err := p.Run(); err != nil {
        fmt.Printf("tea Error: %v", err)
        os.Exit(1)
    }
}
