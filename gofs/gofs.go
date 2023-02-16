package gofs

import (
	"context"
	"errors"
	"fmt"
	"github.com/ahdekkers/go-zipdir/zipdir"
	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-hclog"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type logWriter struct {
	file *os.File
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	n, err = os.Stdout.Write(p)
	if err != nil {
		return
	}

	if w.file != nil {
		n, err = w.file.Write(p)
	}
	return
}

type Opts struct {
	Addr          string
	Port          int
	RootDir       string
	LogLevel      string
	LogFile       string
	NoCache       bool
	NoDirectories bool
}

type Server struct {
	srv     *http.Server
	rootDir string
	logger  hclog.Logger
	stopCh  chan int
	cache   map[string][]byte
	noCache bool
	noDirs  bool
}

/*
Create a file server instance.
*/
func Create(opts Opts) (*Server, error) {
	logger, err := createLogWriter(opts.LogLevel, opts.LogFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %v", err)
	}

	err = checkIsDir(opts.RootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read root dir: %v", err)
	}

	server := &Server{
		rootDir: opts.RootDir,
		logger:  logger,
		stopCh:  make(chan int),
		cache:   make(map[string][]byte),
		noCache: opts.NoCache,
		noDirs:  opts.NoDirectories,
	}

	router := gin.Default()
	router.Handle("GET", "/entries/*addr", server.getEntries)
	router.Handle("GET", "/content/*addr", server.getFile)
	router.Handle("POST", "/content/*addr", server.uploadFile)

	server.srv = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", opts.Addr, opts.Port),
		Handler: router,
	}
	return server, nil
}

/*
Start listening for requests. This call is non-blocking
*/
func (s *Server) Start() {
	go func() {
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Error while starting http file server: %v", err)
		}
	}()
}

func (s *Server) Run() error {
	return s.srv.ListenAndServe()
}

func (s *Server) Stop() error {
	if err := s.srv.Shutdown(context.Background()); err != nil {
		return err
	}
	return nil
}

func (s *Server) GetAddr() string {
	return s.srv.Addr
}

func (s *Server) getFile(ctx *gin.Context) {
	fileAddr := ctx.Param("addr")
	path := filepath.Join(s.rootDir, fileAddr)
	s.logger.Debug("Received get file request", "addr", fileAddr, "fullPath", path)

	if !s.noCache {
		data, found := s.cache[path]
		if found {
			s.logger.Info("Retrieved file data from cache")
			ctx.Data(http.StatusOK, "application/zip", data)
			return
		}
	}

	inf, err := os.Stat(path)
	if err != nil {
		s.logger.Warn("Failed to read file data", "error", err, "path", path)
		ctx.String(http.StatusBadRequest, "Failed to read file at '%s': %v", path, err)
		return
	}

	var data []byte
	if inf.IsDir() {
		if s.noDirs {
			ctx.String(http.StatusBadRequest, "Path '%s' is a directory and noDirs flag is true", path)
			s.logger.Warn("Path '%s' is a directory and noDirs flag is true", "path", path)
			return
		}

		data, err = zipdir.ZipToBytes(path)
		if err != nil {
			s.logger.Warn("Failed to zip dir", "error", err, "path", path)
			ctx.String(http.StatusBadRequest, "Failed to zip dir '%s': %v", path, err)
			return
		}

		s.logger.Info("Successfully returned directory as zip", "path", path)
		ctx.Data(http.StatusOK, "application/zip", data)
	} else {
		data, err = os.ReadFile(path)
		if err != nil {
			ctx.String(http.StatusBadRequest, "Failed to read file at '%s': %v", path, err)
			return
		}

		s.logger.Info("Successfully returned file as raw data", "path", path)
		ctx.Data(http.StatusOK, "raw", data)
	}
	if !s.noCache {
		s.cache[path] = data
	}
}

type File struct {
	Name string
	Data []byte
}

