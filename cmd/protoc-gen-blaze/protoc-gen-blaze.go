package main

import (
	"flag"
	"fmt"

	"os"

	"code.cestus.io/blaze/pkg/generator"
	"github.com/go-logr/logr"
	"github.com/magicmoose/zapr"
	"go.uber.org/zap"
)

var (
	// Version is the version of the application
	Version string = "v0.1.0"
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
	versionFlag := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *versionFlag {
		fmt.Println(Version)
		os.Exit(0)
	}

	var log logr.Logger

	zapLog, err := NewZapDevelopmentConfig().Build()
	if err != nil {
		panic(fmt.Sprintf("Cannot init logger (%v)?", err))
	}
	log = zapr.NewLogger(zapLog).WithValues("version", Version).WithName("test")
	g := newGenerator(log)
	generator.Generate(g)
}
