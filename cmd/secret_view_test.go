package cmd

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/openbao/openbao/api/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecretEntry(t *testing.T) {
	t.Run("value is masked by default", func(t *testing.T) {
		e := secretEntry{vaultSecret: vaultSecret{key: "password", value: "hunter2"}}
		assert.Equal(t, "password: ********", e.Title())
	})

	t.Run("mask does not leak the value length", func(t *testing.T) {
		short := secretEntry{vaultSecret: vaultSecret{key: "a", value: "x"}}
		long := secretEntry{vaultSecret: vaultSecret{key: "a", value: "a-very-long-secret-value"}}
		assert.Equal(t, short.Title(), long.Title())
	})

	t.Run("revealed shows the value", func(t *testing.T) {
		e := secretEntry{vaultSecret: vaultSecret{key: "password", value: "hunter2"}, revealed: true}
		assert.Equal(t, "password: hunter2", e.Title())
	})

	t.Run("non-string values are formatted", func(t *testing.T) {
		e := secretEntry{vaultSecret: vaultSecret{key: "port", value: 5432}, revealed: true}
		assert.Equal(t, "port: 5432", e.Title())
	})

	t.Run("copied indicator is shown", func(t *testing.T) {
		e := secretEntry{vaultSecret: vaultSecret{key: "password", value: "hunter2"}, status: statusCopied}
		assert.Equal(t, "password: ******** (copied)", e.Title())
	})

	t.Run("totp indicator is shown", func(t *testing.T) {
		e := secretEntry{vaultSecret: vaultSecret{key: "otp", value: "GEZDGNBV"}, status: statusTOTPCopied}
		assert.Equal(t, "otp: ******** (totp copied)", e.Title())
	})

	t.Run("filter value is the key", func(t *testing.T) {
		e := secretEntry{vaultSecret: vaultSecret{key: "password", value: "hunter2"}}
		assert.Equal(t, "password", e.FilterValue())
	})
}

func TestFormatSecretMetaLines(t *testing.T) {
	created := time.Date(2026, 8, 20, 10, 30, 0, 0, time.UTC)

	t.Run("nil metadata", func(t *testing.T) {
		assert.Empty(t, formatSecretMetaLines(nil, nil))
	})

	t.Run("version metadata only", func(t *testing.T) {
		vm := &api.KVVersionMetadata{Version: 3, CreatedTime: created}
		assert.Equal(t,
			[]string{"version 3 · created 2026-08-20T10:30:00Z"},
			formatSecretMetaLines(vm, nil),
		)
	})

	t.Run("custom metadata only", func(t *testing.T) {
		assert.Equal(t,
			[]string{"owner=myuser team=infra"},
			formatSecretMetaLines(nil, map[string]any{"team": "infra", "owner": "myuser"}),
		)
	})

	t.Run("version and custom metadata", func(t *testing.T) {
		vm := &api.KVVersionMetadata{Version: 1, CreatedTime: created}
		assert.Equal(t,
			[]string{"version 1 · created 2026-08-20T10:30:00Z", "owner=myuser"},
			formatSecretMetaLines(vm, map[string]any{"owner": "myuser"}),
		)
	})

	t.Run("ignored custom metadata fields are skipped", func(t *testing.T) {
		assert.Empty(t, formatSecretMetaLines(nil, map[string]any{
			"vault_orig_created_time": "2020-01-01T00:00:00Z",
		}))
	})
}

func TestOathtoolError(t *testing.T) {
	t.Run("missing binary", func(t *testing.T) {
		err := oathtoolError(exec.ErrNotFound, "")
		assert.ErrorIs(t, err, errOathtoolMissing)
		assert.Equal(t, "oathtool not installed", humanTOTPError(err))
	})

	t.Run("timeout", func(t *testing.T) {
		err := oathtoolError(context.DeadlineExceeded, "")
		assert.ErrorIs(t, err, errTOTPTimeout)
		assert.Equal(t, "oathtool timed out", humanTOTPError(err))
	})

	t.Run("invalid secret", func(t *testing.T) {
		err := oathtoolError(errors.New("exit status 1"), "oathtool: base32 decoding failed\n")
		assert.ErrorIs(t, err, errInvalidSecret)
		assert.Equal(t, "not a base32 secret", humanTOTPError(err))
	})

	t.Run("unknown failures keep stderr as context but show a short status", func(t *testing.T) {
		err := oathtoolError(errors.New("exit status 1"), "something went wrong\nusage: oathtool\n")
		assert.ErrorIs(t, err, errTOTPFailed)
		assert.Contains(t, err.Error(), "something went wrong")
		assert.NotContains(t, err.Error(), "usage:")
		assert.Equal(t, "totp failed", humanTOTPError(err))
	})

	t.Run("falls back to the exec error", func(t *testing.T) {
		wrapped := errors.New("exit status 1")
		err := oathtoolError(wrapped, "  \n")
		assert.ErrorIs(t, err, errTOTPFailed)
		assert.ErrorIs(t, err, wrapped)
		assert.Equal(t, "totp failed", humanTOTPError(err))
	})
}

// newSecretViewModel returns a model that has received a secret and shows the
// detail view.
func newSecretViewModel(t *testing.T) model {
	t.Helper()
	m := newModel(nil, vaultPath{"secret", "db"}, nil)
	m.width = 80
	m.height = 40
	next, _ := m.Update(secretsMsg{
		secrets: []vaultSecret{
			{key: "password", value: "hunter2"},
			{key: "user", value: "myuser"},
		},
		metaLines: []string{"version 3 · created 2026-08-20T10:30:00Z"},
	})
	nm, ok := next.(model)
	require.True(t, ok)
	return nm
}

func keyPress(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	default:
		return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
	}
}

