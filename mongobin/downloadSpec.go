package mongobin

import (
	"fmt"
	"io/ioutil"
	"runtime"
	"strconv"
	"strings"

	"github.com/acobaugh/osrelease"
)

// We define these as package vars so we can override it in tests
var EtcOsRelease = "/etc/os-release"
var EtcRedhatRelease = "/etc/redhat-release"
var GoOS = runtime.GOOS
var GoArch = runtime.GOARCH

// DownloadSpec specifies what copy of MongoDB to download
type DownloadSpec struct {
	// Version is what version of MongoDB to download
	Version string

	// Platform is "osx" or "linux"
	Platform string

	// SSLBuildNeeded is "ssl" if we need to download the SSL build for macOS
	// (needed for <4.2)
	SSLBuildNeeded bool

	// Arch is one of:
	// - x86_64
	// - arm64
	// - aarch64
	Arch string

	// OSName is one of:
	// - ubuntu2204
	// - ubuntu2004
	// - ubuntu1804
	// - ubuntu1604
	// - ubuntu1404
	// - debian10
	// - debian92
	// - debian81
	// - suse12
	// - rhel70
	// - rhel80
	// - rhel62
	// - amazon
	// - amazon2
	// - "" for other linux or for MacOS
	OSName string
}

// MakeDownloadSpec returns a DownloadSpec for the current operating system
func MakeDownloadSpec(version string) (*DownloadSpec, error) {
	parsedVersion, versionErr := parseVersion(version)
	if versionErr != nil {
		return nil, versionErr
	}

	platform, platformErr := detectPlatform()
	if platformErr != nil {
		return nil, platformErr
	}

	ssl := false
	if platform == "osx" && !versionGTE(parsedVersion, []int{4, 2, 0}) {
		// pre-4.0, the MacOS builds had a special "ssl" designator in the URL
		ssl = true
	}

	osName := detectOSName(parsedVersion)
	if platform == "linux" && osName == "" && versionGTE(parsedVersion, []int{4, 2, 0}) {
		return nil, &UnsupportedSystemError{msg: "MongoDB 4.2 removed support for generic linux tarballs. Specify the download URL manually or use a supported distro. See: https://www.mongodb.com/blog/post/a-proposal-to-endoflife-our-generic-linux-tar-packages"}
	}

	arch, archErr := detectArch(platform, osName, parsedVersion)
	if archErr != nil {
		return nil, archErr
	}

	return &DownloadSpec{
		Version:        version,
		Arch:           arch,
		SSLBuildNeeded: ssl,
		Platform:       platform,
		OSName:         osName,
	}, nil
}

func parseVersion(version string) ([]int, error) {
	versionParts := strings.Split(version, ".")
	if len(versionParts) < 3 {
		return nil, &UnsupportedMongoVersionError{
			version: version,
			msg:     "MongoDB version number must be in the form x.y.z",
		}
	}

	majorVersion, majErr := strconv.Atoi(versionParts[0])
	if majErr != nil {
		return nil, &UnsupportedMongoVersionError{
			version: version,
			msg:     "Could not parse major version",
		}
	}

	minorVersion, minErr := strconv.Atoi(versionParts[1])
	if minErr != nil {
		return nil, &UnsupportedMongoVersionError{
			version: version,
			msg:     "Could not parse minor version",
		}
	}

	patchVersion, patchErr := strconv.Atoi(versionParts[2])
	if patchErr != nil {
		return nil, &UnsupportedMongoVersionError{
			version: version,
			msg:     "Could not parse patch version",
		}
	}

	if (majorVersion < 3) || ((majorVersion == 3) && (minorVersion < 2)) {
		return nil, &UnsupportedMongoVersionError{
			version: version,
			msg:     "Only Mongo version 3.2 and above are supported",
		}
	}

	return []int{majorVersion, minorVersion, patchVersion}, nil
}

func detectPlatform() (string, error) {
	switch GoOS {
	case "darwin":
		return "osx", nil
	case "linux":
		return "linux", nil
	default:
		return "", &UnsupportedSystemError{msg: "your platform, " + GoOS + ", is not supported"}
	}
}

func detectArch(platform string, osName string, mongoVersion []int) (string, error) {
	switch GoArch {
	case "amd64":
		return "x86_64", nil
	case "arm64":
		return arm64ArchFromOSNameAndVersion(platform, osName, mongoVersion)
	default:
		return "", &UnsupportedSystemError{msg: "your architecture, " + GoArch + ", is not supported"}
	}
}

