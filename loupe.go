package main

import (
        "fmt"
        "bytes"
        "os"
        "os/exec"
        tea "github.com/charmbracelet/bubbletea"
        lg "github.com/charmbracelet/lipgloss"
        "github.com/charmbracelet/bubbles/viewport"
        "github.com/charmbracelet/bubbles/textinput"
)

// this model will be used for BubbleTea state
type model struct {
    com string
    stdout_lines, stderr_lines string
    selected_tab int
    exit_code int
    ready bool
    vp viewport.Model
    stdin_ti textinput.Model
}

// this function is required by BubbleTea
func (m model) Init() tea.Cmd {
    return textinput.Blink
}

// this function is required by BubbleTea
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var tiCmd, vp_cmd tea.Cmd

    switch msg := msg.(type) {

    case tea.KeyMsg:
        switch msg.String() {
            case "tab":
                m.selected_tab = (m.selected_tab + 1) % 2
            case "ctrl+c":
                return m, tea.Quit
        }

    case tea.WindowSizeMsg:
        if !m.ready {
            m.vp = viewport.New(msg.Width, msg.Height-10)
            m.vp.SetContent(m.stdout_lines)
            m.ready = true
        } else {
            m.vp.Width = msg.Width
            m.vp.Height = msg.Height
        }
    }
    m.vp, vp_cmd = m.vp.Update(msg)
    m.stdin_ti, tiCmd = m.stdin_ti.Update(msg)

    return m, tea.Batch(tiCmd, vp_cmd)
}

// this function is required by BubbleTea
func (m model) View() string {
    title_style := lg.NewStyle().
                      BorderStyle(lg.RoundedBorder()).
                      BorderForeground(lg.Color("63")).
                      Foreground(lg.Color("5"))

    selected_style := lg.NewStyle().
                         Bold(true).
                         BorderStyle(lg.RoundedBorder()).
                         BorderForeground(lg.Color("63")).
                         Foreground(lg.Color("86"))

    content_style := lg.NewStyle().
                        BorderStyle(lg.RoundedBorder()).
                        BorderForeground(lg.Color("63")).
                        Width(m.vp.Width - 2)

    tab_header := ""

    switch m.selected_tab {
    case 0:
        tab_header = lg.JoinHorizontal(lg.Bottom,
                                       selected_style.Render("stdout"),
                                       title_style.Render("stderr"),
                                       title_style.Render("syscalls"),
                                       title_style.Render("ports"),
                                   )
        m.vp.SetContent(m.stdout_lines)
    case 1:
        tab_header = lg.JoinHorizontal(lg.Bottom,
                                       title_style.Render("stdout"),
                                       selected_style.Render("stderr"),
                                       title_style.Render("syscalls"),
                                       title_style.Render("ports"),
                                   )
        m.vp.SetContent(m.stderr_lines)
    }
    s := tab_header + "\n" + m.com + "\n" + content_style.Render(m.vp.View()) + "\n"
    ec_color := "3"
    if m.exit_code != 0 {
        ec_color = "9"
    }

    ec_style := lg.NewStyle().Foreground(lg.Color(ec_color))
    s += "| Exit code: " + ec_style.Render(fmt.Sprintf("%d", m.exit_code)) + "\n"
    s += m.stdin_ti.View()
    help_style := lg.NewStyle().Foreground(lg.Color("#5C5C5C"))
    s += help_style.Render("\nTAB for switching, CTRL + C to quit.\n")
    return s
}

func main() {
    args := os.Args[1:]

    cmd := exec.Command(args[0], args[1:]...)

    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    // err := cmd.Run()
    cmd.Run()
    // if err != nil {
    //     fmt.Println(err)
    // }
    ti := textinput.New()
    ti.Placeholder = "stdin"
    ti.Prompt = "$ "
    initial_state := model{com: fmt.Sprintf("%v", os.Args[1:]),
                           stdout_lines: stdout.String(),
                           stderr_lines: stderr.String(),
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
