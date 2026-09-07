package cmd

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jon4hz/awoolt/put"
	"github.com/openbao/openbao/api/v2"
)

// Indicators shown next to an entry while and after saving an edit.
const (
	statusSaving = "saving"
	statusSaved  = "saved"
)

// Key names as reported by tea.KeyPressMsg.String().
const (
	keyEsc   = "esc"
	keyEnter = "enter"
)

// editMode is the step of the edit flow the secret view is in.
type editMode int

const (
	editNone    editMode = iota
	editInput            // the value is being typed
	editConfirm          // the diff is shown and waits for confirmation
)

// secretEditor holds the state of an in-progress edit of a single entry.
type secretEditor struct {
	mode     editMode
	input    textinput.Model
	index    int
	key      string
	oldValue string
	// oldMeta is the current custom metadata value of the field, if mirrored.
	oldMeta string
	// updateMeta is set when the field is mirrored in the custom metadata, in
	// which case the metadata is patched too.
	updateMeta bool
}

// editDoneMsg reports the outcome of a patch. On success, secrets holds the
// re-fetched secret.
type editDoneMsg struct {
	index   int
	err     error
	secrets secretsMsg
}

var (
	diffRemovedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("204"))
	diffAddedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("78"))
	editHelpStyle    = lipgloss.NewStyle().Faint(true)
)

// startEdit opens the input for the selected entry, prefilled with its value.
func (m *model) startEdit() tea.Cmd {
	e, ok := m.secretList.SelectedItem().(secretEntry)
	if !ok {
		return nil
	}
	in := textinput.New()
	in.Prompt = ""
	in.SetValue(fmt.Sprint(e.value))
	in.CursorEnd()
	in.SetWidth(max(m.width-len(e.key)-4, 10))

	meta, mirrored := m.customMeta[e.key]
	m.editor = secretEditor{
		mode:       editInput,
		input:      in,
		index:      m.secretList.GlobalIndex(),
		key:        e.key,
		oldValue:   fmt.Sprint(e.value),
		oldMeta:    fmt.Sprint(meta),
		updateMeta: mirrored,
	}
	m.sizeSecretList()
	return m.editor.input.Focus()
}

func (m *model) cancelEdit() {
	m.editor = secretEditor{}
	m.sizeSecretList()
}

// updateEditor handles key presses while an edit is in progress.
func (m *model) updateEditor(msg tea.KeyPressMsg) tea.Cmd {
	switch m.editor.mode {
	case editInput:
		switch msg.String() {
		case keyEsc:
			m.cancelEdit()
			return nil
		case keyEnter:
			if m.editor.input.Value() == m.editor.oldValue {
				m.cancelEdit()
				return nil
			}
			m.editor.input.Blur()
			m.editor.mode = editConfirm
			m.sizeSecretList()
			return nil
		}
		var cmd tea.Cmd
		m.editor.input, cmd = m.editor.input.Update(msg)
		return cmd
	case editConfirm:
		switch msg.String() {
		case "y", keyEnter:
			ed := m.editor
			m.cancelEdit()
			return tea.Batch(
				m.setStatus(ed.index, statusSaving, false, statusErrorDuration),
				patchFieldCmd(m.client, m.path, ed.index, ed.key, ed.input.Value(), ed.updateMeta),
			)
		case "n", keyEsc:
			m.cancelEdit()
			return nil
		}
	case editNone:
	}
	return nil
}

// editorHeight is the number of rows the editor takes below the list.
func (m model) editorHeight() int {
	switch m.editor.mode {
	case editInput:
		return 3
	case editConfirm:
		return len(m.diffLines()) + 2
	case editNone:
	}
	return 0
}

// diffLines renders the pending change as a unified-style diff.
func (m model) diffLines() []string {
	ed := m.editor
	newValue := ed.input.Value()
	lines := []string{
		diffRemovedStyle.Render("- " + ed.key + ": " + ed.oldValue),
		diffAddedStyle.Render("+ " + ed.key + ": " + newValue),
	}
	if ed.updateMeta {
		lines = append(lines,
			diffRemovedStyle.Render("- metadata "+ed.key+"="+ed.oldMeta),
			diffAddedStyle.Render("+ metadata "+ed.key+"="+newValue),
		)
	}
	return lines
}

// editorView renders the input or the confirmation prompt, or an empty string
// if no edit is in progress.
func (m model) editorView() string {
	switch m.editor.mode {
	case editInput:
		return "\n" + m.editor.key + ": " + m.editor.input.View() +
			"\n" + editHelpStyle.Render("enter confirm · esc cancel")
	case editConfirm:
		return "\n" + strings.Join(m.diffLines(), "\n") +
			"\n" + editHelpStyle.Render("apply change? y/n")
	case editNone:
	}
	return ""
}

// applyEditDone shows the outcome of a patch on the edited entry and, on
// success, replaces the shown secret with the re-fetched one.
func (m *model) applyEditDone(msg editDoneMsg) tea.Cmd {
	if msg.err != nil {
		return m.setStatus(msg.index, "save failed: "+shortError(msg.err), true, statusErrorDuration)
	}
	cmd := m.applySecrets(msg.secrets)
	m.secretList.Select(msg.index)
	return tea.Batch(cmd, m.setStatus(msg.index, statusSaved, false, statusDuration))
}

// shortError reduces a (possibly multi-line) API error to its last line, which
// for vault errors is the actual reason.
func shortError(err error) string {
	lines := strings.Split(strings.TrimSpace(err.Error()), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	return strings.TrimPrefix(last, "* ")
}

// patchFieldCmd patches a single field of the secret at path and re-fetches
// the secret afterwards, so the view shows the new version.
func patchFieldCmd(client *api.Client, path vaultPath, index int, key, value string, updateMeta bool) tea.Cmd {
	return func() tea.Msg {
		kv := client.KVv2(path.Engine())
		if err := put.PatchField(context.Background(), kv, path.Path(), key, value, updateMeta); err != nil {
			return editDoneMsg{index: index, err: err}
		}
		switch msg := listSecret(client, path).(type) {
		case secretsMsg:
			return editDoneMsg{index: index, secrets: msg}
		case errMsg:
			return editDoneMsg{index: index, err: msg}
		default:
			return editDoneMsg{index: index, err: fmt.Errorf("unexpected reload result %T", msg)}
		}
	}
}
