package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/openbao/openbao/api/v2"
	"github.com/samber/lo"
)

// MAXHEIGHT is the maximum height of the list, including its chrome.
const MAXHEIGHT = 20

// listChromeHeight is the number of rows the list needs around the items:
// title (2), pagination (2) and help (2). The status bar is disabled.
const listChromeHeight = 6

type (
	vaultSecret struct {
		key   string
		value any
	}

	errMsg     error
	doneMsg    struct{}
	secretsMsg []vaultSecret
	keysMsg    []string
)

type state int

const (
	stateLoading state = iota
	stateList
	stateAbort
	stateDone
)

// pathItem is a single entry of a vault path listing.
type pathItem string

func (p pathItem) FilterValue() string { return string(p) }
func (p pathItem) Title() string       { return string(p) }
func (p pathItem) Description() string { return "" }

type model struct {
	width   int
	height  int
	state   state
	err     error
	spinner spinner.Model
	list    list.Model
	path    vaultPath
	fields  []string
	client  *api.Client
	secrets []vaultSecret
}

func newModel(client *api.Client, path vaultPath, fields []string) model {
	const isDark = true

	l := list.New(nil, newItemDelegate(isDark), 0, 0)
	l.Styles = list.DefaultStyles(isDark)
	l.DisableQuitKeybindings()
	l.SetShowStatusBar(false)

	return model{
		state:   stateLoading,
		client:  client,
		path:    path,
		fields:  fields,
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
		list:    l,
	}
}

func newItemDelegate(isDark bool) list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.ShowDescription = false
	d.SetSpacing(0)
	d.Styles = list.NewDefaultItemStyles(isDark)
	return d
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tea.RequestBackgroundColor,
		m.spinner.Tick,
		listPathsCmd(m.client, m.path),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.sizeList()
		return m, nil
	case tea.BackgroundColorMsg:
		isDark := msg.IsDark()
		m.list.Styles = list.DefaultStyles(isDark)
		m.list.SetDelegate(newItemDelegate(isDark))
		return m, nil
	case keysMsg:
		m.state = stateList
		items := make([]list.Item, len(msg))
		for i, key := range msg {
			items[i] = pathItem(key)
		}
		cmd := m.list.SetItems(items)
		m.list.Title = m.path.String()
		m.list.ResetFilter()
		m.list.ResetSelected()
		m.sizeList()
		return m, cmd
	case secretsMsg:
		m.secrets = msg
		return m, func() tea.Msg { return doneMsg{} }
	case doneMsg:
		if m.state != stateAbort {
			m.state = stateDone
		}
	case errMsg:
		m.err = msg
		return m, func() tea.Msg { return doneMsg{} }
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			m.state = stateAbort
			return m, func() tea.Msg { return doneMsg{} }
		case "esc":
			if m.state == stateList && !m.list.SettingFilter() {
				m.path.Back()
				return m, m.loadPath()
			}
		case "enter":
			if m.state == stateList && !m.list.SettingFilter() {
				if item, ok := m.list.SelectedItem().(pathItem); ok {
					m.path.Add(string(item))
					return m, m.loadPath()
				}
			}
		}
	}

	var cmd tea.Cmd
	switch m.state {
	case stateLoading:
		m.spinner, cmd = m.spinner.Update(msg)
	case stateList:
		m.list, cmd = m.list.Update(msg)
	case stateDone, stateAbort:
		return m, tea.Quit
	}

	return m, cmd
}

func (m *model) sizeList() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	h := min(MAXHEIGHT, max(len(m.list.Items()), 1)+listChromeHeight, m.height-2)
	m.list.SetSize(m.width, h)
}

func (m *model) loadPath() tea.Cmd {
	m.state = stateLoading
	return tea.Batch(
		m.spinner.Tick,
		listPathsCmd(m.client, m.path),
	)
}

func listPathsCmd(client *api.Client, path vaultPath) tea.Cmd {
	return func() tea.Msg {
		secret, err := client.Logical().List(path.MetadataPath())
		if err != nil {
			return errMsg(err)
		}
		if secret == nil {
			return listSecret(client, path)
		}
		keys, ok := secret.Data["keys"].([]any)
		if !ok {
			return errMsg(fmt.Errorf("failed to convert keys: %v", secret.Data["keys"]))
		}
		availableKeys := make([]string, len(keys))
		for i, key := range keys {
			availableKeys[i] = key.(string)
		}
		return keysMsg(availableKeys)
	}
}

func listSecret(client *api.Client, path vaultPath) tea.Msg {
	secret, err := client.KVv2(path.Engine()).Get(context.Background(), path.Path())
	if err != nil {
		return errMsg(err)
	}
	if secret == nil {
		return errMsg(fmt.Errorf("secret not found"))
	}
	vs := make([]vaultSecret, 0, len(secret.Data))
	for k, v := range secret.Data {
		vs = append(vs, vaultSecret{key: k, value: v})
	}
	sort.Slice(vs, func(i, j int) bool {
		return vs[i].key < vs[j].key
	})
	return secretsMsg(vs)
}

var errStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("204")).Bold(true)

func (m model) View() tea.View {
	if m.err != nil {
		return tea.NewView("")
	}
	switch m.state {
	case stateLoading:
		return tea.NewView(m.spinner.View() + "Fetching secrets...")
	case stateList:
		return tea.NewView(m.list.View())
	case stateAbort, stateDone:
		// The result is printed after the program exits, since frames larger
		// than the terminal would be cut off by the renderer.
		return tea.NewView("")
	}
	return tea.NewView("uwu")
}

// output returns the result of the run, printed once the program has exited.
func (m model) output() string {
	if m.err != nil {
		return errStyle.Render("Error: ") + m.err.Error() + "\n"
	}
	if m.state == stateDone {
		return m.printSecrets()
	}
	return ""
}

func (m model) printSecrets() string {
	var s strings.Builder
	fmt.Fprintf(&s, "Path: %s\n", m.path.String())
	for _, secret := range m.secrets {
		if len(m.fields) == 0 || lo.Contains(m.fields, secret.key) {
			fmt.Fprintf(&s, "%s: %v\n", secret.key, secret.value)
		}
	}
	return s.String()
}
