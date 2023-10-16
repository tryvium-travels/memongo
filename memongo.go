package memongo

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tryvium-travels/memongo/memongolog"
	"github.com/tryvium-travels/memongo/monitor"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const mongoConnectionTemplate = "mongodb://localhost:%d/?directConnection=true"

// Server represents a running MongoDB server
type Server struct {
	cmd        *exec.Cmd
	watcherCmd *exec.Cmd
	dbDir      string
	logger     *memongolog.Logger
	port       int
}

// Start runs a MongoDB server at a given MongoDB version using default options
// and returns the Server.
func Start(version string) (*Server, error) {
	return StartWithOptions(&Options{
		MongoVersion: version,
	})
}

// StartWithOptions is like Start(), but accepts options.
func StartWithOptions(opts *Options) (*Server, error) {
	err := opts.fillDefaults()
	if err != nil {
		return nil, err
	}

	logger := opts.getLogger()

	logger.Infof("Starting MongoDB with options %#v", opts)

	binPath, err := opts.getOrDownloadBinPath()
	if err != nil {
		return nil, err
	}

	logger.Debugf("Using binary %s", binPath)

	// Create a db dir. Even the ephemeralForTest engine needs a dbpath.
	dbDir, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, err
	}

	// Construct the command and attach stdout/stderr handlers

	engine := "ephemeralForTest"
	args := []string{"--dbpath", dbDir, "--port", strconv.Itoa(opts.Port)}
	if opts.ShouldUseReplica {
		engine = "wiredTiger"
		args = append(args, "--replSet", "rs0")
	} else if strings.HasPrefix(opts.MongoVersion, "7.") {
		engine = "wiredTiger"
	}
	if engine == "wiredTiger" {
		args = append(args, "--bind_ip", "localhost")
	}

	if opts.Auth {
		args = append(args, "--auth")
		// A keyfile needs to be specified if auth and a replicaset are used
		if opts.ShouldUseReplica {
			tmpFile, err := ioutil.TempFile("", "keyfile")
			// This library is specifically intended for ephemeral mongo
			// databases so we don't need a lot of security here, however
			// if you're reading this file trying to figure out how to generate
			// a keyfile, please see the official MongoDB documentation on how
			// to do this correctly and securely for a production environment.
			tmpFile.Write([]byte("insecurekeyfile"))
			if err != nil {
				return nil, err
			}
			args = append(args, "--keyFile", tmpFile.Name())
		}
	}

	args = append(args, []string{"--storageEngine", engine}...)

	//  Safe to pass binPath and dbDir
	//nolint:gosec
	cmd := exec.Command(binPath, args...)

	stdoutHandler, startupErrCh, startupPortCh := stdoutHandler(logger)
	cmd.Stdout = stdoutHandler
	cmd.Stderr = stderrHandler(logger)

	logger.Debugf("Starting mongod")

	// Run the server
	err = cmd.Start()
	if err != nil {
		remErr := os.RemoveAll(dbDir)
		if remErr != nil {
			logger.Warnf("error removing data directory: %s", remErr)
		}

		return nil, err
	}

	logger.Debugf("Started mongod; starting watcher")

	// Start a watcher: the watcher is a subprocess that ensure if this process
	// dies, the mongo server will be killed (and not reparented under init)
	watcherCmd, err := monitor.RunMonitor(os.Getpid(), cmd.Process.Pid)
	if err != nil {
		killErr := cmd.Process.Kill()
		if killErr != nil {
			logger.Warnf("error stopping mongo process: %s", killErr)
		}

		remErr := os.RemoveAll(dbDir)
		if remErr != nil {
			logger.Warnf("error removing data directory: %s", remErr)
		}

		return nil, err
	}

	logger.Debugf("Started watcher; waiting for mongod to report port number")
	startupTime := time.Now()

	// Wait for the stdout handler to report the server's port number (or a
	// startup error)
	var port int
	select {
	case p := <-startupPortCh:
		port = p
	case err := <-startupErrCh:
		killErr := cmd.Process.Kill()
		if killErr != nil {
			logger.Warnf("error stopping mongo process: %s", killErr)
		}

		remErr := os.RemoveAll(dbDir)
		if remErr != nil {
			logger.Warnf("error removing data directory: %s", remErr)
		}

		return nil, err
	case <-time.After(opts.StartupTimeout):
		killErr := cmd.Process.Kill()
		if killErr != nil {
			logger.Warnf("error stopping mongo process: %s", killErr)
		}

		remErr := os.RemoveAll(dbDir)
		if remErr != nil {
			logger.Warnf("error removing data directory: %s", remErr)
		}

		return nil, fmt.Errorf("timed out waiting for mongod to start")
	}

	logger.Debugf("mongod started up and reported a port number after %s", time.Since(startupTime).String())

	// ---------- START OF REPLICA CODE ----------
	if opts.ShouldUseReplica {
		ctx := context.Background()
		connectionURL := fmt.Sprintf(mongoConnectionTemplate, opts.Port)
		client, err := mongo.Connect(ctx, options.Client().ApplyURI(connectionURL))
		if err != nil {
			logger.Warnf("error while connect to localhost database: %w", err)
			return nil, err
		}

		if err := client.Ping(ctx, nil); err != nil {
			logger.Warnf("error while ping to localhost database: %w", err)
			return nil, err
		}

		var result bson.M
		err = client.Database("admin").RunCommand(ctx, bson.D{{Key: "replSetInitiate", Value: nil}}).Decode(&result)
		if err != nil {
			logger.Warnf("error while init replica set: %w", err)
			return nil, err
		}

		if err := client.Disconnect(ctx); err != nil {
			logger.Warnf("error while disconnect from localhost database: %w", err)
			return nil, err
		}

		logger.Debugf("Started mongo replica")
	}
	// ---------- END OF REPLICA CODE ----------

	// Return a Memongo server
	return &Server{
		cmd:        cmd,
		watcherCmd: watcherCmd,
		dbDir:      dbDir,
		logger:     logger,
		port:       port,
	}, nil
}

