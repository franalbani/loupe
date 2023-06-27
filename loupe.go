package main

import (
        "fmt"
        "bytes"
        "os"
        "os/exec"
        "strings"
        tea "github.com/charmbracelet/bubbletea"
        lg "github.com/charmbracelet/lipgloss"
        "github.com/charmbracelet/bubbles/viewport"
        "github.com/charmbracelet/bubbles/textinput"
)

// this model will be used for BubbleTea state
type model struct {
    com string
    stdout_lines, stderr_lines, strace_lines string
    opened_files string
    selected_tab uint
    exit_code int
    ready bool
    vp viewport.Model
    stdin_ti textinput.Model
}

// this method is required by BubbleTea
func (m model) Init() tea.Cmd {
    return textinput.Blink
}

// this method is required by BubbleTea
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var tiCmd, vp_cmd tea.Cmd

    switch msg := msg.(type) {

    case tea.KeyMsg:
        switch msg.String() {
            case "tab":
                m.selected_tab = (m.selected_tab + 1) % 4
            case "shift+tab":
                m.selected_tab = (m.selected_tab - 1) % 4
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
    }

    content := ""
    switch m.selected_tab {
    case 0:
        content = m.stdout_lines
    case 1:
        content = m.stderr_lines
    case 2:
        content = m.strace_lines
    case 3:
        content = m.opened_files
    default:
        content = "soon"
    }
    m.vp.SetContent(content)

    m.vp, vp_cmd = m.vp.Update(msg)
    m.stdin_ti, tiCmd = m.stdin_ti.Update(msg)

    return m, tea.Batch(tiCmd, vp_cmd)
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
    // FIXME: improve strace output handling
    // maybe with a fifo
    strace_file_path := "/tmp/loupe_strace"
    _args := []string{"strace", "--output", strace_file_path}

    args := os.Args[1:]
    _args = append(_args, args...)
    cmd := exec.Command(_args[0], _args[1:]...)

    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    cmd.Run()
    ti := textinput.New()
    ti.Placeholder = "stdin"
    ti.Prompt = "$ "

    strace_data, _ := os.ReadFile(strace_file_path)

    openat, _ := exec.Command("sh", "-c", "awk '/openat/ {print $2}' /tmp/loupe_strace | sed 's/^\"//; s/\",$//' ").Output()

    initial_state := model{com: strings.Join(os.Args[1:], " "),
                           stdout_lines: stdout.String(),
                           stderr_lines: stderr.String(),
                           strace_lines: string(strace_data),
                           opened_files: string(openat),
                           selected_tab: 0,
                           exit_code: cmd.ProcessState.ExitCode(),
                           stdin_ti: ti,
                           }

    p := tea.NewProgram(initial_state,
                        tea.WithAltScreen(),
                        tea.WithMouseCellMotion())

    if _, err := p.Run(); err != nil {
        fmt.Printf("tea Error: %v", err)
        os.Exit(1)
    }
}
