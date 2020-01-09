package main

import (
	"fmt"

	"os"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
)

var (
	// Version is the version of the application
	Version string
	// BuildTime is the time the application was build
	BuildTime string
)

// NewZapDevelopmentConfig is a development logger config
func NewZapDevelopmentConfig() zap.Config {
	return zap.Config{
		Level:            zap.NewAtomicLevelAt(-3),
		Development:      false,
		Encoding:         "json",
		EncoderConfig:    zap.NewProductionEncoderConfig(),
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}
}

func main() {
	var log logr.Logger

	zapLog, err := NewZapDevelopmentConfig().Build()
	if err != nil {
		panic(fmt.Sprintf("Cannot init logger (%v)?", err))
	}
	log = zapr.NewLogger(zapLog).WithValues("version", Version).WithName("test")
	log.V(0).Info("Initializing", "pid", os.Getpid())
}
