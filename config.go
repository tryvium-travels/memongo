package memongo

import (
	"fmt"
	"log"
	"net"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ntaylor-barnett/memongo/memongolog"
	"github.com/ntaylor-barnett/memongo/mongobin"
)

// Options is the configuration options for a launched MongoDB binary
type Options struct {
	// ShouldUseReplica indicates whether a replica should be used. If this is not specified,
	// no replica will be used and mongo server will be run as standalone.
	ShouldUseReplica bool

	// Port to run MongoDB on. If this is not specified, a random (OS-assigned)
	// port will be used
	Port int

	// Path to the cache for downloaded mongod binaries. Defaults to the
	// system cache location.
	CachePath string

	// If DownloadURL and MongodBin are not given, this version of MongoDB will
	// be downloaded
	MongoVersion string

	// If given, mongod will be downloaded from this URL instead of the
	// auto-detected URL based on the current platform and MongoVersion
	DownloadURL string

	// If given, this binary will be run instead of downloading a mongod binary
	MongodBin string

	// If given, this binary will be run instead of downloading a mongosh binary
	MongoShellBin string

	// Logger for printing messages. Defaults to printing to stdout.
	Logger *log.Logger

	// A LogLevel to log at. Defaults to LogLevelInfo.
	LogLevel memongolog.LogLevel

	// How long to wait for mongod to start up and report a port number. Does
	// not include download time, only startup time. Defaults to 10 seconds.
	StartupTimeout time.Duration

	// The URL to get mongosh from
	ShellDownloadURL string

	// If set, pass the --auth flag to mongod. This will allow tests to setup
	// authentication.
	Auth bool
}

func (opts *Options) fillDefaults() error {
	if opts.MongodBin == "" {
		opts.MongodBin = os.Getenv("MEMONGO_MONGOD_BIN")
	}
	if opts.MongoShellBin == "" {
		opts.MongoShellBin = os.Getenv("MEMONGO_MONGOSH_BIN")
	}
	if opts.MongodBin == "" || opts.MongoShellBin == "" {
		// The user didn't give us a local path to a binary. That means we need
		// a download URL and a cache path.

		// Determine the cache path
		if opts.CachePath == "" {
			opts.CachePath = os.Getenv("MEMONGO_CACHE_PATH")
		}
		if opts.CachePath == "" && os.Getenv("XDG_CACHE_HOME") != "" {
			opts.CachePath = path.Join(os.Getenv("XDG_CACHE_HOME"), "memongo")
		}
		if opts.CachePath == "" {
			if runtime.GOOS == "darwin" {
				opts.CachePath = path.Join(os.Getenv("HOME"), "Library", "Caches", "memongo")
			} else {
				opts.CachePath = path.Join(os.Getenv("HOME"), ".cache", "memongo")
			}
		}

		// Determine the download URL
		if opts.DownloadURL == "" {
			opts.DownloadURL = os.Getenv("MEMONGO_DOWNLOAD_URL")
		}
		if opts.DownloadURL == "" {
			if opts.MongoVersion == "" {
				return fmt.Errorf("one of MongoVersion, DownloadURL, or MongodBin must be given")
			}
			spec, err := mongobin.MakeDownloadSpec(opts.MongoVersion)
			if err != nil {
				return err
			}

			opts.DownloadURL = spec.GetDownloadURL()
		}
		if opts.MongoShellBin != "" {
			// if the shell bin has been provided, we should leave the downloadURL as empty
			opts.ShellDownloadURL = ""
		} else if opts.ShellDownloadURL == "" {
			spec, err := mongobin.MakeDownloadSpec(opts.MongoVersion)
			if err != nil {
				return err
			}
			opts.ShellDownloadURL = spec.GetShellDownloadURL()
		}
	}

	// Determine the port number
	if opts.Port == 0 {
		mongoVersionEnv := os.Getenv("MEMONGO_MONGOD_PORT")
		if mongoVersionEnv != "" {
			port, err := strconv.Atoi(mongoVersionEnv)

			if err != nil {
				return fmt.Errorf("error parsing MEMONGO_MONGOD_PORT: %s", err)
			}

			opts.Port = port
		}
	}

	if opts.Port == 0 {
		port, err := getFreePort()
		if err != nil {
			return fmt.Errorf("error finding a free port: %s", err)
		}

		opts.Port = port

		if opts.StartupTimeout == 0 {
			opts.StartupTimeout = 10 * time.Second
		}
	}

	return nil
}

func (opts *Options) getLogger() *memongolog.Logger {
	return memongolog.New(opts.Logger, opts.LogLevel)
}

func (opts *Options) getOrDownloadBinPath() (*mongobin.MongoPaths, error) {

	// Download or fetch from cache
	binPath, err := mongobin.GetOrDownloadMongod(opts.DownloadURL, opts.ShellDownloadURL, opts.CachePath, opts.getLogger())
	if err != nil {
		return nil, err
	}
	if opts.MongodBin != "" {
		binPath.Mongod = opts.MongodBin
	}
	if opts.MongoShellBin != "" {
		binPath.Mongosh = opts.MongoShellBin
	}

	return binPath, nil
}

func parseMongoMajorVersion(version string) int {
	strParts := strings.Split(version, ".")
	if len(strParts) == 0 {
		return 0
	}

	maj, err := strconv.Atoi(strParts[0])
	if err != nil {
		return 0
	}

	return maj
}

func getFreePort() (int, error) {
	// Based on: https://github.com/phayes/freeport/blob/master/freeport.go
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