func arm64ArchFromOSNameAndVersion(platform string, osName string, mongoVersion []int) (string, error) {
	// version numbers extracted from https://www.mongodb.com/download-center/community/releases/archive
	if !versionGTE(mongoVersion, []int{3, 4, 0}) {
		return "", &UnsupportedSystemError{msg: "arm64 support was introduced in Mongo 3.4.0"}
	}

	// ubuntu1604 arm support was introduced in version 3.4.0 and removed in version 4.0.27
	if osName == "ubuntu1604" && !versionGTE(mongoVersion, []int{4, 0, 27}) {
		return "arm64", nil
	}

	if osName == "ubuntu1804" && versionGTE(mongoVersion, []int{4, 2, 0}) {
		return "aarch64", nil
	}

	if osName == "ubuntu2004" && versionGTE(mongoVersion, []int{4, 4, 0}) {
		return "aarch64", nil
	}

	if osName == "ubuntu2204" && versionGTE(mongoVersion, []int{6, 0, 4}) {
		return "aarch64", nil
	}

	if osName == "amazon2" && versionGTE(mongoVersion, []int{4, 2, 13}) {
		return "aarch64", nil
	}

	// TODO: "rhel82" isn't a value that osName can have yet as osNameFromOsRelease doesn't support this version
	if osName == "rhel82" && versionGTE(mongoVersion, []int{4, 4, 4}) {
		return "aarch64", nil
	}

	if platform == "osx" && versionGTE(mongoVersion, []int{6, 0, 0}) {
		return "arm64", nil
	}

	os := osName
	if os == "" {
		os = platform
	}

	versionString := fmt.Sprintf("%d.%d.%d", mongoVersion[0], mongoVersion[1], mongoVersion[2])
	return "", &UnsupportedSystemError{msg: "Mongo doesn't support your environment, " + os + "/" + GoArch + ", on version " + versionString}
}

func detectOSName(mongoVersion []int) string {
	if GoOS != "linux" {
		// Not on Linux
		return ""
	}

	osRelease, osReleaseErr := osrelease.ReadFile(EtcOsRelease)
	if osReleaseErr == nil {
		return osNameFromOsRelease(osRelease, mongoVersion)
	}

	// We control etcRedhatRelease
	//nolint:gosec
	redhatRelease, redhatReleaseErr := ioutil.ReadFile(EtcRedhatRelease)
	if redhatReleaseErr == nil {
		return osNameFromRedhatRelease(string(redhatRelease))
	}

	return ""
}

func versionGTE(a []int, b []int) bool {
	if a[0] > b[0] {
		return true
	}

	if a[0] < b[0] {
		return false
	}

	if a[1] > b[1] {
		return true
	}

	if a[1] < b[1] {
		return false
	}

	return a[2] >= b[2]
}

func osNameFromOsRelease(osRelease map[string]string, mongoVersion []int) string {
	id := osRelease["ID"]

	majorVersionString := strings.Split(osRelease["VERSION_ID"], ".")[0]
	majorVersion, err := strconv.Atoi(majorVersionString)
	if err != nil {
		return ""
	}

	switch id {
	case "ubuntu":
		return osNameFromUbuntuRelease(majorVersion, mongoVersion)
	case "sles":
		if majorVersion >= 12 {
			return "suse12"
		}
	case "centos", "rhel":
		if majorVersion >= 8 {
			return "rhel80"
		}
		if majorVersion == 7 {
			return "rhel70"
		}
	case "debian":
		return osNameFromDebianRelease(majorVersion, mongoVersion)
	case "amzn":
		return osNameFromAmznRelease(majorVersion, mongoVersion)
	}

	return ""
}
func osNameFromUbuntuRelease(majorVersion int, mongoVersion []int) string {
	if majorVersion >= 22 && versionGTE(mongoVersion, []int{4, 0, 1}) {
		return "ubuntu2204"
	}
	if majorVersion >= 20 && versionGTE(mongoVersion, []int{4, 0, 1}) {
		return "ubuntu2004"
	}
	if majorVersion >= 18 && versionGTE(mongoVersion, []int{4, 0, 1}) {
		return "ubuntu1804"
	}
	if majorVersion >= 16 && versionGTE(mongoVersion, []int{3, 2, 7}) {
		return "ubuntu1604"
	}
	if majorVersion >= 14 {
		return "ubuntu1404"
	}
	return ""
}

func osNameFromDebianRelease(majorVersion int, mongoVersion []int) string {
	if majorVersion >= 11 && versionGTE(mongoVersion, []int{5, 0, 8}) {
		return "debian11"
	}
	if majorVersion >= 10 && versionGTE(mongoVersion, []int{4, 2, 1}) {
		return "debian10"
	}
	if majorVersion >= 9 && versionGTE(mongoVersion, []int{3, 6, 5}) {
		return "debian92"
	}
	if majorVersion >= 8 && versionGTE(mongoVersion, []int{3, 2, 8}) {
		return "debian81"
	}
	return ""
}

func osNameFromAmznRelease(majorVersion int, mongoVersion []int) string {
	if majorVersion == 2 && versionGTE(mongoVersion, []int{4, 0, 0}) {
		return "amazon2"
	}

	// Version before 2 has the release date, not a real version number
	return "amazon"
}

func osNameFromRedhatRelease(redhatRelease string) string {
	// RHEL 7 uses /etc/os-release, so we're just detecting RHEL 6 here
	if strings.Contains(redhatRelease, "release 6") {
		return "rhel62"
	}

	return ""
}
