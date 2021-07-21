package mongobin_test

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tryvium-travels/memongo/memongolog"
	"github.com/tryvium-travels/memongo/mongobin"
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