func update(t *testing.T, m model, msg tea.Msg) (model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	nm, ok := next.(model)
	require.True(t, ok)
	return nm, cmd
}

func selectedEntry(t *testing.T, m model) secretEntry {
	t.Helper()
	e, ok := m.secretList.SelectedItem().(secretEntry)
	require.True(t, ok)
	return e
}

func TestSecretView(t *testing.T) {
	t.Run("secretsMsg opens the detail view", func(t *testing.T) {
		m := newSecretViewModel(t)
		assert.Equal(t, stateSecret, m.state)
		assert.Len(t, m.secretList.Items(), 2)
		assert.Equal(t, []string{"version 3 · created 2026-08-20T10:30:00Z"}, m.metaLines)
	})

	t.Run("s toggles reveal of the selected entry", func(t *testing.T) {
		m := newSecretViewModel(t)
		m, _ = update(t, m, keyPress("s"))
		assert.True(t, selectedEntry(t, m).revealed)
		m, _ = update(t, m, keyPress("s"))
		assert.False(t, selectedEntry(t, m).revealed)
	})

	t.Run("enter toggles reveal of the selected entry", func(t *testing.T) {
		m := newSecretViewModel(t)
		m, _ = update(t, m, keyPress("enter"))
		assert.True(t, selectedEntry(t, m).revealed)
	})

	t.Run("c marks the selected entry as copied and returns a command", func(t *testing.T) {
		m := newSecretViewModel(t)
		m, cmd := update(t, m, keyPress("c"))
		assert.Equal(t, statusCopied, selectedEntry(t, m).status)
		assert.NotNil(t, cmd)
	})

	t.Run("statusClearMsg removes the status indicator", func(t *testing.T) {
		m := newSecretViewModel(t)
		m, _ = update(t, m, keyPress("c"))
		m, _ = update(t, m, statusClearMsg{index: 0})
		assert.Empty(t, selectedEntry(t, m).status)
	})

	t.Run("statusClearMsg for an unknown index is a no-op", func(t *testing.T) {
		m := newSecretViewModel(t)
		m, _ = update(t, m, keyPress("c"))
		m, cmd := update(t, m, statusClearMsg{index: 99})
		assert.Nil(t, cmd)
		assert.Equal(t, statusCopied, selectedEntry(t, m).status)
	})

	t.Run("a generated totp token is copied", func(t *testing.T) {
		m := newSecretViewModel(t)
		m, cmd := update(t, m, totpMsg{index: 0, token: "123456"})
		assert.Equal(t, statusTOTPCopied, selectedEntry(t, m).status)
		require.NotNil(t, cmd)
	})

	t.Run("a failed totp shows a short error on the entry", func(t *testing.T) {
		m := newSecretViewModel(t)
		m, _ = update(t, m, totpMsg{index: 0, err: fmt.Errorf("%w: oathtool: base32 decoding failed", errInvalidSecret)})
		e := selectedEntry(t, m)
		assert.Equal(t, "not a base32 secret", e.status)
		assert.True(t, e.statusErr)
		assert.Contains(t, m.View().Content, "not a base32 secret")
		assert.NotContains(t, m.View().Content, "decoding failed")
	})

	t.Run("the error status is rendered in red", func(t *testing.T) {
		m := newSecretViewModel(t)
		m, _ = update(t, m, totpMsg{index: 0, err: errOathtoolMissing})
		view := m.View().Content
		red := statusErrorStyle.Render("(oathtool not installed)")
		assert.Contains(t, view, red)
		assert.NotEqual(t, "(oathtool not installed)", red, "status style must emit escape codes")
	})

	t.Run("a cleared status drops the error styling", func(t *testing.T) {
		m := newSecretViewModel(t)
		m, _ = update(t, m, totpMsg{index: 0, err: errOathtoolMissing})
		m, _ = update(t, m, statusClearMsg{index: 0})
		e := selectedEntry(t, m)
		assert.Empty(t, e.status)
		assert.False(t, e.statusErr)
	})

	t.Run("t generates a totp for the selected entry", func(t *testing.T) {
		m := newSecretViewModel(t)
		m, cmd := update(t, m, keyPress("t"))
		require.NotNil(t, cmd)
		msg, ok := cmd().(totpMsg)
		require.True(t, ok)
		assert.Equal(t, 0, msg.index)
	})

	t.Run("p prints the secret and quits", func(t *testing.T) {
		m := newSecretViewModel(t)
		m, cmd := update(t, m, keyPress("p"))
		require.NotNil(t, cmd)
		m, _ = update(t, m, cmd())
		assert.Equal(t, stateDone, m.state)
		out := m.output()
		assert.Contains(t, out, "Path: secret/db")
		assert.Contains(t, out, "password: hunter2")
		assert.Contains(t, out, "user: myuser")
	})

	t.Run("esc goes back to the tree", func(t *testing.T) {
		m := newSecretViewModel(t)
		m, cmd := update(t, m, keyPress("esc"))
		assert.Equal(t, stateLoading, m.state)
		assert.Equal(t, "secret", m.path.String())
		assert.NotNil(t, cmd)
	})

	t.Run("all entries fit on one page", func(t *testing.T) {
		m := newSecretViewModel(t)
		view := m.View().Content
		assert.Contains(t, view, "password:")
		assert.Contains(t, view, "user:")
	})

	t.Run("view shows title, metadata and masked entries", func(t *testing.T) {
		m := newSecretViewModel(t)
		view := m.View().Content
		assert.Contains(t, view, "secret/db")
		assert.Contains(t, view, "version 3 · created 2026-08-20T10:30:00Z")
		assert.Contains(t, view, "password: ********")
		assert.NotContains(t, view, "hunter2")
	})
}
