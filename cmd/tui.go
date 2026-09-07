package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

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

	errMsg  error
	doneMsg struct{}
	keysMsg []string

	// secretsMsg reports the entries of a fetched secret together with its
	// metadata, already formatted for display.
	secretsMsg struct {
		secrets    []vaultSecret
		metaLines  []string
		customMeta map[string]any
	}

	// metadataMsg reports the custom metadata of the secrets listed at path,
	// keyed by secret name and already formatted for display.
	metadataMsg struct {
		path string
		meta map[string]string
	}
)

type state int

const (
	stateLoading state = iota
	stateList
	stateSecret
	stateAbort
	stateDone
)

// pathItem is a single entry of a vault path listing.
type pathItem struct {
	name string
	meta string
}

func (p pathItem) FilterValue() string { return p.name }
func (p pathItem) Title() string       { return p.name }
func (p pathItem) Description() string { return p.meta }

type model struct {
	width      int
	height     int
	state      state
	err        error
	spinner    spinner.Model
	list       list.Model
	secretList list.Model
	metaLines  []string
	customMeta map[string]any
	editor     secretEditor
	path       vaultPath
	fields     []string
	client     *api.Client
	secrets    []vaultSecret
	isDark     bool
	showDesc   bool
}

func newModel(client *api.Client, path vaultPath, fields []string) model {
	const isDark = true

	l := list.New(nil, newItemDelegate(isDark, false), 0, 0)
	l.Styles = newListStyles(isDark)
	l.DisableQuitKeybindings()
	l.SetShowStatusBar(false)
	l.InfiniteScrolling = true

	return model{
		state:      stateLoading,
		client:     client,
		path:       path,
		fields:     fields,
		spinner:    spinner.New(spinner.WithSpinner(spinner.Dot)),
		list:       l,
		secretList: newSecretList(isDark),
		isDark:     isDark,
	}
}

func newListStyles(isDark bool) list.Styles {
	lightDark := lipgloss.LightDark(isDark)
	s := list.DefaultStyles(isDark)
	s.TitleBar = s.TitleBar.Padding(0, 0, 1, 0)
	s.Title = lipgloss.NewStyle().
		Foreground(lightDark(lipgloss.Color("#5A56E0"), lipgloss.Color("#7571F9"))).
		Bold(true)
	return s
}

func newItemDelegate(isDark, showDesc bool) list.DefaultDelegate {
	lightDark := lipgloss.LightDark(isDark)

	d := list.NewDefaultDelegate()
	d.ShowDescription = showDesc
	d.SetSpacing(0)

	s := list.NewDefaultItemStyles(isDark)
	s.NormalTitle = s.NormalTitle.
		Foreground(lightDark(lipgloss.Color("235"), lipgloss.Color("252")))
	s.SelectedTitle = lipgloss.NewStyle().
		Border(lipgloss.Border{Left: ">"}, false, false, false, true).
		BorderForeground(lipgloss.Color("#F780E2")).
		Foreground(lightDark(lipgloss.Color("#02BA84"), lipgloss.Color("#02BF87"))).
		Padding(0, 0, 0, 1)
	s.SelectedDesc = s.SelectedTitle.
		Border(lipgloss.Border{Left: " "}, false, false, false, true).
		Foreground(lightDark(lipgloss.Color("#02CF92"), lipgloss.Color("#02A877")))
	d.Styles = s
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
		m.sizeSecretList()
		return m, nil
	case tea.BackgroundColorMsg:
		m.isDark = msg.IsDark()
		m.list.Styles = newListStyles(m.isDark)
		m.list.SetDelegate(newItemDelegate(m.isDark, m.showDesc))
		m.secretList.Styles = newListStyles(m.isDark)
		m.secretList.SetDelegate(newItemDelegate(m.isDark, false))
		return m, nil
	case keysMsg:
		m.state = stateList
		items := make([]list.Item, len(msg))
		for i, key := range msg {
			items[i] = pathItem{name: key}
		}
		m.setShowDescription(false)
		cmd := m.list.SetItems(items)
		m.list.Title = m.path.String()
		m.list.ResetFilter()
		m.list.ResetSelected()
		m.sizeList()
		return m, tea.Batch(cmd, fetchMetadataCmd(m.client, m.path, leafKeys(msg)))
	case metadataMsg:
		if msg.path != m.path.String() {
			// Stale result: the user already navigated to another path.
			return m, nil
		}
		var cmds []tea.Cmd
		for i, li := range m.list.Items() {
			if item, ok := li.(pathItem); ok {
				if meta, ok := msg.meta[item.name]; ok {
					item.meta = meta
					cmds = append(cmds, m.list.SetItem(i, item))
				}
			}
		}
		if len(msg.meta) > 0 {
			m.setShowDescription(true)
		}
		return m, tea.Batch(cmds...)
	case secretsMsg:
		cmd := m.applySecrets(msg)
		m.secretList.ResetSelected()
		return m, cmd
	case editDoneMsg:
		return m, m.applyEditDone(msg)
	case statusClearMsg:
		return m, m.clearStatus(msg.index)
	case totpMsg:
		return m, m.applyTOTP(msg)
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
		}
		if m.state == stateSecret && m.editor.mode != editNone {
			return m, m.updateEditor(msg)
		}
		switch msg.String() {
		case keyEsc:
			if m.state == stateList && !m.list.SettingFilter() {
				m.path.Back()
				return m, m.loadPath()
			}
			if m.state == stateSecret && !m.secretList.SettingFilter() {
				m.path.Back()
				return m, m.loadPath()
			}
		case keyEnter:
			if m.state == stateList && !m.list.SettingFilter() {
				if item, ok := m.list.SelectedItem().(pathItem); ok {
					m.path.Add(item.name)
					return m, m.loadPath()
				}
			}
			if m.state == stateSecret && !m.secretList.SettingFilter() {
				return m, m.toggleReveal()
			}
		case "s":
			if m.state == stateSecret && !m.secretList.SettingFilter() {
				return m, m.toggleReveal()
			}
		case "c":
			if m.state == stateSecret && !m.secretList.SettingFilter() {
				return m, m.copySelected()
			}
		case "t":
			if m.state == stateSecret && !m.secretList.SettingFilter() {
				return m, m.totpSelected()
			}
		case "p":
			if m.state == stateSecret && !m.secretList.SettingFilter() {
				return m, func() tea.Msg { return doneMsg{} }
			}
		case "e":
			if m.state == stateSecret && !m.secretList.SettingFilter() {
				return m, m.startEdit()
			}
		}
	}

	var cmd tea.Cmd
	switch m.state {
	case stateLoading:
		m.spinner, cmd = m.spinner.Update(msg)
	case stateList:
		m.list, cmd = m.list.Update(msg)
	case stateSecret:
		if m.editor.mode == editInput {
			m.editor.input, cmd = m.editor.input.Update(msg)
		} else {
			m.secretList, cmd = m.secretList.Update(msg)
		}
	case stateDone, stateAbort:
		return m, tea.Quit
	}

	return m, cmd
}

