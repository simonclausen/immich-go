package nextcloud

import (
	"io"
	"io/fs"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreferredLocalFSOpenPrefersLocalFile(t *testing.T) {
	t.Parallel()

	local := fstest.MapFS{
		"Photos/IMG_0001.JPG": {Data: []byte("local")},
	}
	remote := fstest.MapFS{
		"Photos/IMG_0001.JPG": {Data: []byte("remote")},
	}

	fsys := NewPreferredLocalFS(local, remote)
	f, err := fsys.Open("Photos/IMG_0001.JPG")
	require.NoError(t, err)
	defer f.Close()

	b, err := io.ReadAll(f)
	require.NoError(t, err)
	assert.Equal(t, "local", string(b))
}

func TestPreferredLocalFSFallsBackToRemoteOpen(t *testing.T) {
	t.Parallel()

	remote := fstest.MapFS{
		"Photos/IMG_0001.JPG": {Data: []byte("remote")},
	}

	fsys := NewPreferredLocalFS(fstest.MapFS{}, remote)
	f, err := fsys.Open("Photos/IMG_0001.JPG")
	require.NoError(t, err)
	defer f.Close()

	b, err := io.ReadAll(f)
	require.NoError(t, err)
	assert.Equal(t, "remote", string(b))
}

func TestPreferredLocalFSDelegatesEnumerationToRemote(t *testing.T) {
	t.Parallel()

	remote := &testSearchFS{
		MapFS: fstest.MapFS{
			"Photos/Trips/IMG_0001.JPG": {Data: []byte("remote")},
		},
		entries: []SearchEntry{{Path: "Photos/Trips/IMG_0001.JPG", Info: testSearchInfo{name: "IMG_0001.JPG", size: 6}}},
	}

	fsys := NewPreferredLocalFS(fstest.MapFS{}, remote)
	dirEntries, err := fs.ReadDir(fsys, "Photos/Trips")
	require.NoError(t, err)
	require.Len(t, dirEntries, 1)
	assert.Equal(t, "IMG_0001.JPG", dirEntries[0].Name())

	searchFS, ok := fsys.(searchFilesFS)
	require.True(t, ok)
	entries, err := searchFS.SearchFiles("Photos")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, []string{"Photos"}, remote.scopes)

	info, err := fs.Stat(fsys, "Photos/Trips/IMG_0001.JPG")
	require.NoError(t, err)
	assert.Equal(t, int64(6), info.Size())
}

type testSearchFS struct {
	fstest.MapFS
	entries []SearchEntry
	scopes  []string
	err     error
}

func (t *testSearchFS) SearchFiles(scope string) ([]SearchEntry, error) {
	t.scopes = append(t.scopes, scope)
	if t.err != nil {
		return nil, t.err
	}
	return append([]SearchEntry(nil), t.entries...), nil
}

type testSearchInfo struct {
	name string
	size int64
}

func (t testSearchInfo) Name() string       { return t.name }
func (t testSearchInfo) Size() int64        { return t.size }
func (t testSearchInfo) Mode() fs.FileMode  { return 0o644 }
func (t testSearchInfo) ModTime() time.Time { return time.Unix(1700000000, 0) }
func (t testSearchInfo) IsDir() bool        { return false }
func (t testSearchInfo) Sys() any           { return nil }
