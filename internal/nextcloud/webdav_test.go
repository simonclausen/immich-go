package nextcloud

import (
	"io"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWebDAVFSRejectsEmptyUID(t *testing.T) {
	t.Parallel()

	client := &Client{}
	_, err := NewWebDAVFS(client, " ")
	require.Error(t, err)
	assert.ErrorContains(t, err, "user ID")
}

func TestWebDAVFSReadDirAndOpen(t *testing.T) {
	t.Parallel()

	rootInfo := fakeFileInfo{name: "alice", dir: true, modTime: time.Unix(1700000000, 0)}
	photosInfo := fakeFileInfo{name: "Photos", dir: true, modTime: time.Unix(1700000001, 0)}
	imageInfo := fakeFileInfo{name: "IMG_0001.JPG", size: 5, modTime: time.Unix(1700000002, 0)}

	dav := &fakeDAVClient{
		stats: map[string]fakeFileInfo{
			"/files/alice":                     rootInfo,
			"/files/alice/Photos":              photosInfo,
			"/files/alice/Photos/IMG_0001.JPG": imageInfo,
		},
		dirs: map[string][]fakeFileInfo{
			"/files/alice":        {photosInfo},
			"/files/alice/Photos": {imageInfo},
		},
		files: map[string]string{
			"/files/alice/Photos/IMG_0001.JPG": "hello",
		},
	}

	fsys := newWebDAVFS(dav, "/files/alice", "nextcloud:alice")

	entries, err := fs.ReadDir(fsys, "Photos")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "IMG_0001.JPG", entries[0].Name())

	f, err := fsys.Open("Photos/IMG_0001.JPG")
	require.NoError(t, err)
	defer f.Close()

	b, err := io.ReadAll(f)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(b))
}

func TestWebDAVFSRejectsPathEscape(t *testing.T) {
	t.Parallel()

	fsys := newWebDAVFS(&fakeDAVClient{}, "/files/alice", "nextcloud:alice")
	_, err := fsys.Stat("../secrets.txt")
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid argument")
}

type fakeDAVClient struct {
	stats map[string]fakeFileInfo
	dirs  map[string][]fakeFileInfo
	files map[string]string
}

func (f *fakeDAVClient) ReadDir(path string) ([]os.FileInfo, error) {
	entries, ok := f.dirs[path]
	if !ok {
		return nil, fs.ErrNotExist
	}
	result := make([]os.FileInfo, 0, len(entries))
	for _, entry := range entries {
		entry := entry
		result = append(result, entry)
	}
	return result, nil
}

func (f *fakeDAVClient) ReadStream(path string) (io.ReadCloser, error) {
	b, ok := f.files[path]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return io.NopCloser(strings.NewReader(b)), nil
}

func (f *fakeDAVClient) Stat(path string) (os.FileInfo, error) {
	info, ok := f.stats[path]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return info, nil
}

type fakeFileInfo struct {
	name    string
	size    int64
	dir     bool
	modTime time.Time
}

func (f fakeFileInfo) Name() string { return f.name }
func (f fakeFileInfo) Size() int64  { return f.size }
func (f fakeFileInfo) Mode() fs.FileMode {
	if f.dir {
		return fs.ModeDir | 0o755
	}
	return 0o644
}
func (f fakeFileInfo) ModTime() time.Time { return f.modTime }
func (f fakeFileInfo) IsDir() bool        { return f.dir }
func (f fakeFileInfo) Sys() any           { return nil }