// applySecrets shows the given secret in the detail view. The selection is
// left untouched so callers can decide whether to keep or reset it.
func (m *model) applySecrets(msg secretsMsg) tea.Cmd {
	m.state = stateSecret
	m.secrets = msg.secrets
	m.metaLines = msg.metaLines
	m.customMeta = msg.customMeta
	items := make([]list.Item, len(msg.secrets))
	for i, secret := range msg.secrets {
		items[i] = secretEntry{vaultSecret: secret}
	}
	cmd := m.secretList.SetItems(items)
	m.secretList.ResetFilter()
	m.sizeSecretList()
	return cmd
}

// setShowDescription toggles the metadata descriptions of the list items and
// resizes the list to account for the changed item height.
func (m *model) setShowDescription(show bool) {
	if m.showDesc == show {
		return
	}
	m.showDesc = show
	m.list.SetDelegate(newItemDelegate(m.isDark, show))
	m.sizeList()
}

func (m *model) sizeList() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	itemHeight := 1
	if m.showDesc {
		itemHeight = 2
	}
	h := min(MAXHEIGHT, max(len(m.list.Items())*itemHeight, 1)+listChromeHeight, m.height-2)
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

// leafKeys returns the keys that are secrets rather than sub-paths.
func leafKeys(keys []string) []string {
	return lo.Filter(keys, func(key string, _ int) bool {
		return !strings.HasSuffix(key, "/")
	})
}

// ignoredMetadataFields are custom metadata fields that are not worth showing.
var ignoredMetadataFields = map[string]struct{}{
	"vault_orig_created_time":      {},
	"vault_orig_last_updated_time": {},
}

// formatCustomMetadata renders custom metadata as "key=value ..." with the
// keys in sorted order, or an empty string if there is none.
func formatCustomMetadata(meta map[string]any) string {
	keys := make([]string, 0, len(meta))
	for k := range meta {
		if _, ignored := ignoredMetadataFields[k]; !ignored {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return ""
	}
	sort.Strings(keys)
	pairs := make([]string, len(keys))
	for i, k := range keys {
		pairs[i] = fmt.Sprintf("%s=%v", k, meta[k])
	}
	return strings.Join(pairs, " ")
}

// fetchMetadataCmd loads the metadata of the given secrets and reports which of
// them have custom metadata attached. Individual failures are ignored so the
// listing stays usable even without metadata access.
func fetchMetadataCmd(client *api.Client, path vaultPath, keys []string) tea.Cmd {
	if len(keys) == 0 {
		return nil
	}
	engine := path.Engine()
	base := path.Path()
	pathStr := path.String()
	return func() tea.Msg {
		var (
			mu   sync.Mutex
			wg   sync.WaitGroup
			sem  = make(chan struct{}, 8)
			meta = make(map[string]string)
		)
		for _, key := range keys {
			wg.Add(1)
			go func(key string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				secretPath := key
				if base != "" {
					secretPath = base + "/" + key
				}
				md, err := client.KVv2(engine).GetMetadata(context.Background(), secretPath)
				if err != nil || md == nil {
					return
				}
				if formatted := formatCustomMetadata(md.CustomMetadata); formatted != "" {
					mu.Lock()
					meta[key] = formatted
					mu.Unlock()
				}
			}(key)
		}
		wg.Wait()
		return metadataMsg{path: pathStr, meta: meta}
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
	return secretsMsg{
		secrets:    vs,
		metaLines:  formatSecretMetaLines(secret.VersionMetadata, secret.CustomMetadata),
		customMeta: secret.CustomMetadata,
	}
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
	case stateSecret:
		return tea.NewView(m.secretView())
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
