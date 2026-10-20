package updatepolicy

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestManifestPolicy(t *testing.T) {
	valid := Manifest{SchemaVersion: 1, Version: "2026.10.20", Revision: strings.Repeat("a", 40), Images: map[string]string{"amd64": ImageRepository + "@sha256:" + strings.Repeat("b", 64)}}
	data, _ := json.Marshal(valid)
	_, err := Parse(data, "v2026.10.20")
	require.NoError(t, err)
	for _, mutate := range []func(*Manifest){func(m *Manifest) { m.SchemaVersion = 2 }, func(m *Manifest) { m.Version = "2026.10.21" }, func(m *Manifest) { m.Revision = "main" }, func(m *Manifest) { m.Images = map[string]string{"amd64": ImageRepository + ":latest"} }, func(m *Manifest) {
		m.Images = map[string]string{"amd64": "evil.example/app@sha256:" + strings.Repeat("b", 64)}
	}} {
		m := valid
		mutate(&m)
		data, _ := json.Marshal(m)
		_, err := Parse(data, valid.Version)
		require.Error(t, err)
	}
	_, err = AssetURL("../../other")
	require.Error(t, err)
	_, err = Parse([]byte(strings.Repeat(" ", MaxManifestSize+1)), valid.Version)
	require.Error(t, err)
}

func TestForkReleaseVersionAndRepository(t *testing.T) {
	for _, version := range []string{"2026.10.20", "v2026.1.2", "2028.2.29", "2026.01.02"} {
		require.True(t, ValidVersion(version), version)
		url, err := AssetURL(version)
		require.NoError(t, err)
		require.Contains(t, url, "https://github.com/MIEnchating/sub2api/releases/download/v")
	}
	for _, version := range []string{"0.2.17-custom.18", "latest", "2026.2.29", "2026.13.1", "2026.10.20-rc.1", "2026.10.20/other"} {
		require.False(t, ValidVersion(version), version)
	}
}
