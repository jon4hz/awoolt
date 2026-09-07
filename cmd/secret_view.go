package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/openbao/openbao/api/v2"
)

// maskedValue hides secret values with a fixed width so the mask does not leak
// the value length.
const maskedValue = "********"

// How long a status indicator stays on an entry. Failures are shown longer,
// since they carry a message that has to be read.
const (
	statusDuration      = 1500 * time.Millisecond
	statusErrorDuration = 4 * time.Second
)

// Indicators shown next to an entry after an action.
const (
	statusCopied     = "copied"
	statusTOTPCopied = "totp copied"
)

// secretChromeHeight is the number of rows the secret list needs around its
// items: pagination (2) and help (2). The title is rendered outside the list.
const secretChromeHeight = 4

// statusClearMsg removes the status indicator from the entry at index.
type statusClearMsg struct{ index int }

// statusErrorStyle colors failed actions, since the entry itself is rendered in
// the list's regular colors.
var statusErrorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("204")).Bold(true)

// secretEntry is a single key/value pair of a secret, shown in the detail view.
type secretEntry struct {
	vaultSecret
	revealed bool
	// status is a short-lived indicator shown behind the value, e.g. "copied".
	status string
	// statusErr marks the status as a failure, which is rendered in red.
	statusErr bool
}

func (e secretEntry) FilterValue() string { return e.key }
func (e secretEntry) Description() string { return "" }

func (e secretEntry) Title() string {
	value := maskedValue
	if e.revealed {
		value = fmt.Sprint(e.value)
	}
	title := e.key + ": " + value
	if e.status != "" {
		status := "(" + e.status + ")"
		if e.statusErr {
			// The status is last in the title, so the color reset at the end of
			// the styled span cannot bleed into the rest of the line.
			status = statusErrorStyle.Render(status)
		}
		title += " " + status
	}
	return title
}

// formatSecretMetaLines renders the version and custom metadata of a secret as
// display lines, or nil if there is no metadata worth showing.
func formatSecretMetaLines(vm *api.KVVersionMetadata, custom map[string]any) []string {
	var lines []string
	if vm != nil {
		lines = append(lines, fmt.Sprintf("version %d · created %s",
			vm.Version, vm.CreatedTime.UTC().Format(time.RFC3339)))
	}
	if formatted := formatCustomMetadata(custom); formatted != "" {
		lines = append(lines, formatted)
	}
	return lines
}

var secretViewKeys = []key.Binding{
	key.NewBinding(key.WithKeys("s", "enter"), key.WithHelp("s/enter", "show")),
	key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "copy")),
	key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "copy totp")),
	key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
	key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "print")),
	key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
}

func newSecretList(isDark bool) list.Model {
	l := list.New(nil, newItemDelegate(isDark, false), 0, 0)
	l.Styles = newListStyles(isDark)
	l.DisableQuitKeybindings()
	l.SetShowStatusBar(false)
	l.SetShowTitle(false)
	// No filtering: a secret has few entries, and disabling it keeps the
	// title bar (which doubles as the filter input) out of the layout.
	l.SetFilteringEnabled(false)
	l.InfiniteScrolling = true
	l.AdditionalShortHelpKeys = func() []key.Binding { return secretViewKeys }
	l.AdditionalFullHelpKeys = func() []key.Binding { return secretViewKeys }
	return l
}

var metaLineStyle = lipgloss.NewStyle().Faint(true)

// secretView renders the detail view of a secret: the path, its metadata and
// the list of masked entries.
func (m model) secretView() string {
	var s strings.Builder
	s.WriteString(newListStyles(m.isDark).Title.Render(m.path.String()))
	for _, line := range m.metaLines {
		s.WriteByte('\n')
		s.WriteString(metaLineStyle.Render(line))
	}
	s.WriteByte('\n')
	s.WriteString(m.secretList.View())
	s.WriteString(m.editorView())
	return s.String()
}

// toggleReveal shows or hides the value of the selected entry.
func (m *model) toggleReveal() tea.Cmd {
	e, ok := m.secretList.SelectedItem().(secretEntry)
	if !ok {
		return nil
	}
	e.revealed = !e.revealed
	return m.secretList.SetItem(m.secretList.GlobalIndex(), e)
}

// copySelected copies the value of the selected entry to the clipboard and
// marks the entry as copied until the indicator expires.
func (m *model) copySelected() tea.Cmd {
	e, ok := m.secretList.SelectedItem().(secretEntry)
	if !ok {
		return nil
	}
	index := m.secretList.GlobalIndex()
	return tea.Batch(
		m.setStatus(index, statusCopied, false, statusDuration),
		tea.SetClipboard(fmt.Sprint(e.value)),
	)
}

