package cmd

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newSecretViewModelWithMeta returns a detail view of a secret whose "user"
// field is mirrored in the custom metadata.
func newSecretViewModelWithMeta(t *testing.T) model {
	t.Helper()
	m := newModel(nil, vaultPath{"secret", "db"}, nil)
	m.width = 80
	m.height = 40
	m, _ = update(t, m, secretsMsg{
		secrets: []vaultSecret{
			{key: "password", value: "hunter2"},
			{key: "user", value: "myuser"},
		},
		metaLines:  []string{"version 3", "user=myuser"},
		customMeta: map[string]any{"user": "myuser"},
	})
	return m
}

func typeText(t *testing.T, m model, s string) model {
	t.Helper()
	for _, r := range s {
		m, _ = update(t, m, keyPress(string(r)))
	}
	return m
}

func TestSecretEdit(t *testing.T) {
	t.Run("e opens an input prefilled with the value", func(t *testing.T) {
		m := newSecretViewModel(t)
		m, _ = update(t, m, keyPress("e"))
		assert.Equal(t, editInput, m.editor.mode)
		assert.Equal(t, "hunter2", m.editor.input.Value())
		view := m.View().Content
		assert.Contains(t, view, "hunter2")
		assert.Contains(t, view, "password")
	})

	t.Run("esc cancels the input", func(t *testing.T) {
		m := newSecretViewModel(t)
		m, _ = update(t, m, keyPress("e"))
		m, _ = update(t, m, keyPress("esc"))
		assert.Equal(t, editNone, m.editor.mode)
		assert.Equal(t, stateSecret, m.state, "esc while editing must not leave the secret")
	})

	t.Run("list keys are typed into the input while editing", func(t *testing.T) {
		m := newSecretViewModel(t)
		m, _ = update(t, m, keyPress("e"))
		m, _ = update(t, m, keyPress("s"))
		assert.False(t, selectedEntry(t, m).revealed)
		assert.Equal(t, "hunter2s", m.editor.input.Value())
	})

	t.Run("enter with an unchanged value closes the editor without a diff", func(t *testing.T) {
		m := newSecretViewModel(t)
		m, _ = update(t, m, keyPress("e"))
		m, cmd := update(t, m, keyPress("enter"))
		assert.Equal(t, editNone, m.editor.mode)
		assert.Nil(t, cmd)
	})

	t.Run("enter shows the diff and asks for confirmation", func(t *testing.T) {
		m := newSecretViewModel(t)
		m, _ = update(t, m, keyPress("e"))
		m = typeText(t, m, "!")
		m, _ = update(t, m, keyPress("enter"))
		assert.Equal(t, editConfirm, m.editor.mode)
		view := m.View().Content
		assert.Contains(t, view, "- password: hunter2")
		assert.Contains(t, view, "+ password: hunter2!")
		assert.NotContains(t, view, "metadata")
	})

	t.Run("the diff includes the metadata when the field is mirrored", func(t *testing.T) {
		m := newSecretViewModelWithMeta(t)
		m, _ = update(t, m, keyPress("j"))
		require.Equal(t, "user", selectedEntry(t, m).key)
		m, _ = update(t, m, keyPress("e"))
		m = typeText(t, m, "2")
		m, _ = update(t, m, keyPress("enter"))
		assert.True(t, m.editor.updateMeta)
		view := m.View().Content
		assert.Contains(t, view, "- metadata user=myuser")
		assert.Contains(t, view, "+ metadata user=myuser2")
	})

	t.Run("n cancels the confirmation", func(t *testing.T) {
		m := newSecretViewModel(t)
		m, _ = update(t, m, keyPress("e"))
		m = typeText(t, m, "!")
		m, _ = update(t, m, keyPress("enter"))
		m, cmd := update(t, m, keyPress("n"))
		assert.Equal(t, editNone, m.editor.mode)
		assert.Nil(t, cmd)
		assert.Equal(t, "hunter2", selectedEntry(t, m).value)
	})

	t.Run("y confirms and returns the patch command", func(t *testing.T) {
		m := newSecretViewModel(t)
		m, _ = update(t, m, keyPress("e"))
		m = typeText(t, m, "!")
		m, _ = update(t, m, keyPress("enter"))
		m, cmd := update(t, m, keyPress("y"))
		assert.Equal(t, editNone, m.editor.mode)
		assert.NotNil(t, cmd)
		assert.Equal(t, statusSaving, selectedEntry(t, m).status)
	})

	t.Run("a successful edit refreshes the secret and keeps the selection", func(t *testing.T) {
		m := newSecretViewModelWithMeta(t)
		m, _ = update(t, m, keyPress("j"))
		m, _ = update(t, m, editDoneMsg{index: 1, secrets: secretsMsg{
			secrets: []vaultSecret{
				{key: "password", value: "hunter2"},
				{key: "user", value: "myuser2"},
			},
			metaLines:  []string{"version 4", "user=myuser2"},
			customMeta: map[string]any{"user": "myuser2"},
		}})
		e := selectedEntry(t, m)
		assert.Equal(t, "user", e.key)
		assert.Equal(t, "myuser2", e.value)
		assert.Equal(t, statusSaved, e.status)
		assert.Equal(t, []string{"version 4", "user=myuser2"}, m.metaLines)
	})

	t.Run("a failed edit shows a red status", func(t *testing.T) {
		m := newSecretViewModel(t)
		m, _ = update(t, m, editDoneMsg{index: 0, err: errors.New("Error making API request.\n\nCode: 403. Errors:\n\n* permission denied")})
		e := selectedEntry(t, m)
		assert.Equal(t, "save failed: permission denied", e.status)
		assert.True(t, e.statusErr)
		assert.Equal(t, "hunter2", e.value)
	})
}
