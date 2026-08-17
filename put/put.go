// Package put writes secrets to a vault/openbao KV v2 engine, optionally
// mirroring selected fields into the secret's custom metadata.
package put

import (
	"context"
	"errors"
	"fmt"

	"github.com/openbao/openbao/api/v2"
)

// KV is the subset of the openbao KV v2 client used to write secrets.
type KV interface {
	Put(ctx context.Context, path string, data map[string]any, opts ...api.KVOption) (*api.KVSecret, error)
	PutMetadata(ctx context.Context, path string, input api.KVMetadataPutInput) error
}

// Secret is a single KV v2 secret to write.
type Secret struct {
	// Path is the secret path relative to the engine mount.
	Path string
	// Data holds the key/value pairs stored in the secret.
	Data map[string]string
	// MetadataFields lists the keys of Data that are additionally written as
	// custom metadata. Note that custom metadata is not encrypted.
	MetadataFields []string
}

// Validate reports whether the secret can be written.
func (s Secret) Validate() error {
	if s.Path == "" {
		return errors.New("path must not be empty")
	}
	if len(s.Data) == 0 {
		return errors.New("data must not be empty")
	}
	for _, field := range s.MetadataFields {
		if _, ok := s.Data[field]; !ok {
			return fmt.Errorf("metadata field %q not found in data", field)
		}
	}
	return nil
}

// WriteAll validates all secrets and then writes them to the KV engine.
func WriteAll(ctx context.Context, kv KV, secrets []Secret) error {
	for _, secret := range secrets {
		if err := secret.Validate(); err != nil {
			return fmt.Errorf("invalid secret %q: %w", secret.Path, err)
		}
	}
	for _, secret := range secrets {
		if err := write(ctx, kv, secret); err != nil {
			return fmt.Errorf("failed to write secret %q: %w", secret.Path, err)
		}
	}
	return nil
}

func write(ctx context.Context, kv KV, secret Secret) error {
	data := make(map[string]any, len(secret.Data))
	for k, v := range secret.Data {
		data[k] = v
	}
	if _, err := kv.Put(ctx, secret.Path, data); err != nil {
		return err
	}

	if len(secret.MetadataFields) == 0 {
		return nil
	}
	metadata := make(map[string]any, len(secret.MetadataFields))
	for _, field := range secret.MetadataFields {
		metadata[field] = secret.Data[field]
	}
	return kv.PutMetadata(ctx, secret.Path, api.KVMetadataPutInput{CustomMetadata: metadata})
}