// totpSelected generates a TOTP token from the value of the selected entry,
// which is expected to be a base32 encoded shared secret.
func (m *model) totpSelected() tea.Cmd {
	e, ok := m.secretList.SelectedItem().(secretEntry)
	if !ok {
		return nil
	}
	return totpCmd(m.secretList.GlobalIndex(), fmt.Sprint(e.value))
}

// applyTOTP copies a generated token to the clipboard, or shows why generating
// it failed.
func (m *model) applyTOTP(msg totpMsg) tea.Cmd {
	if msg.err != nil {
		return m.setStatus(msg.index, humanTOTPError(msg.err), true, statusErrorDuration)
	}
	return tea.Batch(
		m.setStatus(msg.index, statusTOTPCopied, false, statusDuration),
		tea.SetClipboard(msg.token),
	)
}

// setStatus shows an indicator on the entry at index and clears it again after
// the given duration.
func (m *model) setStatus(index int, status string, isErr bool, d time.Duration) tea.Cmd {
	items := m.secretList.Items()
	if index < 0 || index >= len(items) {
		return nil
	}
	e, ok := items[index].(secretEntry)
	if !ok {
		return nil
	}
	e.status = status
	e.statusErr = isErr
	return tea.Batch(
		m.secretList.SetItem(index, e),
		tea.Tick(d, func(time.Time) tea.Msg {
			return statusClearMsg{index: index}
		}),
	)
}

// clearStatus removes the status indicator from the entry at index.
func (m *model) clearStatus(index int) tea.Cmd {
	items := m.secretList.Items()
	if index < 0 || index >= len(items) {
		return nil
	}
	e, ok := items[index].(secretEntry)
	if !ok || e.status == "" {
		return nil
	}
	e.status = ""
	e.statusErr = false
	return m.secretList.SetItem(index, e)
}

// oathtoolBin generates the TOTP tokens. It is looked up in PATH.
const oathtoolBin = "oathtool"

// totpTimeout bounds the oathtool call so a hanging binary cannot freeze the
// TUI, which has no way to cancel the command.
const totpTimeout = 5 * time.Second

// totpMsg reports the token generated for the entry at index.
type totpMsg struct {
	index int
	token string
	err   error
}

// totpCmd runs "oathtool -b --totp <secret>" and reports the resulting token.
//
// The secret is passed as an argument because oathtool takes it no other way,
// which makes it visible to other users in the process list while the command
// runs.
func totpCmd(index int, secret string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), totpTimeout)
		defer cancel()

		var stderr bytes.Buffer
		cmd := exec.CommandContext(ctx, oathtoolBin, "-b", "--totp", secret)
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			return totpMsg{index: index, err: oathtoolError(err, stderr.String())}
		}

		token := strings.TrimSpace(string(out))
		if token == "" {
			return totpMsg{index: index, err: errors.New("oathtool returned no token")}
		}
		return totpMsg{index: index, token: token}
	}
}

// Failure modes of the oathtool call, so the view can show a short message
// while the underlying error stays available for wrapping.
var (
	errOathtoolMissing = errors.New("oathtool not installed")
	errInvalidSecret   = errors.New("not a base32 secret")
	errTOTPTimeout     = errors.New("oathtool timed out")
	errTOTPFailed      = errors.New("totp failed")
)

// oathtoolError classifies a failed oathtool call into one of the known failure
// modes, keeping the original error and stderr as context.
func oathtoolError(err error, stderr string) error {
	stderr = strings.TrimSpace(stderr)
	switch {
	case errors.Is(err, exec.ErrNotFound):
		return errOathtoolMissing
	case errors.Is(err, context.DeadlineExceeded):
		return errTOTPTimeout
	case strings.Contains(strings.ToLower(stderr), "base32"):
		return errInvalidSecret
	}
	if stderr != "" {
		return fmt.Errorf("%w: %s", errTOTPFailed, firstLine(stderr))
	}
	return fmt.Errorf("%w: %w", errTOTPFailed, err)
}

// humanTOTPError renders a TOTP failure as an indicator short enough to fit
// behind a list entry.
func humanTOTPError(err error) string {
	for _, known := range []error{errOathtoolMissing, errInvalidSecret, errTOTPTimeout} {
		if errors.Is(err, known) {
			return known.Error()
		}
	}
	return errTOTPFailed.Error()
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

func (m *model) sizeSecretList() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	headerHeight := 1 + len(m.metaLines)
	h := min(MAXHEIGHT, max(len(m.secretList.Items()), 1)+secretChromeHeight, m.height-headerHeight-m.editorHeight()-2)
	m.secretList.SetSize(m.width, h)
}
