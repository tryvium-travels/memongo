//go:build !windows

package mongobin_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/100mslive/memongo/v2/memongolog"
	"github.com/100mslive/memongo/v2/mongobin"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// fakeMongodTarball is a tarball with bin/mongod: a shell script that exits
// 0, padded so writing it takes long enough for downloaders to overlap.
func fakeMongodTarball(t *testing.T) []byte {
	t.Helper()

	body := "#!/bin/sh\nexit 0\n" + strings.Repeat("#"+strings.Repeat("x", 1023)+"\n", 4*1024)

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "mongodb-fake/bin/mongod", Mode: 0755, Size: int64(len(body))}))
	_, err := tw.Write([]byte(body))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())

	return buf.Bytes()
}

// Parallel `go test` packages share one cache, and each runs mongod as soon
// as it finds it there. A downloader must never leave a file at that path
// that cannot be executed yet.
func TestGetOrDownloadParallelDownloadersAlwaysSeeExecutable(t *testing.T) {
	mongobin.Afs = afero.Afero{Fs: afero.NewOsFs()}

	tarball := fakeMongodTarball(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(tarball)
	}))
	defer srv.Close()
	url := srv.URL + "/mongodb-fake.tgz"

	const rounds, downloaders = 20, 8
	logger := memongolog.New(nil, memongolog.LogLevelSilent)

	for round := 0; round < rounds; round++ {
		cacheDir := t.TempDir()

		var wg sync.WaitGroup
		errs := make(chan error, downloaders)
		for i := 0; i < downloaders; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				mongodPath, err := mongobin.GetOrDownloadMongod(url, cacheDir, logger)
				if err != nil {
					errs <- err
					return
				}
				if err := exec.Command(mongodPath).Run(); err != nil {
					errs <- err
				}
			}()
		}
		wg.Wait()
		close(errs)

		for err := range errs {
			require.NoError(t, err, "round %d", round)
		}
	}
}
