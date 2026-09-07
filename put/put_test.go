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

	patchCalls         []patchCall
	metadataPatchCalls []metadataPatchCall
	patchErr           error
	metadataPatchErr   error
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

type patchCall struct {
	path string
	data map[string]any
}

type metadataPatchCall struct {
	path  string
	input api.KVMetadataPatchInput
}

func (f *fakeKV) Patch(_ context.Context, path string, data map[string]any, _ ...api.KVOption) (*api.KVSecret, error) {
	if f.patchErr != nil {
		return nil, f.patchErr
	}
	f.patchCalls = append(f.patchCalls, patchCall{path: path, data: data})
	return &api.KVSecret{}, nil
}

func (f *fakeKV) PatchMetadata(_ context.Context, path string, input api.KVMetadataPatchInput) error {
	if f.metadataPatchErr != nil {
		return f.metadataPatchErr
	}
	f.metadataPatchCalls = append(f.metadataPatchCalls, metadataPatchCall{path: path, input: input})
	return nil
}

func TestPatchField_PatchesOnlyTheField(t *testing.T) {
	t.Parallel()
	kv := &fakeKV{}

	err := PatchField(context.Background(), kv, "app/db", "password", "new", false)

	require.NoError(t, err)
	require.Len(t, kv.patchCalls, 1)
	assert.Equal(t, "app/db", kv.patchCalls[0].path)
	assert.Equal(t, map[string]any{"password": "new"}, kv.patchCalls[0].data)
	assert.Empty(t, kv.metadataPatchCalls)
}

func TestPatchField_UpdatesMetadataWhenRequested(t *testing.T) {
	t.Parallel()
	kv := &fakeKV{}

	err := PatchField(context.Background(), kv, "app/db", "username", "admin2", true)

	require.NoError(t, err)
	require.Len(t, kv.patchCalls, 1)
	require.Len(t, kv.metadataPatchCalls, 1)
	assert.Equal(t, "app/db", kv.metadataPatchCalls[0].path)
	assert.Equal(t, map[string]any{"username": "admin2"}, kv.metadataPatchCalls[0].input.CustomMetadata)
}

func TestPatchField_Validates(t *testing.T) {
	t.Parallel()
	kv := &fakeKV{}

	assert.ErrorContains(t, PatchField(context.Background(), kv, "", "k", "v", false), "path must not be empty")
	assert.ErrorContains(t, PatchField(context.Background(), kv, "app/db", "", "v", false), "key must not be empty")
	assert.Empty(t, kv.patchCalls)
}

func TestPatchField_PatchErrorSkipsMetadata(t *testing.T) {
	t.Parallel()
	kv := &fakeKV{patchErr: errors.New("permission denied")}

	err := PatchField(context.Background(), kv, "app/db", "username", "x", true)

	assert.ErrorContains(t, err, "app/db")
	assert.ErrorContains(t, err, "permission denied")
	assert.Empty(t, kv.metadataPatchCalls)
}

func TestPatchField_MetadataErrorIncludesPath(t *testing.T) {
	t.Parallel()
	kv := &fakeKV{metadataPatchErr: errors.New("permission denied")}

	err := PatchField(context.Background(), kv, "app/db", "username", "x", true)

	assert.ErrorContains(t, err, "app/db")
	assert.ErrorContains(t, err, "metadata")
}
