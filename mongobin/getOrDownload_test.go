package mongobin_test

import (
	"testing"

	"github.com/ntaylor-barnett/memongo/memongolog"
	"github.com/ntaylor-barnett/memongo/mongobin"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	path, err := mongobin.GetOrDownloadMongod(spec.GetDownloadURL(), spec.GetShellDownloadURL(), cacheDir, memongolog.New(nil, memongolog.LogLevelDebug))
	require.NoError(t, err)

	assert.Equal(t, cacheDir+"/mongodb-osx-ssl-x86_64-4_0_5_tgz_d50ef2155b/bin/mongod", path.Mongod)
	assert.Equal(t, cacheDir+"/mongodb-osx-ssl-x86_64-4_0_5_tgz_d50ef2155b/bin/mongosh", path.Mongosh)

	stat, err := mongobin.Afs.Stat(path.Mongod)
	require.NoError(t, err)
	stat, err = mongobin.Afs.Stat(path.Mongosh)
	require.NoError(t, err)

	assert.True(t, stat.Size() > 50000000)
	assert.True(t, stat.Mode()&0100 != 0)

	// Second call should used the cached file
	path2, err := mongobin.GetOrDownloadMongod(spec.GetDownloadURL(), spec.GetShellDownloadURL(), cacheDir, memongolog.New(nil, memongolog.LogLevelDebug))
	require.NoError(t, err)

	assert.Equal(t, path, path2)

	stat2, err := mongobin.Afs.Stat(path2.Mongod)
	require.NoError(t, err)
	stat2, err = mongobin.Afs.Stat(path2.Mongosh)
	require.NoError(t, err)
	assert.Equal(t, stat.ModTime(), stat2.ModTime())
}

func TestGetOrDownloadDifferentFilesystems(t *testing.T) {
	mongobin.Afs = afero.Afero{Fs: afero.NewMemMapFs()}

	spec := mongobin.DownloadSpec{
		Version:        "4.0.5",
		Platform:       "osx",
		SSLBuildNeeded: true,
		Arch:           "x86_64",
	}

	// Initialize the cache in a different filesystem
	cacheFs := afero.Afero{Fs: afero.NewMemMapFs()}
	cacheDir, err := cacheFs.TempDir("", "")
	require.NoError(t, err)

	// First call should download the file
	path, err := mongobin.GetOrDownloadMongod(spec.GetDownloadURL(), spec.GetShellDownloadURL(), cacheDir, memongolog.New(nil, memongolog.LogLevelDebug))
	require.NoError(t, err)

	assert.Equal(t, cacheDir+"/bin/mongodb-osx-ssl-x86_64-4_0_5_tgz_d50ef2155b/mongod", path)

	stat, err := mongobin.Afs.Stat(path.Mongod)
	require.NoError(t, err)

	assert.True(t, stat.Size() > 50000000)
	assert.True(t, stat.Mode()&0100 != 0)

	// Second call should used the cached file
	path2, err := mongobin.GetOrDownloadMongod(spec.GetDownloadURL(), spec.GetShellDownloadURL(), cacheDir, memongolog.New(nil, memongolog.LogLevelDebug))
	require.NoError(t, err)

	assert.Equal(t, path, path2)

	stat2, err := mongobin.Afs.Stat(path2.Mongod)
	require.NoError(t, err)

	assert.Equal(t, stat.ModTime(), stat2.ModTime())
}
