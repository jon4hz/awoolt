package put

import (
	"context"
	"errors"
	"testing"

	"github.com/openbao/openbao/api/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type putCall struct {
	path string
	data map[string]any
}

type metadataCall struct {
	path  string
	input api.KVMetadataPutInput
}

type fakeKV struct {
	putCalls      []putCall
	metadataCalls []metadataCall
	putErr        error
	metadataErr   error
}

func (f *fakeKV) Put(_ context.Context, path string, data map[string]any, _ ...api.KVOption) (*api.KVSecret, error) {
	if f.putErr != nil {
		return nil, f.putErr
	}
	f.putCalls = append(f.putCalls, putCall{path: path, data: data})
	return &api.KVSecret{}, nil
}

func (f *fakeKV) PutMetadata(_ context.Context, path string, input api.KVMetadataPutInput) error {
	if f.metadataErr != nil {
		return f.metadataErr
	}
	f.metadataCalls = append(f.metadataCalls, metadataCall{path: path, input: input})
	return nil
}

func TestValidate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		secret  Secret
		wantErr string
	}{
		{
			name:   "valid secret",
			secret: Secret{Path: "app/db", Data: map[string]string{"username": "admin"}},
		},
		{
			name: "valid secret with metadata fields",
			secret: Secret{
				Path:           "app/db",
				Data:           map[string]string{"username": "admin", "password": "hunter2"},
				MetadataFields: []string{"username"},
			},
		},
		{
			name:    "empty path",
			secret:  Secret{Data: map[string]string{"username": "admin"}},
			wantErr: "path must not be empty",
		},
		{
			name:    "empty data",
			secret:  Secret{Path: "app/db"},
			wantErr: "data must not be empty",
		},
		{
			name: "metadata field not in data",
			secret: Secret{
				Path:           "app/db",
				Data:           map[string]string{"username": "admin"},
				MetadataFields: []string{"password"},
			},
			wantErr: `metadata field "password" not found in data`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.secret.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestWriteAll_WritesData(t *testing.T) {
	t.Parallel()
	kv := &fakeKV{}
	secrets := []Secret{
		{Path: "app/db", Data: map[string]string{"username": "admin", "password": "hunter2"}},
		{Path: "app/cache", Data: map[string]string{"username": "redis"}},
	}

	err := WriteAll(context.Background(), kv, secrets)

	require.NoError(t, err)
	require.Len(t, kv.putCalls, 2)
	assert.Equal(t, "app/db", kv.putCalls[0].path)
	assert.Equal(t, map[string]any{"username": "admin", "password": "hunter2"}, kv.putCalls[0].data)
	assert.Equal(t, "app/cache", kv.putCalls[1].path)
	assert.Equal(t, map[string]any{"username": "redis"}, kv.putCalls[1].data)
}

func TestWriteAll_WritesCustomMetadataForSelectedFields(t *testing.T) {
	t.Parallel()
	kv := &fakeKV{}
	secrets := []Secret{{
		Path:           "app/db",
		Data:           map[string]string{"username": "admin", "password": "hunter2"},
		MetadataFields: []string{"username"},
	}}

	err := WriteAll(context.Background(), kv, secrets)

	require.NoError(t, err)
	require.Len(t, kv.metadataCalls, 1)
	assert.Equal(t, "app/db", kv.metadataCalls[0].path)
	assert.Equal(t, map[string]any{"username": "admin"}, kv.metadataCalls[0].input.CustomMetadata)
}

func TestWriteAll_NoMetadataCallWithoutMetadataFields(t *testing.T) {
	t.Parallel()
	kv := &fakeKV{}
	secrets := []Secret{{Path: "app/db", Data: map[string]string{"username": "admin"}}}

	err := WriteAll(context.Background(), kv, secrets)

	require.NoError(t, err)
	assert.Empty(t, kv.metadataCalls)
}

func TestWriteAll_ValidatesBeforeWriting(t *testing.T) {
	t.Parallel()
	kv := &fakeKV{}
	secrets := []Secret{
		{Path: "app/db", Data: map[string]string{"username": "admin"}},
		{Path: "", Data: map[string]string{"username": "admin"}},
	}

	err := WriteAll(context.Background(), kv, secrets)

	assert.ErrorContains(t, err, "path must not be empty")
	assert.Empty(t, kv.putCalls, "no secret must be written if any secret is invalid")
}

func TestWriteAll_PutErrorIncludesPath(t *testing.T) {
	t.Parallel()
	kv := &fakeKV{putErr: errors.New("permission denied")}
	secrets := []Secret{{Path: "app/db", Data: map[string]string{"username": "admin"}}}

	err := WriteAll(context.Background(), kv, secrets)

	assert.ErrorContains(t, err, "app/db")
	assert.ErrorContains(t, err, "permission denied")
	assert.Empty(t, kv.metadataCalls, "metadata must not be written if data write failed")
}

func TestWriteAll_MetadataErrorIncludesPath(t *testing.T) {
	t.Parallel()
	kv := &fakeKV{metadataErr: errors.New("permission denied")}
	secrets := []Secret{{
		Path:           "app/db",
		Data:           map[string]string{"username": "admin"},
		MetadataFields: []string{"username"},
	}}

	err := WriteAll(context.Background(), kv, secrets)

	assert.ErrorContains(t, err, "app/db")
	assert.ErrorContains(t, err, "permission denied")
}
