package mongobin_test

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tryvium-travels/memongo/mongobin"
)

const (
	testMongoVersion   = "4.0.5"
	latestMongoVersion = "6.0.4"
)

func TestMakeDownloadSpec(t *testing.T) {
	tests := map[string]struct {
		mongoVersion string
		etcFolder    string
		goOs         string
		goArch       string

		expectedSpec  *mongobin.DownloadSpec
		expectedError string
	}{
		"mac and older mongo": {
			goOs: "darwin",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        testMongoVersion,
				Platform:       "osx",
				SSLBuildNeeded: true,
				Arch:           "x86_64",
				OSName:         "",
			},
		},
		"mac and newer mongo": {
			goOs:         "darwin",
			mongoVersion: "4.2.1",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        "4.2.1",
				Platform:       "osx",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "",
			},
		},
		"arm64 mac and newer mongo": {
			goOs:         "darwin",
			goArch:       "arm64",
			mongoVersion: latestMongoVersion,

			expectedSpec: &mongobin.DownloadSpec{
				Version:        latestMongoVersion,
				Platform:       "osx",
				SSLBuildNeeded: false,
				Arch:           "arm64",
				OSName:         "",
			},
		},
		"windows": {
			goOs: "windows",

			expectedError: "memongo does not support automatic downloading on your system: your platform, windows, is not supported",
		},
		"ubuntu 22.04 newer mongo": {
			mongoVersion: latestMongoVersion,
			etcFolder:    "ubuntu2204",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        latestMongoVersion,
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "ubuntu2204",
			},
		},
		"arm64 ubuntu 22.04 newer mongo": {
			mongoVersion: latestMongoVersion,
			etcFolder:    "ubuntu2204",
			goArch:       "arm64",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        latestMongoVersion,
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "aarch64",
				OSName:         "ubuntu2204",
			},
		},
		"ubuntu 22.04": {
			mongoVersion: testMongoVersion,
			etcFolder:    "ubuntu2204",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        testMongoVersion,
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "ubuntu1804", // Ubuntu 22.04 is not supported by Mongo 4.0.5, so it falls back to Ubuntu 18.04
			},
		},
		"ubuntu 20.04": {
			mongoVersion: testMongoVersion,
			etcFolder:    "ubuntu2004",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        testMongoVersion,
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "ubuntu1804", // Ubuntu 20.04 is not supported by Mongo 4.0.5, so it falls back to Ubuntu 18.04
			},
		},
		"arm64 ubuntu 20.04 and newer mongo": {
			mongoVersion: latestMongoVersion,
			etcFolder:    "ubuntu2004",
			goArch:       "arm64",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        latestMongoVersion,
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "aarch64",
				OSName:         "ubuntu2004",
			},
		},
		"ubuntu 18.04": {
			mongoVersion: testMongoVersion,
			etcFolder:    "ubuntu1804",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        testMongoVersion,
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "ubuntu1804",
			},
		},
		"arm64 ubuntu 18.04 and newer mongo": {
			mongoVersion: latestMongoVersion,
			etcFolder:    "ubuntu1804",
			goArch:       "arm64",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        latestMongoVersion,
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "aarch64",
				OSName:         "ubuntu1804",
			},
		},
		"ubuntu 18.04 older mongo": {
			mongoVersion: "4.0.0",
			etcFolder:    "ubuntu1804",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        "4.0.0",
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "ubuntu1604",
			},
		},
		"ubuntu 18.04 much older mongo": {
			mongoVersion: "3.2.6",
			etcFolder:    "ubuntu1804",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        "3.2.6",
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "ubuntu1404",
			},
		},
		"ubuntu 16.04": {
			mongoVersion: testMongoVersion,
			etcFolder:    "ubuntu1604",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        testMongoVersion,
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "ubuntu1604",
			},
		},
		"arm64 ubuntu 16.04": {
			mongoVersion: testMongoVersion,
			etcFolder:    "ubuntu1604",
			goArch:       "arm64",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        testMongoVersion,
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "arm64",
				OSName:         "ubuntu1604",
			},
		},
		"ubuntu 16.04 older mongo": {
			mongoVersion: "3.2.6",
			etcFolder:    "ubuntu1604",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        "3.2.6",
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "ubuntu1404",
			},
		},
		"SUSE 12": {
			mongoVersion: testMongoVersion,
			etcFolder:    "suse12",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        testMongoVersion,
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "suse12",
			},
		},
		"CENTOS 8": {
			mongoVersion: testMongoVersion,
			etcFolder:    "centos8",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        testMongoVersion,
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "rhel80",
			},
		},
		"RHEL 7": {
			mongoVersion: testMongoVersion,
			etcFolder:    "rhel7",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        testMongoVersion,
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "rhel70",
			},
		},
		"RHEL 6": {
			mongoVersion: testMongoVersion,
			etcFolder:    "rhel6",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        testMongoVersion,
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "rhel62",
			},
		},
		"Debian buster": {
			mongoVersion: testMongoVersion,
			etcFolder:    "debianbuster",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        testMongoVersion,
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "debian92",
			},
		},
		"Debian bullseye new mongo": {
			mongoVersion: latestMongoVersion,
			etcFolder:    "debianbullseye",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        latestMongoVersion,
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "debian11",
			},
		},
		"Debian buster new mongo": {
			mongoVersion: "4.2.1",
			etcFolder:    "debianbuster",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        "4.2.1",
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "debian10",
			},
		},
		"Debian buster older mongo": {
			mongoVersion: "3.6.4",
			etcFolder:    "debianbuster",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        "3.6.4",
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "debian81",
			},
		},
		"Debian buster much older mongo": {
			mongoVersion: "3.2.7",
			etcFolder:    "debianbuster",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        "3.2.7",
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "",
			},
		},
		"Debian stretch": {
			mongoVersion: testMongoVersion,
			etcFolder:    "debianstretch",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        testMongoVersion,
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "debian92",
			},
		},
		"Debian stretch older mongo": {
			mongoVersion: "3.6.4",
			etcFolder:    "debianstretch",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        "3.6.4",
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "debian81",
			},
		},
		"Debian stretch much older mongo": {
			mongoVersion: "3.2.7",
			etcFolder:    "debianstretch",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        "3.2.7",
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "",
			},
		},
		"Debian jessie": {
			mongoVersion: testMongoVersion,
			etcFolder:    "debianjessie",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        testMongoVersion,
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "debian81",
			},
		},
		"Debian jessie older mongo": {
			mongoVersion: "3.2.7",
			etcFolder:    "debianjessie",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        "3.2.7",
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "",
			},
		},
		"Amazon Linux": {
			mongoVersion: testMongoVersion,
			etcFolder:    "amazon",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        testMongoVersion,
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "amazon",
			},
		},
		"Amazon Linux 2": {
			mongoVersion: testMongoVersion,
			etcFolder:    "amazon2",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        testMongoVersion,
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "amazon2",
			},
		},
		"ARM64 Amazon Linux 2 and newer mongo": {
			mongoVersion: latestMongoVersion,
			etcFolder:    "amazon2",
			goArch:       "arm64",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        latestMongoVersion,
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "aarch64",
				OSName:         "amazon2",
			},
		},
		"Amazon Linux 2 older mongo": {
			mongoVersion: "3.6.5",
			etcFolder:    "amazon2",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        "3.6.5",
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "amazon",
			},
		},
		"Other Linux": {
			etcFolder: "other-linux",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        testMongoVersion,
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "",
			},
		},
		"Empty /etc": {
			etcFolder: "empty-etc",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        testMongoVersion,
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "",
			},
		},
		"Other OS": {
			goOs: "foo",

			expectedError: "memongo does not support automatic downloading on your system: your platform, foo, is not supported",
		},
		"Other Arch": {
			goArch: "386",

			expectedError: "memongo does not support automatic downloading on your system: your architecture, 386, is not supported",
		},
		"MongoDB 4.2": {
			etcFolder:    "ubuntu1804",
			mongoVersion: "4.2.3",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        "4.2.3",
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "ubuntu1804",
			},
		},
		"MongoDB 3.6": {
			mongoVersion: "3.6.1",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        "3.6.1",
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "",
			},
		},
		"MongoDB 3.4": {
			mongoVersion: "3.4.0",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        "3.4.0",
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "",
			},
		},
		"MongoDB 3.2": {
			mongoVersion: "3.2.0",

			expectedSpec: &mongobin.DownloadSpec{
				Version:        "3.2.0",
				Platform:       "linux",
				SSLBuildNeeded: false,
				Arch:           "x86_64",
				OSName:         "",
			},
		},
		"MongoDB Unsupported arch for version": {
			mongoVersion: "3.3.0",
			goArch:       "arm64",

			expectedError: "memongo does not support automatic downloading on your system: arm64 support was introduced in Mongo 3.4.0",
		},
		"MongoDB Unsupported newer version for arm64 ubuntu1604": {
			mongoVersion: "6.0.0",
			etcFolder:    "ubuntu1604",
			goArch:       "arm64",

			expectedError: "memongo does not support automatic downloading on your system: Mongo doesn't support your environment, ubuntu1604/arm64, on version 6.0.0",
		},
		"MongoDB Unsupported older version for arm64 ubuntu1804": {
			mongoVersion: "4.1.0",
			etcFolder:    "ubuntu1804",
			goArch:       "arm64",

			expectedError: "memongo does not support automatic downloading on your system: Mongo doesn't support your environment, ubuntu1804/arm64, on version 4.1.0",
		},
		"MongoDB Unsupported older version for arm64 ubuntu2004": {
			mongoVersion: "4.1.0",
			etcFolder:    "ubuntu2004",
			goArch:       "arm64",

			expectedError: "memongo does not support automatic downloading on your system: Mongo doesn't support your environment, ubuntu1804/arm64, on version 4.1.0", // The OS name is wrong because we don't support Ubuntu 20.04 on Mongo 4.1.0 so it falls back to Ubuntu 18.04
		},
		"MongoDB Unsupported older version for arm64 ubuntu2204": {
			mongoVersion: "4.1.0",
			etcFolder:    "ubuntu2204",
			goArch:       "arm64",

			expectedError: "memongo does not support automatic downloading on your system: Mongo doesn't support your environment, ubuntu1804/arm64, on version 4.1.0", // The OS name is wrong because we don't support Ubuntu 22.04 on Mongo 4.1.0 so it falls back to Ubuntu 18.04
		},
		"MongoDB Unsupported older version for arm64 amazon2": {
			mongoVersion: "4.1.0",
			etcFolder:    "amazon2",
			goArch:       "arm64",

			expectedError: "memongo does not support automatic downloading on your system: Mongo doesn't support your environment, amazon2/arm64, on version 4.1.0",
		},
		"MongoDB Unsupported older version for arm64 rhel82": {
			mongoVersion: "4.1.0",
			etcFolder:    "rhel82",
			goArch:       "arm64",

			expectedError: "memongo does not support automatic downloading on your system: Mongo doesn't support your environment, linux/arm64, on version 4.1.0",
		},
		"MongoDB Unsupported version for arm mac": {
			mongoVersion: "4.1.0",
			goOs:         "darwin",
			goArch:       "arm64",

			expectedError: "memongo does not support automatic downloading on your system: Mongo doesn't support your environment, osx/arm64, on version 4.1.0",
		},
		"MongoDB 3.0": {
			mongoVersion: "3.0.2",

			expectedError: "memongo does not support MongoDB version \"3.0.2\": Only Mongo version 3.2 and above are supported",
		},
		"MongoDB 2.8": {
			mongoVersion: "2.8.10",

			expectedError: "memongo does not support MongoDB version \"2.8.10\": Only Mongo version 3.2 and above are supported",
		},
		"MongoDB bad version": {
			mongoVersion: "asdf",

			expectedError: "memongo does not support MongoDB version \"asdf\": MongoDB version number must be in the form x.y.z",
		},
		"MongoDB bad major version": {
			mongoVersion: "d.4.0",

			expectedError: "memongo does not support MongoDB version \"d.4.0\": Could not parse major version",
		},
		"MongoDB bad minor version": {
			mongoVersion: "4.d.0",

			expectedError: "memongo does not support MongoDB version \"4.d.0\": Could not parse minor version",
		},
		"MongoDB bad patch version": {
			mongoVersion: "4.0.d",

			expectedError: "memongo does not support MongoDB version \"4.0.d\": Could not parse patch version",
		},
		"MongoDB missing patch version": {
			mongoVersion: "4.0",

			expectedError: "memongo does not support MongoDB version \"4.0\": MongoDB version number must be in the form x.y.z",
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			if test.etcFolder == "" {
				mongobin.EtcOsRelease = "./testdata/etc/empty-etc/os-release"
				mongobin.EtcRedhatRelease = "./testdata/etc/empty-etc/redhat-release"
			} else {
				mongobin.EtcOsRelease = "./testdata/etc/" + test.etcFolder + "/os-release"
				mongobin.EtcRedhatRelease = "./testdata/etc/" + test.etcFolder + "/redhat-release"
			}

			if test.goArch == "" {
				mongobin.GoArch = "amd64"
			} else {
				mongobin.GoArch = test.goArch
			}

			if test.goOs == "" {
				mongobin.GoOS = "linux"
			} else {
				mongobin.GoOS = test.goOs
			}

			defer func() {
				mongobin.EtcOsRelease = "/etc/os-release"
				mongobin.EtcRedhatRelease = "/etc/redhat-release"
				mongobin.GoOS = runtime.GOOS
				mongobin.GoArch = runtime.GOARCH
			}()

			mongoVersion := test.mongoVersion
			if mongoVersion == "" {
				mongoVersion = testMongoVersion
			}

			result, err := mongobin.MakeDownloadSpec(mongoVersion)

			if test.expectedError != "" {
				require.Error(t, err)
				require.Equal(t, test.expectedError, err.Error())
			} else {
				require.NoError(t, err)
			}

			if test.expectedSpec != nil {
				require.Equal(t, test.expectedSpec, result)
			} else {
				require.Nil(t, result)
			}
		})
	}
}
