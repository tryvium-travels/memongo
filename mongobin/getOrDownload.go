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
	"regexp"
	"strings"
	"time"

	"github.com/ntaylor-barnett/memongo/memongolog"
	"github.com/spf13/afero"
)

var Afs afero.Afero

func init() {
	Afs = afero.Afero{
		Fs: afero.NewOsFs(),
	}
}

func suffixInArray(target string, list []string) (string, bool) {
	for _, v := range list {
		if strings.HasSuffix(target, v) {
			return v, true
		}
	}
	return "", false
}

func extractURLToDest(logger *memongolog.Logger, downloadUrl, outputPath string, files []string) error {
	logger.Infof("downloading from %s to %s", downloadUrl, outputPath)
	defer func() {
		logger.Infof("finished processing %s", downloadUrl)
	}()
	mongodPath := outputPath
	urlStr := downloadUrl
	// Download the file
	// nolint:gosec
	resp, httpGetErr := http.Get(urlStr)
	if httpGetErr != nil {
		return fmt.Errorf("error getting tarball from %s: %s", urlStr, httpGetErr)
	}
	defer resp.Body.Close()

	tgzTempFile, tmpFileErr := Afs.TempFile("", "")
	if tmpFileErr != nil {
		return fmt.Errorf("error creating temp file for tarball: %s", tmpFileErr)
	}
	defer func() {
		_ = tgzTempFile.Close()
		_ = Afs.Remove(tgzTempFile.Name())
	}()

	_, copyErr := io.Copy(tgzTempFile, resp.Body)
	if copyErr != nil {
		return fmt.Errorf("error downloading tarball from %s: %s", urlStr, copyErr)
	}

	_, seekErr := tgzTempFile.Seek(0, 0)
	if seekErr != nil {
		return fmt.Errorf("error seeking back to start of file: %s", seekErr)
	}

	// Extract mongod
	gzReader, gzErr := gzip.NewReader(tgzTempFile)
	if gzErr != nil {
		return fmt.Errorf("error intializing gzip reader from %s: %s", tgzTempFile.Name(), gzErr)
	}

	tarReader := tar.NewReader(gzReader)
	filesFound := 0
	for {
		nextFile, tarErr := tarReader.Next()
		if tarErr == io.EOF {
			return fmt.Errorf("did not find a mongod binary in the tar from %s", urlStr)
		}
		if tarErr != nil {
			return fmt.Errorf("error reading from tar: %s", tarErr)
		}

		if fName, ok := suffixInArray(nextFile.Name, files); ok { //  strings.HasSuffix(nextFile.Name, "bin/mongod")
			filesFound++

			destPath := path.Join(mongodPath, fName)
			mkdirErr := Afs.MkdirAll(path.Dir(destPath), 0755)
			if mkdirErr != nil {
				return fmt.Errorf("error creating directory %s: %s", mongodPath, mkdirErr)
			}

			// Extract to a temp file first, then copy to the destination, so we get
			// atomic behavior if there's multiple parallel downloaders
			mongodTmpFile, tmpFileErr := Afs.TempFile("", "")
			if tmpFileErr != nil {
				return fmt.Errorf("error creating temp file for mongod: %s", tmpFileErr)
			}
			defer func() {
				_ = mongodTmpFile.Close()
			}()

			_, writeErr := io.Copy(mongodTmpFile, tarReader)
			if writeErr != nil {
				return fmt.Errorf("error writing mongod binary at %s: %s", mongodTmpFile.Name(), writeErr)
			}

			_ = mongodTmpFile.Close()

			chmodErr := Afs.Chmod(mongodTmpFile.Name(), 0755)
			if chmodErr != nil {
				return fmt.Errorf("error chmod-ing mongodb binary at %s: %s", mongodTmpFile, chmodErr)
			}

			renameErr := Afs.Rename(mongodTmpFile.Name(), destPath)
			if renameErr != nil {
				linkErr := &os.LinkError{}
				if errors.As(renameErr, &linkErr) {
					// If /tmp is on another filesystem, we have to copy the file instead.
					logger.Debugf("Unable to move %s to %s, copying instead", mongodTmpFile.Name(), mongodPath)
					mongodFile, err := Afs.Create(destPath)
					if err != nil {
						return fmt.Errorf("creating mongod binary at %s: %s", mongodTmpFile, err)
					}
					defer mongodFile.Close()

					_, copyErr := io.Copy(mongodFile, mongodTmpFile)
					if copyErr != nil {
						fmt.Errorf("error copying mongod binary from %s to %s: %s", mongodTmpFile.Name(), mongodPath, copyErr)
					}
				}

				return fmt.Errorf("error writing mongod binary from %s to %s: %s", mongodTmpFile.Name(), mongodPath, renameErr)
			}
			logger.Infof("created %s", destPath)
		}
		if filesFound == len(files) {
			break
		}
	}
	return nil

}

type MongoPaths struct {
	Mongod  string
	Mongosh string
}

// GetOrDownloadMongod returns the path to the mongod binary from the tarball
// at the given URL. If the URL has not yet been downloaded, it's downloaded
// and saved the the cache. If it has been downloaded, the existing mongod
// path is returned.
func GetOrDownloadMongod(urlStr string, mongoshUrl string, cachePath string, logger *memongolog.Logger, force bool) (*MongoPaths, error) {
	var urlToUse string

	if urlStr != "" {
		urlToUse = urlStr
	} else {
		urlToUse = mongoshUrl
	}
	if urlToUse == "" {
		logger.Debugf("nothing to download")
	}
	dirname, dirErr := directoryNameForURL(urlToUse)
	if dirErr != nil {
		return nil, dirErr
	}

	dirPath := path.Join(cachePath, dirname)

	mongodFiles := []string{"bin/mongod"}
	mongoshFiles := []string{"bin/mongocryptd-mongosh", "bin/mongosh"}
	mp := &MongoPaths{
		Mongod:  path.Join(dirPath, mongodFiles[0]),
		Mongosh: path.Join(dirPath, mongoshFiles[1]),
	}
	//mongodPath := path.Join(dirPath, "mongod")
	var reqFiles []string
	if urlStr != "" {
		reqFiles = append(reqFiles, mongodFiles...)
	}
	if mongoshUrl != "" {
		reqFiles = append(reqFiles, mongoshFiles...)
	}
	filesMissing := false
	if !force {
		for _, f := range reqFiles {
			existsInCache, existsErr := Afs.Exists(path.Join(dirPath, f))
			if existsErr != nil {
				return nil, fmt.Errorf("error while checking for mongod in cache: %s", existsErr)
			}
			if !existsInCache {
				filesMissing = true
				break
			}
		}

		// Check the cache

		if !filesMissing {
			logger.Debugf("mongod from %s exists in cache at %s", urlStr, dirPath)
			return mp, nil
		}
	}
	downloadStartTime := time.Now()
	if urlStr != "" {
		err := extractURLToDest(logger, urlStr, dirPath, mongodFiles)
		if err != nil {
			return nil, err
		}
	}
	if mongoshUrl != "" {
		err := extractURLToDest(logger, mongoshUrl, dirPath, mongoshFiles)
		if err != nil {
			return nil, err
		}
	}
	logger.Infof("finished downloading in %s", time.Since(downloadStartTime).String())
	return mp, nil
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
