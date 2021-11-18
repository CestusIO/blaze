package main

import (
	"flag"
	"fmt"

	"os"

	gengo "code.cestus.io/blaze/cmd/protoc-gen-blaze/internal_gengo"
	"github.com/go-logr/logr"
	"github.com/magicmoose/zapr"
	"go.uber.org/zap"
	"google.golang.org/protobuf/compiler/protogen"
)

var (
	// Version is the version of the application
	Version string = "v0.5.0"
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

	var (
		log   logr.Logger
		flags flag.FlagSet
	)

	zapLog, err := NewZapDevelopmentConfig().Build()
	if err != nil {
		panic(fmt.Sprintf("Cannot init logger (%v)?", err))
	}
	log = zapr.NewLogger(zapLog).WithValues("version", Version).WithName("test")
	blaze := gengo.NewGenerator(log, Version)
	protogen.Options{
		ParamFunc: flags.Set,
	}.Run(func(gen *protogen.Plugin) error {
		gen.SupportedFeatures = gengo.SupportedFeatures
		for _, f := range gen.Files {
			if f.Generate {
				blaze.GenerateFile(gen, f)
				blaze.GenerateSampleFile(gen, f)
			}
		}
		return nil
	})
	//g := newGenerator(log)
	//generator.Generate(g)

}
