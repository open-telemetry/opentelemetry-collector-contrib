package pytransform

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/daidokoro/go-embed-python/python"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/pytransformprocessor/internal/metadata"
	"go.uber.org/zap"
)

var (
	pylogserver         *http.Server
	embeddedPython      *python.EmbeddedPython
	initPythonEmbedOnce sync.Once
	startOnce           sync.Once
	closeOnce           sync.Once
)

// The log server receives print statements from the python script
// and prints them using the zap logger to stdout

func startLogServer(logger *zap.Logger) {
	startOnce.Do(func() {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		port := r.Intn(60000-10000) + 10000
		mux := http.NewServeMux()

		// handle log events from python script
		mux.Handle("/log", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer r.Body.Close()
			var m map[string]string

			if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
				logger.Error("failed reading python script log body", zap.Error(err))
			}

			logger.Info(m["message"],
				zap.String("func", "python.print()"),
				zap.String("source", fmt.Sprintf("pytransform/%s", m["pipeline"])))
		}))

		pylogserver = &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: mux,
		}

		go func() {
			logger.Info("starting python logging http server", zap.String("addr", pylogserver.Addr))
			if err := pylogserver.ListenAndServe(); err != http.ErrServerClosed {
				logger.Error("error starting http server", zap.Error(err))
				return
			}

			logger.Info("http server closed")
		}()
	})
	return
}

func closeLogServer() error {
	var err error
	closeOnce.Do(func() {
		if pylogserver != nil {
			err = pylogserver.Close()
		}
	})
	return err
}

func getEmbeddedPython() (*python.EmbeddedPython, error) {
	var err error
	initPythonEmbedOnce.Do(func() {
		embeddedPython, err = python.NewEmbeddedPython(metadata.Type)
	})
	return embeddedPython, err
}

// runPythonCode - runs a python script and returns the stdout and stderr in bytes
func runPythonCode(code, pyinput, pipeline string, ep *python.EmbeddedPython,
	logger *zap.Logger) ([]byte, error) {

	result := make(chan []byte)
	errchan := make(chan error)

	code = fmt.Sprintf("%s\n%s", prefixScript, code)
	cmd := ep.PythonCmd("-c", code)

	go func() {
		var buf bytes.Buffer

		// add script prefix
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		cmd.Env = append(
			os.Environ(),
			fmt.Sprintf("INPUT=%s", pyinput),
			fmt.Sprintf("PIPELINE=%s", pipeline),
			fmt.Sprintf("LOG_URL=http://localhost%s/log", pylogserver.Addr))

		if err := cmd.Run(); err != nil {
			err = fmt.Errorf("error running python script: %s - %v", buf.String(), err)
			logger.Error(err.Error())
			errchan <- err
		}

		result <- buf.Bytes()
	}()

	select {
	case err := <-errchan:
		return nil, err
	case b := <-result:
		return b, nil
	case <-time.After(1500 * time.Millisecond):
		logger.Error("timeout running python script")
		logger.Debug("killing python process", zap.Int("pid", cmd.Process.Pid))
		process, err := os.FindProcess(cmd.Process.Pid)
		if err != nil {
			return nil, fmt.Errorf("error finding python process after timeout: %v", err)
		}

		if err = process.Kill(); err != nil {
			return nil, fmt.Errorf("error while cancelling python process after timeout: %v", err)
		}

		return nil, errors.New("timeout running python script")
	}
}
