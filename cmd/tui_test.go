package cmd

import (
	"testing"

	"charm.land/bubbles/v2/list"
	"github.com/stretchr/testify/assert"
)

func TestFormatCustomMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
		expected string
	}{
		{
			name:     "nil map",
			metadata: nil,
			expected: "",
		},
		{
			name:     "empty map",
			metadata: map[string]any{},
			expected: "",
		},
		{
			name:     "single pair",
			metadata: map[string]any{"owner": "jonah"},
			expected: "owner=jonah",
		},
		{
			name:     "multiple pairs sorted by key",
			metadata: map[string]any{"team": "infra", "owner": "jonah"},
			expected: "owner=jonah team=infra",
		},
		{
			name: "ignored fields are skipped",
			metadata: map[string]any{
				"owner":                        "jonah",
				"vault_orig_created_time":      "2020-01-01T00:00:00Z",
				"vault_orig_last_updated_time": "2020-01-02T00:00:00Z",
			},
			expected: "owner=jonah",
		},
		{
			name: "only ignored fields",
			metadata: map[string]any{
				"vault_orig_created_time":      "2020-01-01T00:00:00Z",
				"vault_orig_last_updated_time": "2020-01-02T00:00:00Z",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, formatCustomMetadata(tt.metadata))
		})
	}
}

func TestPathItem(t *testing.T) {
	t.Run("title is the name", func(t *testing.T) {
		item := pathItem{name: "mysecret", meta: "owner=jonah"}
		assert.Equal(t, "mysecret", item.Title())
	})

	t.Run("description is the metadata", func(t *testing.T) {
		item := pathItem{name: "mysecret", meta: "owner=jonah"}
		assert.Equal(t, "owner=jonah", item.Description())
	})

	t.Run("filter value ignores metadata", func(t *testing.T) {
		item := pathItem{name: "mysecret", meta: "owner=jonah"}
		assert.Equal(t, "mysecret", item.FilterValue())
	})
}

func TestLeafKeys(t *testing.T) {
	keys := []string{"folder/", "secret-a", "nested/", "secret-b"}
	assert.Equal(t, []string{"secret-a", "secret-b"}, leafKeys(keys))
}

func TestSizeList(t *testing.T) {
	newSizedModel := func(items int) model {
		m := newModel(nil, vaultPath{"secret"}, nil)
		m.width = 80
		m.height = 40
		li := make([]list.Item, items)
		for i := range li {
			li[i] = pathItem{name: "item"}
		}
		m.list.SetItems(li)
		return m
	}

	t.Run("one line per item without descriptions", func(t *testing.T) {
		m := newSizedModel(3)
		m.sizeList()
		assert.Equal(t, 3+listChromeHeight, m.list.Height())
	})

	t.Run("two lines per item with descriptions", func(t *testing.T) {
		m := newSizedModel(3)
		m.showDesc = true
		m.sizeList()
		assert.Equal(t, 3*2+listChromeHeight, m.list.Height())
	})

	t.Run("capped at MAXHEIGHT", func(t *testing.T) {
		m := newSizedModel(30)
		m.showDesc = true
		m.sizeList()
		assert.Equal(t, MAXHEIGHT, m.list.Height())
	})
}