func (s *Server) uploadFile(ctx *gin.Context) {
	contentType := ctx.Request.Header.Get("content-type")
	destAddr := ctx.Param("addr")
	path := filepath.Join(s.rootDir, destAddr)
	s.logger.Debug("Received upload file request",
		"content-type", contentType, "destAddr", destAddr, "fullPath", path)

	reqData, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		s.logger.Warn("Failed to read upload file request data", "error", err)
		ctx.String(http.StatusBadRequest, "Failed to read request data: %v", err)
		return
	}

	if contentType == "application/zip" {
		if s.noDirs {
			ctx.String(http.StatusBadRequest, "Content type is application/zip but noDirs flag is set to true")
			s.logger.Warn("Content type is application/zip but noDirs flag is set to true")
			return
		}

		err = zipdir.UnzipToDir(path, reqData)
		if err != nil {
			s.logger.Warn("Failed to unzip upload file request data", "error", err)
			ctx.String(http.StatusBadRequest, err.Error())
		}
	} else {
		dir := path[:strings.LastIndex(path, "/")]
		err = os.MkdirAll(dir, os.ModePerm)
		if err != nil {
			s.logger.Warn("Failed to create dirs", "error", err, "dirs", dir)
			ctx.String(http.StatusBadRequest, "Failed to make dirs '%s': %v", dir, err)
			return
		}

		file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.ModePerm)
		if err != nil {
			if file != nil {
				file.Close()
			}

			s.logger.Warn("Failed to create/truncate file during upload file request", "file", path, "error", err)
			ctx.String(http.StatusBadRequest, "Failed to open file '%s': %v", path, err)
			return
		}

		_, err = file.Write(reqData)
		file.Close()
		if err != nil {
			s.logger.Warn("Failed to write file data during upload file request", "error", err, "file", path)
			ctx.String(http.StatusBadRequest, "Failed to write data to file '%s': %v", path, err)
			return
		}
	}

	s.logger.Info("File data successfully uploaded", "path", path)
	ctx.String(http.StatusOK, "Successfully wrote data to '%s'", path)
}

func (s *Server) getEntries(ctx *gin.Context) {
	relativePath := ctx.Param("addr")
	path := filepath.Join(s.rootDir, relativePath)
	entries, err := os.ReadDir(path)
	if err != nil {
		s.logger.Warn("Failed to get entries in directory", "error", err, "dirPath", path)
		ctx.String(http.StatusBadRequest, "Failed to read directory '%s': %v", path, err)
		return
	}

	var entryNames []string
	for _, entry := range entries {
		entryNames = append(entryNames, entry.Name())
	}

	respString := strings.Join(entryNames, ",")
	s.logger.Info("Successfully processed entries request", "entries", respString)
	ctx.String(http.StatusOK, respString)
}

func createLogWriter(level, logFile string) (hclog.Logger, error) {
	var file *os.File
	if logFile != "" {
		dir, _ := filepath.Split(logFile)
		err := os.MkdirAll(dir, os.ModePerm)
		if err != nil {
			return nil, fmt.Errorf("failed to make dirs '%s': %v", dir, err)
		}

		file, err = os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_RDWR, os.ModePerm)
		if err != nil {
			return nil, fmt.Errorf("failed to open file '%s': %v", logFile, err)
		}
	}

	return hclog.New(&hclog.LoggerOptions{
		Name:  "gofs",
		Level: hclog.LevelFromString(level),
		Output: &logWriter{
			file: file,
		},
	}), nil
}

func checkIsDir(dir string) error {
	inf, err := os.Stat(dir)
	if errors.Is(err, os.ErrNotExist) {
		err = os.MkdirAll(dir, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create dir '%s': %v", dir, err)
		}
	} else if err == nil {
		if !inf.IsDir() {
			return fmt.Errorf("rootDir '%s' is not directory", dir)
		}
	}
	return err
}
