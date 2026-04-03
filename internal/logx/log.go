package logx

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

const loggerFlags = log.LstdFlags | log.Lshortfile

func New(quiet bool, logDir string, name string) (*log.Logger, io.Closer, error) {
	if quiet {
		return log.New(io.Discard, "", loggerFlags), io.NopCloser(nil), nil
	}

	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, nil, err
	}

	path := filepath.Join(logDir, name+".log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, err
	}

	writer := io.MultiWriter(os.Stdout, file)
	return log.New(writer, "", loggerFlags), file, nil
}
