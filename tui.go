package main

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/openbao/openbao/api/v2"
	"github.com/samber/lo"
)

const MAXHEIGHT = 20

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
	stateForm
	stateAbort
	stateDone
)

type model struct {
	width     int
	height    int
	state     state
	err       error
	spinner   *spinner.Spinner
	form      *huh.Form
	huhSelect *huh.Select[string]
	options   []string
	path      vaultPath
	fields    []string
	client    *api.Client
	secrets   []vaultSecret
}

func newModel(client *api.Client, path vaultPath, fields []string) model {
	return model{
		state:   stateLoading,
		client:  client,
		path:    path,
		fields:  fields,
		spinner: spinner.New().Title("Fetching secrets..."),
		form:    huh.NewForm(),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Init(),
		listPathsCmd(m.client, m.path),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		return m, m.updateHeight()
	case keysMsg:
		m.state = stateForm
		m.options = msg
		return m, m.updateHeight()
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
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.state = stateAbort
			return m, func() tea.Msg { return doneMsg{} }
		case "esc":
			if m.isFiltering() {
				break
			}
			m.path.Back()
			m.state = stateLoading
			return m, tea.Batch(
				m.spinner.Init(),
				listPathsCmd(m.client, m.path),
			)
		}
	}

	var cmds []tea.Cmd
	switch m.state {
	case stateLoading:
		spn, cmd := m.spinner.Update(msg)
		if sp, ok := spn.(*spinner.Spinner); ok {
			m.spinner = sp
			cmds = append(cmds, cmd)
		}
	case stateForm:
		// Process the form
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			break
		default:
			form, cmd := m.form.Update(msg)
			if f, ok := form.(*huh.Form); ok {
				m.form = f
				cmds = append(cmds, cmd)
			}
			if m.form.State == huh.StateCompleted {
				m.path.Add(m.form.GetString(""))
				m.state = stateLoading
				cmds = append(cmds, m.spinner.Init(), listPathsCmd(m.client, m.path))
			}
		}
	case stateDone, stateAbort:
		return m, tea.Quit
	}

	return m, tea.Batch(cmds...)
}

// hacky since huh doesn't expose a way to check if filtering is enabled yet
func (m model) isFiltering() bool {
	val := reflect.ValueOf(m.huhSelect).Elem().FieldByName("filtering")
	return val.Bool()
}

func (m *model) updateHeight() tea.Cmd {
	huhSelect := huh.NewSelect[string]().
		Title(m.path.String()).
		Options(huh.NewOptions(m.options...)...).
		WithHeight(
			min(MAXHEIGHT, len(m.options)+2, m.height-3),
		)

	m.huhSelect = huhSelect.(*huh.Select[string])

	m.form = huh.NewForm(
		huh.NewGroup(
			huhSelect,
		),
	).WithWidth(m.width)

	return m.form.Init()
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

func (m model) View() string {
	if m.err != nil {
		return errStyle.Render("Error: ") + m.err.Error()
	}
	switch m.state {
	case stateLoading:
		return m.spinner.View()
	case stateForm:
		return m.form.View()
	case stateAbort:
		return ""
	case stateDone:
		return m.printSecrets()
	}
	return "uwu"
}

func (m model) printSecrets() string {
	var s strings.Builder
	s.WriteString(fmt.Sprintf("Path: %s\n", m.path.String()))
	for _, secret := range m.secrets {
		if len(m.fields) == 0 || lo.Contains(m.fields, secret.key) {
			s.WriteString(fmt.Sprintf("%s: %v\n", secret.key, secret.value))
		}
	}
	return s.String()
}
