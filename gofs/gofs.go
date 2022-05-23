package gofs

import (
	"archive/zip"
	"bytes"
	"fmt"
	"github.com/ahdekkers/go-zipdir/zipdir"
	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-hclog"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
)

type logWriter struct {
	file *os.File
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	n, err = os.Stdout.Write(p)
	if err != nil {
		return
	}

	n, err = w.file.Write(p)
	return
}

type Opts struct {
	Addr     string
	Port     int
	RootDir  string
	LogLevel string
	LogFile  string
}

type Server struct {
	addr    string
	rootDir string
	logger  hclog.Logger
}

/*
Create a file server instance. This call will block until an error is thrown or interrupted
*/
func Create(opts Opts) error {
	logger, err := createLogWriter(opts.LogLevel, opts.LogFile)
	if err != nil {
		return fmt.Errorf("failed to create logger: %v", err)
	}

	err = checkIsDir(opts.RootDir)
	if err != nil {
		return fmt.Errorf("failed to read root dir: %v", err)
	}

	server := &Server{
		addr:    fmt.Sprintf("%s:%d", opts.Addr, opts.Port),
		rootDir: opts.RootDir,
		logger:  logger,
	}
	err = server.listenForRequests()
	if err != nil {

	}
	return err
}

func (s *Server) listenForRequests() error {
	router := gin.Default()

	router.Handle("GET", "/*addr", s.getFile)
	router.Handle("POST", "/*addr", s.uploadFile)

	return router.Run(s.addr)
}

func (s *Server) getFile(ctx *gin.Context) {
	fileAddr := ctx.Param("addr")
	path := filepath.Join(s.rootDir, fileAddr)
	s.logger.Debug("Received get file request", "addr", fileAddr, "fullPath", path)

	inf, err := os.Stat(path)
	if err != nil {
		s.logger.Warn("Failed to read file data", "error", err, "path", path)
		ctx.String(http.StatusBadRequest, "Failed to read file at '%s': %v", path, err)
		return
	}

	if inf.IsDir() {
		data, err := zipdir.ZipToBytes(path)
		if err != nil {
			s.logger.Warn("Failed to zip dir", "error", err, "path", path)
			ctx.String(http.StatusBadRequest, "Failed to zip dir '%s': %v", path, err)
			return
		}

		s.logger.Info("Successfully returned directory as zip", "path", path)
		ctx.Data(http.StatusOK, "application/zip", data)
	} else {
		fileData, err := os.ReadFile(path)
		if err != nil {
			ctx.String(http.StatusBadRequest, "Failed to read file at '%s': %v", path, err)
			return
		}

		s.logger.Info("Successfully returned file as raw data", "path", path)
		ctx.Data(http.StatusOK, "raw", fileData)
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

	reqData, err := ioutil.ReadAll(ctx.Request.Body)
	if err != nil {
		s.logger.Warn("Failed to read upload file request data", "error", err)
		ctx.String(http.StatusBadRequest, "Failed to read request data: %v", err)
		return
	}

	if contentType == "application/zip" {
		err = unzipToDir(path, reqData)
		if err != nil {
			s.logger.Warn("Failed to unzip upload file request data", "error", err)
			ctx.String(http.StatusBadRequest, err.Error())
		}
	} else {
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

func unzipToDir(dir string, zipData []byte) error {
	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return fmt.Errorf("Failed to read zip data: %v", err)
	}

	var files []File
	for _, file := range zipReader.File {
		fileReader, err := file.Open()
		if err != nil {
			return fmt.Errorf("Failed to unzip file '%s': %v", file.Name, err)
		}

		data, err := ioutil.ReadAll(fileReader)
		fileReader.Close()
		if err != nil {
			return fmt.Errorf("Failed to read file data for file '%s': %v", file.Name, err)
		}
		files = append(files, File{
			Name: file.Name,
			Data: data,
		})
	}

	for _, file := range files {
		path := filepath.Join(dir, file.Name)
		outFile, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_RDWR, os.ModePerm)
		if err != nil {
			if outFile != nil {
				outFile.Close()
			}
			return fmt.Errorf("Failed to open or create file '%s': %v", path, err)
		}

		_, err = outFile.Write(file.Data)
		if err != nil {
			return fmt.Errorf("Failed to write data to file '%s': %v", path, err)
		}
	}
	return nil
}

func createLogWriter(level, logFile string) (hclog.Logger, error) {
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_RDWR, os.ModePerm)
	if err != nil {
		return nil, err
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
	_, err := os.ReadDir(dir)
	return err
}
