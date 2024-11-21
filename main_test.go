package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVaultPath(t *testing.T) {
	type testCase struct {
		vp             vaultPath
		metadataResult string
		engineResult   string
		stringResult   string
		pathResult     string
	}
	for _, tc := range []testCase{
		{
			vp:             vaultPath{"engine", "path", "to", "secret"},
			metadataResult: "engine/metadata/path/to/secret/",
			engineResult:   "engine",
			stringResult:   "engine/path/to/secret",
			pathResult:     "path/to/secret",
		},
	} {
		assert.Equal(t, tc.metadataResult, tc.vp.MetadataPath())
		assert.Equal(t, tc.engineResult, tc.vp.Engine())
		assert.Equal(t, tc.stringResult, tc.vp.String())
		assert.Equal(t, tc.pathResult, tc.vp.Path())
	}
}
