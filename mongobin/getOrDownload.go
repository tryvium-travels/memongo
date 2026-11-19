package mongobin

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/100mslive/memongo/v2/memongolog"
	"github.com/spf13/afero"
)

var Afs afero.Afero

func init() {
	Afs = afero.Afero{
		Fs: afero.NewOsFs(),
	}
}

// GetOrDownloadMongod returns the path to the mongod binary from the tarball
// at the given URL. If the URL has not yet been downloaded, it's downloaded
// and saved the the cache. If it has been downloaded, the existing mongod
// path is returned.
func GetOrDownloadMongod(urlStr string, cachePath string, logger *memongolog.Logger) (string, error) {
	dirname, dirErr := directoryNameForURL(urlStr)
	if dirErr != nil {
		return "", dirErr
	}

	dirPath := path.Join(cachePath, dirname)
	mongodPath := path.Join(dirPath, "mongod")

	// Check the cache
	existsInCache, existsErr := Afs.Exists(mongodPath)
	if existsErr != nil {
		return "", fmt.Errorf("error while checking for mongod in cache: %s", existsErr)
	}
	if existsInCache {
		logger.Debugf("mongod from %s exists in cache at %s", urlStr, mongodPath)
		return mongodPath, nil
	}

	logger.Infof("mongod from %s does not exist in cache, downloading to %s", urlStr, mongodPath)
	downloadStartTime := time.Now()

	// Download the file
	// nolint:gosec
	resp, httpGetErr := http.Get(urlStr)
	if httpGetErr != nil {
		return "", fmt.Errorf("error getting tarball from %s: %s", urlStr, httpGetErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP request failed with status code %d", resp.StatusCode)
	}

	tgzTempFile, tmpFileErr := Afs.TempFile("", "")
	if tmpFileErr != nil {
		return "", fmt.Errorf("error creating temp file for tarball: %s", tmpFileErr)
	}
	defer func() {
		_ = tgzTempFile.Close()
		_ = Afs.Remove(tgzTempFile.Name())
	}()

	_, copyErr := io.Copy(tgzTempFile, resp.Body)
	if copyErr != nil {
		return "", fmt.Errorf("error downloading tarball from %s: %s", urlStr, copyErr)
	}

	_, seekErr := tgzTempFile.Seek(0, 0)
	if seekErr != nil {
		return "", fmt.Errorf("error seeking back to start of file: %s", seekErr)
	}

	// Extract mongod
	gzReader, gzErr := gzip.NewReader(tgzTempFile)
	if gzErr != nil {
		return "", fmt.Errorf("error initializing gzip reader from %s: %w, %s", tgzTempFile.Name(), gzErr, urlStr)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	for {
		nextFile, tarErr := tarReader.Next()
		if tarErr == io.EOF {
			return "", fmt.Errorf("did not find a mongod binary in the tar from %s", urlStr)
		}
		if tarErr != nil {
			return "", fmt.Errorf("error reading from tar: %s", tarErr)
		}

		if strings.HasSuffix(nextFile.Name, "bin/mongod") {
			err := saveFile(path.Join(dirPath, filepath.Base(nextFile.Name)), tarReader, logger)
			if err != nil {
				return "", err
			}
			break
		}
	}

	logger.Infof("finished downloading mongod to %s in %s", mongodPath, time.Since(downloadStartTime).String())

	return mongodPath, nil
}

// saveFile writes the mongod binary to mongodPath.
//
// Parallel test processes share the cache, and a process that finds
// mongodPath runs it at once. So the binary must be complete and executable
// before it appears there: each writer chmods its own temp file and then
// renames it into place, which is atomic.
func saveFile(mongodPath string, tarReader *tar.Reader, logger *memongolog.Logger) error {
	mkdirErr := Afs.MkdirAll(path.Dir(mongodPath), 0755)
	if mkdirErr != nil {
		return fmt.Errorf("error creating directory %s: %s", path.Dir(mongodPath), mkdirErr)
	}

	mongodTmpFile, tmpFileErr := Afs.TempFile("", "")
	if tmpFileErr != nil {
		return fmt.Errorf("error creating temp file for mongod: %s", tmpFileErr)
	}
	// Some filesystems report the new name after a rename, so keep the
	// temp path for cleanup.
	tmpPath := mongodTmpFile.Name()
	renamed := false
	defer func() {
		_ = mongodTmpFile.Close()
		if !renamed {
			_ = Afs.Remove(tmpPath)
		}
	}()

	_, writeErr := io.Copy(mongodTmpFile, tarReader)
	if writeErr != nil {
		return fmt.Errorf("error writing mongod binary at %s: %s", tmpPath, writeErr)
	}

	_ = mongodTmpFile.Close()

	chmodErr := Afs.Chmod(tmpPath, 0755)
	if chmodErr != nil {
		return fmt.Errorf("error chmod-ing mongod binary at %s: %s", tmpPath, chmodErr)
	}

	renameErr := Afs.Rename(tmpPath, mongodPath)
	if renameErr == nil {
		renamed = true
		return nil
	}

	linkErr := &os.LinkError{}
	if !errors.As(renameErr, &linkErr) {
		return fmt.Errorf("error moving mongod binary from %s to %s: %s", tmpPath, mongodPath, renameErr)
	}

	// /tmp is on another filesystem: copy the binary into the cache
	// directory first, so the last step is still an atomic rename.
	logger.Debugf("Unable to move %s to %s, copying instead", tmpPath, mongodPath)

	return copyIntoPlace(tmpPath, mongodPath)
}

// copyIntoPlace copies srcPath to a temp file next to mongodPath, makes it
// executable, and renames it to mongodPath.
func copyIntoPlace(srcPath, mongodPath string) error {
	content, err := Afs.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("read file err: %w", err)
	}

	dstTmpFile, err := Afs.TempFile(path.Dir(mongodPath), "mongod-")
	if err != nil {
		return fmt.Errorf("creating temp file for mongod in %s: %w", path.Dir(mongodPath), err)
	}
	dstTmpPath := dstTmpFile.Name()
	renamed := false
	defer func() {
		_ = dstTmpFile.Close()
		if !renamed {
			_ = Afs.Remove(dstTmpPath)
		}
	}()

	_, copyErr := dstTmpFile.Write(content)
	if copyErr != nil {
		return fmt.Errorf("error copying mongod binary from %s to %s: %w", srcPath, dstTmpPath, copyErr)
	}

	_ = dstTmpFile.Close()

	chmodErr := Afs.Chmod(dstTmpPath, 0755)
	if chmodErr != nil {
		return fmt.Errorf("error chmod-ing mongod binary at %s: %w", dstTmpPath, chmodErr)
	}

	renameErr := Afs.Rename(dstTmpPath, mongodPath)
	if renameErr != nil {
		return fmt.Errorf("error moving mongod binary from %s to %s: %w", dstTmpPath, mongodPath, renameErr)
	}
	renamed = true

	return nil
}

// After the download a tarball, we extract it to a directory in the cache.
// We want the name of this directory to be both human-redable, and also
// unique (no two URLs should have the same directory name). We can't just
// use the name of the tarball, because the URL can be passed in by the
// user (so https://mongodb.org/dl/linux/foobar.tgz has to have a different
// path than https://mymirror.org/dl/linux/foobar.tgz).
//
// To meet these requirements, we name the directory <basename>_<hash>, where
// basname is the last path element of the URL stripped of any non-path-safe
// characters, and the hash is the first 10 characters of the sha256 checksum of
// the whole URL.
func directoryNameForURL(urlStr string) (string, error) {
	shasum := sha256.New()
	_, _ = shasum.Write([]byte(urlStr))

	shahex := hex.EncodeToString(shasum.Sum(nil))
	hash := shahex[0:10]

	urlParsed, parseErr := url.Parse(urlStr)
	if parseErr != nil {
		return "", fmt.Errorf("could not parse url: %s", parseErr)
	}

	basename := sanitizeFilename(path.Base(urlParsed.Path))

	return fmt.Sprintf("%s_%s", basename, hash), nil
}

var filenameUnsafeCharRegex = regexp.MustCompile("[^a-zA-Z0-9_-]")

func sanitizeFilename(unsanitized string) string {
	return filenameUnsafeCharRegex.ReplaceAllString(unsanitized, "_")
}
