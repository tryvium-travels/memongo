package mongobin_test

import (
	"errors"
	"io/fs"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tryvium-travels/memongo/memongolog"
	"github.com/tryvium-travels/memongo/mongobin"
	"github.com/tryvium-travels/memongo/mongobin/mockAfero"
)

func TestGetOrDownload(t *testing.T) {
	mongobin.Afs = afero.Afero{Fs: afero.NewMemMapFs()}

	spec := mongobin.DownloadSpec{
		Version:        "4.0.5",
		Platform:       "osx",
		SSLBuildNeeded: true,
		Arch:           "x86_64",
	}

	cacheDir, err := mongobin.Afs.TempDir("", "")
	require.NoError(t, err)

	// First call should download the file
	path, err := mongobin.GetOrDownloadMongod(spec.GetDownloadURL(), cacheDir, memongolog.New(nil, memongolog.LogLevelDebug))
	require.NoError(t, err)

	assert.Equal(t, cacheDir+"/mongodb-osx-ssl-x86_64-4_0_5_tgz_d50ef2155b/mongod", path)

	stat, err := mongobin.Afs.Stat(path)
	require.NoError(t, err)

	assert.True(t, stat.Size() > 50000000)
	assert.True(t, stat.Mode()&0100 != 0)

	// Second call should used the cached file
	path2, err := mongobin.GetOrDownloadMongod(spec.GetDownloadURL(), cacheDir, memongolog.New(nil, memongolog.LogLevelDebug))
	require.NoError(t, err)

	assert.Equal(t, path, path2)

	stat2, err := mongobin.Afs.Stat(path2)
	require.NoError(t, err)

	assert.Equal(t, stat.ModTime(), stat2.ModTime())
}

func TestGetOrDownloadDifferentFilesystems(t *testing.T) {

	FS := afero.NewMemMapFs() // afero.NewOsFs()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mockAfero.NewMockFs(ctrl)

	m.EXPECT().Rename(gomock.Any(), gomock.Any()).Return(&os.LinkError{"rename", "oldname", "newname", errors.New("rename error")}).Times(2)

	// General mock faking :)
	m.EXPECT().Mkdir(gomock.Any(), gomock.Any()).DoAndReturn(func(dir string, perm fs.FileMode) error { return FS.Mkdir(dir, perm) }).AnyTimes()
	m.EXPECT().Stat(gomock.Any()).DoAndReturn(func(name string) (os.FileInfo, error) { return FS.Stat(name) }).AnyTimes()

	m.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(name string, flag int, perm os.FileMode) (afero.File, error) {
		return FS.OpenFile(name, flag, perm)
	}).AnyTimes()

	m.EXPECT().Remove(gomock.Any()).DoAndReturn(func(name string) error { return FS.Remove(name) }).AnyTimes()
	m.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).DoAndReturn(func(dir string, perm fs.FileMode) error { return FS.MkdirAll(dir, perm) }).AnyTimes()
	m.EXPECT().Chmod(gomock.Any(), gomock.Any()).DoAndReturn(func(name string, mode os.FileMode) error { return FS.Chmod(name, mode) }).AnyTimes()
	m.EXPECT().Create(gomock.Any()).DoAndReturn(func(name string) (afero.File, error) { return FS.Create(name) }).AnyTimes()
	m.EXPECT().Open(gomock.Any()).DoAndReturn(func(name string) (afero.File, error) { return FS.Open(name) }).AnyTimes()

	mongobin.Afs = afero.Afero{Fs: m}

	spec := mongobin.DownloadSpec{
		Version:        "4.0.5",
		Platform:       "osx",
		SSLBuildNeeded: true,
		Arch:           "x86_64",
	}

	// Initialize the cache in a different filesystem
	cacheFs := afero.Afero{Fs: m}
	cacheDir, err := cacheFs.TempDir("", "")
	require.NoError(t, err)

	// First call should download the file
	path, err := mongobin.GetOrDownloadMongod(spec.GetDownloadURL(), cacheDir, memongolog.New(nil, memongolog.LogLevelDebug))
	require.NoError(t, err)

	assert.Equal(t, cacheDir+"/mongodb-osx-ssl-x86_64-4_0_5_tgz_d50ef2155b/mongod", path)

	stat, err := mongobin.Afs.Stat(path)
	require.NoError(t, err)

	assert.True(t, stat.Size() > 50000000)
	// TODO Restore this assert
	// assert.True(t, stat.Mode()&0100 != 0)

	// Second call should used the cached file
	path2, err := mongobin.GetOrDownloadMongod(spec.GetDownloadURL(), cacheDir, memongolog.New(nil, memongolog.LogLevelDebug))
	require.NoError(t, err)

	assert.Equal(t, path, path2)

	stat2, err := mongobin.Afs.Stat(path2)
	require.NoError(t, err)

	assert.Equal(t, stat.ModTime(), stat2.ModTime())
}
