package main

import (
	"fmt"
	"strings"
)

type vaultPath []string

func (v vaultPath) Engine() string {
	return v[0]
}

func (v vaultPath) MetadataPath() string {
	var s strings.Builder
	for i, p := range v {
		s.WriteString(fmt.Sprintf("%s/", p))
		if i == 0 {
			s.WriteString("metadata/")
		}
	}
	return s.String()
}

func (v vaultPath) String() string {
	return strings.Join(v, "/")
}

func (v vaultPath) Path() string {
	return strings.Join(v[1:], "/")
}

func (v *vaultPath) Add(path ...string) {
	for _, p := range path {
		*v = append(*v, strings.TrimSuffix(p, "/"))
	}
}

func (v *vaultPath) Back() *vaultPath {
	if len(*v) > 1 {
		*v = (*v)[:len(*v)-1]
	}
	return v
}
