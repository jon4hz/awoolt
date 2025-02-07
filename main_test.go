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
		backPathResult string
	}
	for _, tc := range []testCase{
		{
			vp:             vaultPath{"engine", "path", "to", "secret"},
			metadataResult: "engine/metadata/path/to/secret/",
			engineResult:   "engine",
			stringResult:   "engine/path/to/secret",
			pathResult:     "path/to/secret",
			backPathResult: "path/to",
		},
		{
			vp:             vaultPath{"engine"},
			metadataResult: "engine/metadata/",
			engineResult:   "engine",
			stringResult:   "engine",
			pathResult:     "",
			backPathResult: "",
		},
		{
			vp:             vaultPath{"engine", "path"},
			metadataResult: "engine/metadata/path/",
			engineResult:   "engine",
			stringResult:   "engine/path",
			pathResult:     "path",
			backPathResult: "",
		},
	} {
		assert.Equal(t, tc.metadataResult, tc.vp.MetadataPath())
		assert.Equal(t, tc.engineResult, tc.vp.Engine())
		assert.Equal(t, tc.stringResult, tc.vp.String())
		assert.Equal(t, tc.pathResult, tc.vp.Path())
		assert.Equal(t, tc.backPathResult, tc.vp.Back().Path())
	}
}
