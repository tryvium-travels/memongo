package mongobin

import "fmt"

// GetDownloadURL returns the download URL to download the binary
// from the MongoDB website
func (spec *DownloadSpec) GetDownloadURL() string {
	archiveName := "mongodb-"

	if spec.Platform == "linux" {
		archiveName += "linux-" + spec.Arch + "-"

		if spec.OSName != "" {
			archiveName += spec.OSName + "-"
		}

		archiveName += spec.Version + ".tgz"
	} else {
		if spec.SSLBuildNeeded {
			archiveName += "osx-ssl-"
		} else {
			archiveName += "macos-"
		}

		archiveName += spec.Arch + "-" + spec.Version + ".tgz"
	}

	return fmt.Sprintf(
		"https://fastdl.mongodb.org/%s/%s",
		spec.Platform,
		archiveName,
	)
}

// GetShellDownloadURL returns the download URL to get the mongosh utility. This just returns a single linux TGZ file.
func (spec *DownloadSpec) GetShellDownloadURL() string {
	archiveName := "https://downloads.mongodb.com/compass/mongosh-1.1.8-linux-x64.tgz"
	return archiveName
}