// Port returns the port the server is listening on.
func (s *Server) Port() int {
	return s.port
}

// URI returns a mongodb:// URI to connect to
func (s *Server) URI() string {
	return fmt.Sprintf("mongodb://localhost:%d", s.port)
}

// URIWithRandomDB returns a mongodb:// URI to connect to, with
// a random database name (e.g. mongodb://localhost:1234/somerandomname)
func (s *Server) URIWithRandomDB() string {
	return fmt.Sprintf("mongodb://localhost:%d/%s", s.port, RandomDatabase())
}

// Stop kills the mongo server
func (s *Server) Stop() {
	err := s.cmd.Process.Kill()
	if err != nil {
		s.logger.Warnf("error stopping mongod process: %s", err)
		return
	}

	err = s.watcherCmd.Process.Kill()
	if err != nil {
		s.logger.Warnf("error stopping watcher process: %s", err)
		return
	}

	err = os.RemoveAll(s.dbDir)
	if err != nil {
		s.logger.Warnf("error removing data directory: %s", err)
		return
	}
}

// Cribbed from https://github.com/nodkz/mongodb-memory-server/blob/master/packages/mongodb-memory-server-core/src/util/MongoInstance.ts#L206
var (
	reReady                 = regexp.MustCompile(`waiting for connections.*port\D*(\d+)`)
	reAlreadyInUse          = regexp.MustCompile("addr already in use")
	reAlreadyRunning        = regexp.MustCompile("mongod already running")
	rePermissionDenied      = regexp.MustCompile("mongod permission denied")
	reDataDirectoryNotFound = regexp.MustCompile("data directory .*? not found")
	reShuttingDown          = regexp.MustCompile("shutting down with code")
)

// The stdout handler relays lines from mongod's stout to our logger, and also
// watches during startup for error or success messages.
//
// It returns two channels: an error channel and a port channel. Only one
// message will be sent to one of these two channels. A port number will
// be sent to the port channel if the server start up correctly, and an
// error will be send to the error channel if the server does not start up
// correctly.
func stdoutHandler(log *memongolog.Logger) (io.Writer, <-chan error, <-chan int) {
	errChan := make(chan error)
	portChan := make(chan int)

	reader, writer := io.Pipe()

	go func() {
		scanner := bufio.NewScanner(reader)
		haveSentMessage := false

		for scanner.Scan() {
			line := scanner.Text()

			log.Debugf("[Mongod stdout] %s", line)

			if !haveSentMessage {
				downcaseLine := strings.ToLower(line)

				if match := reReady.FindStringSubmatch(downcaseLine); match != nil {
					port, err := strconv.Atoi(match[1])
					if err != nil {
						errChan <- fmt.Errorf("could not parse port from mongod log line: %s", downcaseLine)
					} else {
						portChan <- port
					}
					haveSentMessage = true
				} else if reAlreadyInUse.MatchString(downcaseLine) {
					errChan <- fmt.Errorf("mongod startup failed, address in use")
					haveSentMessage = true
				} else if reAlreadyRunning.MatchString(downcaseLine) {
					errChan <- fmt.Errorf("mongod startup failed, already running")
					haveSentMessage = true
				} else if rePermissionDenied.MatchString(downcaseLine) {
					errChan <- fmt.Errorf("mongod startup failed, permission denied")
					haveSentMessage = true
				} else if reDataDirectoryNotFound.MatchString(downcaseLine) {
					errChan <- fmt.Errorf("mongod startup failed, data directory not found")
					haveSentMessage = true
				} else if reShuttingDown.MatchString(downcaseLine) {
					errChan <- fmt.Errorf("mongod startup failed, server shut down")
					haveSentMessage = true
				}
			}
		}

		if err := scanner.Err(); err != nil {
			log.Warnf("reading mongod stdin failed: %s", err)
		}

		if !haveSentMessage {
			errChan <- fmt.Errorf("mongod exited before startup completed")
		}
	}()

	return writer, errChan, portChan
}

// The stderr handler just relays messages from stderr to our logger
func stderrHandler(log *memongolog.Logger) io.Writer {
	reader, writer := io.Pipe()

	go func() {
		scanner := bufio.NewScanner(reader)

		for scanner.Scan() {
			log.Debugf("[Mongod stderr] %s", scanner.Text())
		}

		if err := scanner.Err(); err != nil {
			log.Warnf("reading mongod stdin failed: %s", err)
		}
	}()

	return writer
}
