package kineticaotelexporter // import

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"gopkg.in/yaml.v3"
)

const (
	// The value of "type" key in configuration.
	typeStr = "kinetica"
	// The stability level of the exporter.
	stability = component.StabilityLevelAlpha

	maxSize              = 10
	maxBackups           = 5
	maxAge               = 10
	defaultLogConfigFile = "config_log_zap.yaml"
)

var (
	kineticaLogger      *zap.Logger
	logConfigFileExists bool
)

// Config defines configuration for the Kinetica exporter.
type Config struct {
	Host               string              `mapstructure:"host"`
	Schema             string              `mapstructure:"schema"`
	Username           string              `mapstructure:"username"`
	Password           configopaque.String `mapstructure:"password"`
	BypassSslCertCheck bool                `mapstructure:"bypasssslcertcheck"`
	LogConfigFile      string              `mapstructure:"logconfigfile"`
}

// Validate the config
//
//	@receiver cfg
//	@return error
func (cfg *Config) Validate() error {
	kineticaHost, err := url.ParseRequestURI(cfg.Host)
	if err != nil {
		return err
	}
	if kineticaHost.Scheme != "http" && kineticaHost.Scheme != "https" {
		return errors.New("Protocol must be either `http` or `https`")
	}

	if !fileExists(cfg.LogConfigFile) {
		fmt.Println("WARNING : LOG config file ", cfg.LogConfigFile, " does not exist; will work with default logger ...")
		logConfigFileExists = false
	} else {
		logConfigFileExists = true
	}

	fmt.Println("Password = ", string(cfg.Password))

	return nil
}

func parseNumber(s string, fallback int) int {
	v, err := strconv.Atoi(s)
	if err == nil {
		return v
	}
	return fallback
}

// createLogger - creates a Logger using user defined configuration from a file.
// If the file name is not found or there is some error it will create a default
// logger using the default logger config file named 'config_log_zap.yaml'.
//
//	@receiver cfg
//	@return *zap.Logger
func (cfg *Config) createLogger() *zap.Logger {

	if kineticaLogger != nil {
		return kineticaLogger
	}

	var logConfig zap.Config

	yamlFile, _ := os.ReadFile(cfg.LogConfigFile)
	if err := yaml.Unmarshal(yamlFile, &logConfig); err != nil {
		fmt.Println("Error in loading log config file : ", cfg.LogConfigFile)
		fmt.Println(err)
		fmt.Println("Creating default logger ...")
		kineticaLogger = cfg.createDefaultLogger()
		return kineticaLogger
	}
	fmt.Println("Log Config from YAML file : ", logConfig)

	var (
		stdout      zapcore.WriteSyncer
		file        zapcore.WriteSyncer
		logFilePath *url.URL
	)

	for _, path := range logConfig.OutputPaths {

		if path == "stdout" || path == "stderror" {
			stdout = zapcore.AddSync(os.Stdout)
			fmt.Println("Created stdout syncer ...")

		} else if strings.HasPrefix(path, "lumberjack://") {
			var err error
			logFilePath, err = url.Parse(path)
			fmt.Println("LogFilePath : ", logFilePath)

			if err == nil {
				filename := strings.TrimLeft(logFilePath.Path, "/")
				fmt.Println("LogFileName : ", filename)

				if filename != "" {
					q := logFilePath.Query()
					l := &lumberjack.Logger{
						Filename:   filename,
						MaxSize:    parseNumber(q.Get("maxSize"), maxSize),
						MaxAge:     parseNumber(q.Get("maxAge"), maxAge),
						MaxBackups: parseNumber(q.Get("maxBackups"), maxBackups),
						LocalTime:  false,
						Compress:   false,
					}

					file = zapcore.AddSync(l)
					fmt.Println("Created file syncer ...")
				}
			}
		} else {
			// Unknown output format
			fmt.Println("Invalid output path specified in config ...")
		}
	}

	if stdout == nil && file == nil {
		fmt.Println("Both stdout and file not available; creating default logger ...")
		return cfg.createDefaultLogger()
	}

	level := zap.NewAtomicLevelAt(logConfig.Level.Level())

	consoleEncoder := zapcore.NewConsoleEncoder(logConfig.EncoderConfig)
	fileEncoder := zapcore.NewJSONEncoder(logConfig.EncoderConfig)

	core := zapcore.NewTee(
		zapcore.NewCore(consoleEncoder, stdout, level),
		zapcore.NewCore(fileEncoder, file, level),
	)

	kineticaLogger = zap.New(core)
	return kineticaLogger
}

// createDefaultLogger
//
//	@receiver cfg
//	@return *zap.Logger
func (cfg *Config) createDefaultLogger() *zap.Logger {
	if kineticaLogger != nil {
		return kineticaLogger
	}

	var logConfig zap.Config

	yamlFile, _ := os.ReadFile(defaultLogConfigFile)
	if err := yaml.Unmarshal(yamlFile, &logConfig); err != nil {
		fmt.Println("Error in loading log config file : ", cfg.LogConfigFile)
		fmt.Println(err)
		fmt.Println("Creating default logger ...")
		kineticaLogger = cfg.createDefaultLogger()
		return kineticaLogger
	}
	fmt.Println("Default Log Config from YAML file : ", logConfig)

	var (
		stdout      zapcore.WriteSyncer
		file        zapcore.WriteSyncer
		logFilePath *url.URL
	)

	for _, path := range logConfig.OutputPaths {

		if path == "stdout" || path == "stderror" {
			stdout = zapcore.AddSync(os.Stdout)
			fmt.Println("Created stdout syncer ...")

		} else if strings.HasPrefix(path, "lumberjack://") {
			var err error
			logFilePath, err = url.Parse(path)
			fmt.Println("LogFilePath : ", logFilePath)

			if err == nil {
				filename := strings.TrimLeft(logFilePath.Path, "/")
				fmt.Println("LogFileName : ", filename)

				if filename != "" {
					q := logFilePath.Query()
					l := &lumberjack.Logger{
						Filename:   filename,
						MaxSize:    parseNumber(q.Get("maxSize"), maxSize),
						MaxAge:     parseNumber(q.Get("maxAge"), maxAge),
						MaxBackups: parseNumber(q.Get("maxBackups"), maxBackups),
						LocalTime:  false,
						Compress:   false,
					}

					file = zapcore.AddSync(l)
					fmt.Println("Created file syncer ...")
				}
			}
		} else {
			// Unknown output format
			fmt.Println("Invalid output path specified in config ...")
		}
	}

	if stdout == nil && file == nil {
		fmt.Println("Both stdout and file not available; creating default logger ...")
		return cfg.createDefaultLogger()
	}

	level := zap.NewAtomicLevelAt(logConfig.Level.Level())

	consoleEncoder := zapcore.NewConsoleEncoder(logConfig.EncoderConfig)
	fileEncoder := zapcore.NewJSONEncoder(logConfig.EncoderConfig)

	core := zapcore.NewTee(
		zapcore.NewCore(consoleEncoder, stdout, level),
		zapcore.NewCore(fileEncoder, file, level),
	)

	kineticaLogger = zap.New(core)
	return kineticaLogger
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

var _ component.Config = (*Config)(nil)
