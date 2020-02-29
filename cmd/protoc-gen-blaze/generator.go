package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"fmt"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"log"
	"os"
	"path"
	"strconv"
	"strings"

	"code.cestus.io/blaze/internal/generation/stringutils"
	"code.cestus.io/blaze/internal/generation/typemap"
	"code.cestus.io/blaze/pkg/generator"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/pkg/errors"

	"github.com/go-logr/logr"
	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
)

type blaze struct {
	filesHandled   int
	currentPackage string // Go name of current package we're working on

	reg *typemap.Registry

	// Map to record whether we've built each package
	pkgs          map[string]string
	pkgNamesInUse map[string]bool

	importPrefix string            // String to prefix to imported package file names.
	importMap    map[string]string // Mapping from .proto file name to import path.

	// Package output:
	sourceRelativePaths bool // instruction on where to write output files

	// Package naming:
	genPkgName          string // Name of the package that we're generating
	fileToGoPackageName map[*descriptor.FileDescriptorProto]string

	// List of files that were inputs to the generator. We need to hold this in
	// the struct so we can write a header for the file that lists its inputs.
	genFiles []*descriptor.FileDescriptorProto

	// Output buffer that holds the bytes we want to write out for a single file.
	// Gets reset after working on a file.
	output *bytes.Buffer

	log logr.Logger

	// Md5 of the filenames used to generatge.
	// This allows for unique naming of descriptors
	filesHash string
}

func newGenerator(log logr.Logger) *blaze {
	b := &blaze{
		pkgs:                make(map[string]string),
		pkgNamesInUse:       make(map[string]bool),
		importMap:           make(map[string]string),
		fileToGoPackageName: make(map[*descriptor.FileDescriptorProto]string),
		output:              bytes.NewBuffer(nil),
		log:                 log,
	}

	return b
}

func (s *blaze) Generate(in *plugin.CodeGeneratorRequest) *plugin.CodeGeneratorResponse {
	params, err := parseCommandLineParams(in.GetParameter())
	if err != nil {
		s.log.Error(err, "could not parse parameters passed to --blaze_out")
	}
	s.importPrefix = params.importPrefix
	s.importMap = params.importMap

	s.genFiles = generator.FilesToGenerate(in)

	// build unique name of file descriptor
	h := md5.New()
	for _, d := range s.genFiles {
		io.WriteString(h, d.GetName())
	}
	s.filesHash = fmt.Sprintf("%x", h.Sum(nil))

	s.sourceRelativePaths = params.paths == "source_relative"

	// Collect information on types.
	s.reg = typemap.New(in.ProtoFile)

	// Register names of packages that we import.
	s.registerPackageName("bytes")
	s.registerPackageName("strings")
	s.registerPackageName("context")
	s.registerPackageName("http")
	s.registerPackageName("io")
	s.registerPackageName("ioutil")
	s.registerPackageName("json")
	s.registerPackageName("jsonpb")
	s.registerPackageName("proto")
	s.registerPackageName("strconv")
	s.registerPackageName("blaze")
	s.registerPackageName("fmt")
	s.registerPackageName("logr")
	s.registerPackageName("chi")
	s.registerPackageName("blazetrace")

	// Time to figure out package names of objects defined in protobuf. First,
	// we'll figure out the name for the package we're generating.
	genPkgName, err := deduceGenPkgName(s.genFiles)
	if err != nil {
		s.log.Error(err, "package name deduction failed")
	}
	s.genPkgName = genPkgName

	// Next, we need to pick names for all the files that are dependencies.
	for _, f := range in.ProtoFile {
		if fileDescSliceContains(s.genFiles, f) {
			// This is a file we are generating. It gets the shared package name.
			s.fileToGoPackageName[f] = s.genPkgName
		} else {
			// This is a dependency. Use its package name.
			name := f.GetPackage()
			if name == "" {
				name = stringutils.BaseName(f.GetName())
			}
			name = stringutils.CleanIdentifier(name)
			alias := s.registerPackageName(name)
			s.fileToGoPackageName[f] = alias
		}
	}
	// Showtime! Generate the response.
	resp := new(plugin.CodeGeneratorResponse)
	for _, f := range s.genFiles {
		respFile := s.generate(f)
		if respFile != nil {
			resp.File = append(resp.File, respFile)
		}
	}
	return resp
}

func (s *blaze) registerPackageName(name string) (alias string) {
	alias = name
	i := 1
	for s.pkgNamesInUse[alias] {
		alias = name + strconv.Itoa(i)
		i++
	}
	s.pkgNamesInUse[alias] = true
	s.pkgs[name] = alias
	return alias
}

// deduceGenPkgName figures out the go package name to use for generated code.
// Will try to use the explicit go_package setting in a file (if set, must be
// consistent in all files). If no files have go_package set, then use the
// protobuf package name (must be consistent in all files)
func deduceGenPkgName(genFiles []*descriptor.FileDescriptorProto) (string, error) {
	var genPkgName string
	for _, f := range genFiles {
		name, explicit := goPackageName(f)
		if explicit {
			name = stringutils.CleanIdentifier(name)
			if genPkgName != "" && genPkgName != name {
				// Make sure they're all set consistently.
				return "", errors.Errorf("files have conflicting go_package settings, must be the same: %q and %q", genPkgName, name)
			}
			genPkgName = name
		}
	}
	if genPkgName != "" {
		return genPkgName, nil
	}

	// If there is no explicit setting, then check the implicit package name
	// (derived from the protobuf package name) of the files and make sure it's
	// consistent.
	for _, f := range genFiles {
		name, _ := goPackageName(f)
		name = stringutils.CleanIdentifier(name)
		if genPkgName != "" && genPkgName != name {
			return "", errors.Errorf("files have conflicting package names, must be the same or overridden with go_package: %q and %q", genPkgName, name)
		}
		genPkgName = name
	}

	// All the files have the same name, so we're good.
	return genPkgName, nil
}

func (s *blaze) generate(file *descriptor.FileDescriptorProto) *plugin.CodeGeneratorResponse_File {
	resp := new(plugin.CodeGeneratorResponse_File)
	if len(file.Service) == 0 {
		return nil
	}

	s.generateFileHeader(file)

	s.generateImports(file)
	if s.filesHandled == 0 {
		s.generateUtilImports()
	}

	// For each service, generate client stubs and server
	for i, service := range file.Service {
		s.generateService(file, service, i)
	}

	// Util functions only generated once per package
	if s.filesHandled == 0 {
		s.generateUtils()
	}

	s.generateFileDescriptor(file)

	resp.Name = proto.String(s.goFileName(file))
	resp.Content = proto.String(s.formattedOutput())
	s.output.Reset()

	s.filesHandled++
	return resp
}

// P forwards to g.gen.P, which prints output.
func (s *blaze) P(args ...string) {
	for _, v := range args {
		s.output.WriteString(v)
	}
	s.output.WriteByte('\n')
}

func (s *blaze) goPackageName(file *descriptor.FileDescriptorProto) string {
	return s.fileToGoPackageName[file]
}
func (s *blaze) generateFileHeader(file *descriptor.FileDescriptorProto) {
	s.P("// Code generated by protoc-gen-blaze ", Version, ", DO NOT EDIT.")
	s.P("// source: ", file.GetName())
	s.P()
	if s.filesHandled == 0 {
		s.P("/*")
		s.P("Package ", s.genPkgName, " is a generated blaze stub package.")
		s.P("This code was generated with code.cestus.io/blaze/cmd/protoc-gen-blaze ", Version, ".")
		s.P()
		comment, err := s.reg.FileComments(file)
		if err == nil && comment.Leading != "" {
			for _, line := range strings.Split(comment.Leading, "\n") {
				line = strings.TrimPrefix(line, " ")
				// ensure we don't escape from the block comment
				line = strings.Replace(line, "*/", "* /", -1)
				s.P(line)
			}
			s.P()
		}
		s.P("It is generated from these files:")
		for _, f := range s.genFiles {
			s.P("\t", f.GetName())
		}
		s.P("*/")
	}
	s.P(`package `, s.genPkgName)
	s.P()
}

func (s *blaze) generateImports(file *descriptor.FileDescriptorProto) {
	if len(file.Service) == 0 {
		return
	}

	allServicesEmpty := true
	for _, svc := range file.Service {
		if len(svc.Method) > 0 {
			allServicesEmpty = false
		}
	}

	s.P(`import `, s.pkgs["bytes"], ` "bytes"`)
	// some packages is only used in generated method code.
	if !allServicesEmpty {
		s.P(`import `, s.pkgs["strings"], ` "strings"`)
		s.P(`import `, s.pkgs["fmt"], ` "fmt"`)
		s.P(`import `, s.pkgs["strconv"], ` "strconv"`)
	}
	s.P(`import `, s.pkgs["context"], ` "context"`)
	s.P(`import `, s.pkgs["ioutil"], ` "io/ioutil"`)
	s.P(`import `, s.pkgs["http"], ` "net/http"`)
	s.P()
	s.P(`import `, s.pkgs["jsonpb"], ` "github.com/gogo/protobuf/jsonpb"`)
	s.P(`import `, s.pkgs["proto"], ` "github.com/gogo/protobuf/proto"`)
	s.P(`import `, s.pkgs["logr"], ` "github.com/go-logr/logr"`)
	s.P(`import `, s.pkgs["chi"], ` "github.com/go-chi/chi"`)
	s.P(`import `, s.pkgs["blaze"], ` "code.cestus.io/blaze"`)
	s.P(`import `, s.pkgs["blazetrace"], ` "code.cestus.io/blaze/pkg/blazetrace"`)
	s.P()

	// It's legal to import a message and use it as an input or output for a
	// method. Make sure to import the package of any such message. First, dedupe
	// them.
	deps := make(map[string]string) // Map of package name to quoted import path.
	ourImportPath := path.Dir(s.goFileName(file))
	for _, svc := range file.Service {
		for _, m := range svc.Method {
			defs := []*typemap.MessageDefinition{
				s.reg.MethodInputDefinition(m),
				s.reg.MethodOutputDefinition(m),
			}
			for _, def := range defs {
				// By default, import path is the dirname of the Go filename.
				importPath := path.Dir(s.goFileName(def.File))
				if importPath == ourImportPath {
					continue
				}

				importPathOpt, _ := parseGoPackageOption(def.File.GetOptions().GetGoPackage())
				if importPathOpt != "" {
					importPath = importPathOpt
				}

				if substitution, ok := s.importMap[def.File.GetName()]; ok {
					importPath = substitution
				}
				importPath = s.importPrefix + importPath
				pkg := s.goPackageName(def.File)
				deps[pkg] = strconv.Quote(importPath)

			}
		}
	}
	for pkg, importPath := range deps {
		s.P(`import `, pkg, ` `, importPath)
	}
	if len(deps) > 0 {
		s.P()
	}
}

func (s *blaze) generateUtilImports() {
}

// Generate utility functions used in Blaze code.
// These should be generated just once per package.
func (s *blaze) generateUtils() {
}

// Big header comments to makes it easier to visually parse a generated file.
func (s *blaze) sectionComment(sectionTitle string) {
	s.P()
	s.P(`// `, strings.Repeat("=", len(sectionTitle)))
	s.P(`// `, sectionTitle)
	s.P(`// `, strings.Repeat("=", len(sectionTitle)))
	s.P()
}

func (s *blaze) generateService(file *descriptor.FileDescriptorProto, service *descriptor.ServiceDescriptorProto, index int) {
	servName := serviceName(service)

	s.sectionComment(servName + ` Interface`)
	s.generateBlazeInterface(file, service)

	s.sectionComment(servName + ` Protobuf Client`)
	s.generateClient("Protobuf", file, service)

	s.sectionComment(servName + ` JSON Client`)
	s.generateClient("JSON", file, service)

	// Service
	s.sectionComment(servName + ` Service`)
	s.generateServer(file, service)
	s.generateImplementInterface(file, service)
}

func (s *blaze) generateBlazeInterface(file *descriptor.FileDescriptorProto, service *descriptor.ServiceDescriptorProto) {
	servName := serviceName(service)

	comments, err := s.reg.ServiceComments(file, service)
	if err == nil {
		s.printComments(comments)
	}
	s.P(`type `, servName, ` interface {`)
	for _, method := range service.Method {
		comments, err = s.reg.MethodComments(file, service, method)
		if err == nil {
			s.printComments(comments)
		}
		s.P(s.generateSignature(method))
		s.P()
	}
	s.P(`}`)
}

func (s *blaze) generateServer(file *descriptor.FileDescriptorProto, service *descriptor.ServiceDescriptorProto) {
	servName := serviceName(service)

	// Server implementation.
	servStruct := serviceStruct(service)
	s.P(`type `, servStruct, ` struct {`)
	s.P(`  `, servName)
	s.P(`  `, "log logr.Logger")
	s.P(`  `, "mux *chi.Mux")
	s.P(`  `, "mountPath string")
	s.P(`     serviceTracer `, s.pkgs["blazetrace"], `.ServiceTracer`)
	s.P(`     serviceOptions `, s.pkgs["blaze"], `.ServiceOptions`)
	s.P(`}`)
	s.P()

	// Constructor for service implementation
	s.P(`func New`, servName, `Service(svc `, servName, `, log logr.Logger, opts ...`, s.pkgs["blaze"], `.ServiceOption) `, s.pkgs["blaze"], `.Service {`)
	s.P(`	serviceOptions := `, s.pkgs["blaze"], `.ServiceOptions{`)
	s.P(`   Trace: blazetrace.NewServiceTracer(),`)
	s.P(`   }`)
	s.P(`	for _, o := range opts {`)
	s.P(`		o(&serviceOptions)`)
	s.P(`	}`)
	s.P(`	var r *chi.Mux`)
	s.P(`	if serviceOptions.Mux != nil {`)
	s.P(`		r = serviceOptions.Mux`)
	s.P(`	} else {`)
	s.P(`		r = chi.NewRouter()`)
	s.P(`	}`)
	s.P(``)
	s.P(`   service := `, servStruct, `{`)
	s.P(`       log:           log,`)
	s.P(`       mux:           r,`)
	s.P(`		serviceOptions: serviceOptions,`)
	s.P(`		mountPath:     `, servName, `PathPrefix,`)
	s.P(`   	serviceTracer:  serviceOptions.Trace,`)
	s.P(`       `, servName, `: svc,`)
	s.P(`}`)
	for _, method := range service.Method {
		methName := "serve" + stringutils.CamelCase(method.GetName())
		s.P(`r.Post("/`, method.GetName(), `", service.`, methName, `)`)
	}
	s.P(`return &service`)
	s.P(`}`)
	s.P()
	// Getting the mount path as package/version
	ss := strings.ReplaceAll(s.genPkgName, "_", "/")
	s.P(`const `, servName, `PathPrefix = "/`, ss, `"`)

	s.P(`// Methods.`)
	for _, method := range service.Method {
		s.generateServerMethod(service, method)
	}

}

func (s *blaze) generateServerMethod(service *descriptor.ServiceDescriptorProto, method *descriptor.MethodDescriptorProto) {
	methName := stringutils.CamelCase(method.GetName())
	servStruct := serviceStruct(service)
	servName := serviceName(service)
	s.P(`func (s *`, servStruct, `) serve`, methName, `(resp `, s.pkgs["http"], `.ResponseWriter, req *`, s.pkgs["http"], `.Request) {`)
	s.P(`  ctx, req, psd := s.serviceTracer.Extract(req)`)
	s.P(`  ctx, span := s.serviceTracer.StartSpan(ctx, "`, methName, `", psd, blazetrace.WithAttributes(blazetrace.ServiceName.String("`, servName, `")))`)
	s.P(`  defer s.serviceTracer.EndSpan(span)`)
	s.P(`  header := req.Header.Get("Content-Type")`)
	s.P(`  i := strings.Index(header, ";")`)
	s.P(`  if i == -1 {`)
	s.P(`    i = len(header)`)
	s.P(`  }`)
	s.P(`  switch strings.TrimSpace(strings.ToLower(header[:i])) {`)
	s.P(`  case "application/json":`)
	s.P(`    s.serve`, methName, `JSON(ctx, resp, req)`)
	s.P(`  case "application/protobuf":`)
	s.P(`    s.serve`, methName, `Protobuf(ctx, resp, req)`)
	s.P(`  default:`)
	s.P(`    msg := `, s.pkgs["fmt"], `.Sprintf("unexpected Content-Type: %q", req.Header.Get("Content-Type"))`)
	s.P(`    blerr := `, s.pkgs["blaze"], `.ServerInvalidRequestError("Content-Type",msg, req.Method, req.URL.Path)`)
	s.P(`    `, s.pkgs["blaze"], `.ServerWriteError(ctx, resp, blerr, s.log)`)
	s.P(`  }`)
	s.P(`}`)
	s.P()
	s.generateServerJSONMethod(service, method)
	s.generateServerProtobufMethod(service, method)
}

func (s *blaze) generateServerJSONMethod(service *descriptor.ServiceDescriptorProto, method *descriptor.MethodDescriptorProto) {
	servStruct := serviceStruct(service)
	methName := stringutils.CamelCase(method.GetName())
	servName := serviceName(service)
	s.P(`func (s *`, servStruct, `) serve`, methName, `JSON(ctx `, s.pkgs["context"], `.Context, resp `, s.pkgs["http"], `.ResponseWriter, req *`, s.pkgs["http"], `.Request) {`)
	s.P(`  var err error`)
	s.P()
	s.P(`  reqContent := new(`, s.goTypeName(method.GetInputType()), `)`)
	s.P(`  unmarshaler := `, s.pkgs["jsonpb"], `.Unmarshaler{AllowUnknownFields: true}`)
	s.P(`  if err = unmarshaler.Unmarshal(req.Body, reqContent); err != nil {`)
	s.P(`    `, s.pkgs["blaze"], `.ServerWriteError(ctx, resp, `, s.pkgs["blaze"], `.ErrorMalformed("the json request could not be decoded"), s.log)`)
	s.P(`    return`)
	s.P(`  }`)
	s.P()
	s.P(`  // Call service method`)
	s.P(`  var respContent *`, s.goTypeName(method.GetOutputType()))
	s.P(`  func() {`)
	s.P(`    defer `, s.pkgs["blaze"], `.ServerEnsurePanicResponses(ctx, resp, s.log)`)
	s.P(`    respContent, err = s.`, servName, `.`, methName, `(ctx, reqContent)`)
	s.P(`  }()`)
	s.P()
	s.P(`  if err != nil {`)
	s.P(`    `, s.pkgs["blaze"], `.ServerWriteError(ctx, resp, err, s.log)`)
	s.P(`    return`)
	s.P(`  }`)
	s.P(`  if respContent == nil {`)
	s.P(`    `, s.pkgs["blaze"], `.ServerWriteError(ctx, resp, `, s.pkgs["blaze"], `.ErrorInternal("received a nil response. nil responses are not supported"), s.log)`)
	s.P(`    return`)
	s.P(`  }`)
	s.P()
	s.P()
	s.P(`  var buf `, s.pkgs["bytes"], `.Buffer`)
	s.P(`  marshaler := &`, s.pkgs["jsonpb"], `.Marshaler{`)
	s.P(`		OrigName:    true,`)
	s.P(`		EnumsAsInts: s.serviceOptions.JSONEnumsAsInts,`)
	s.P(`		EmitDefaults: s.serviceOptions.JSONEmitDefaults,`)
	s.P(`  }`)
	s.P(`  if err = marshaler.Marshal(&buf, respContent); err != nil {`)
	s.P(`    `, s.pkgs["blaze"], `.ServerWriteError(ctx, resp, `, s.pkgs["blaze"], `.ErrorInternalWith(err, "failed to marshal json response"), s.log)`)
	s.P(`    return`)
	s.P(`  }`)
	s.P()
	s.P(`  respBytes := buf.Bytes()`)
	s.P(`  resp.Header().Set("Content-Type", "application/json")`)
	s.P(`  resp.Header().Set("Content-Length", strconv.Itoa(len(respBytes)))`)
	s.P(`  resp.WriteHeader(`, s.pkgs["http"], `.StatusOK)`)
	s.P()
	s.P(`  if n, err := resp.Write(respBytes); err != nil {`)
	s.P(`    msg := fmt.Sprintf("failed to write response, %d of %d bytes written: %s", n, len(respBytes), err.Error())`)
	s.P(`    blerr := `, s.pkgs["blaze"], `.ErrorInternal(msg)`)
	s.P(`    s.log.Error(blerr, msg)`)
	s.P(`  }`)
	s.P(`}`)
	s.P()
}

func (s *blaze) generateServerProtobufMethod(service *descriptor.ServiceDescriptorProto, method *descriptor.MethodDescriptorProto) {
	servStruct := serviceStruct(service)
	methName := stringutils.CamelCase(method.GetName())
	servName := serviceName(service)
	s.P(`func (s *`, servStruct, `) serve`, methName, `Protobuf(ctx `, s.pkgs["context"], `.Context, resp `, s.pkgs["http"], `.ResponseWriter, req *`, s.pkgs["http"], `.Request) {`)
	s.P(`  var err error`)
	s.P(`  if err != nil {`)
	s.P(`    `, s.pkgs["blaze"], `.ServerWriteError(ctx, resp, err, s.log)`)
	s.P(`    return`)
	s.P(`  }`)
	s.P()
	s.P(`  buf, err := `, s.pkgs["ioutil"], `.ReadAll(req.Body)`)
	s.P(`  if err != nil {`)
	s.P(`    `, s.pkgs["blaze"], `.ServerWriteError(ctx, resp, `, s.pkgs["blaze"], `.ErrorInternalWith(err, "failed to read request body"), s.log)`)
	s.P(`    return`)
	s.P(`  }`)
	s.P(`  reqContent := new(`, s.goTypeName(method.GetInputType()), `)`)
	s.P(`  if err = `, s.pkgs["proto"], `.Unmarshal(buf, reqContent); err != nil {`)
	s.P(`    `, s.pkgs["blaze"], `.ServerWriteError(ctx, resp, `, s.pkgs["blaze"], `.ErrorMalformed("the protobuf request could not be decoded"), s.log)`)
	s.P(`    return`)
	s.P(`  }`)
	s.P()
	s.P(`  // Call service method`)
	s.P(`  var respContent *`, s.goTypeName(method.GetOutputType()))
	s.P(`  func() {`)
	s.P(`    defer  `, s.pkgs["blaze"], `.ServerEnsurePanicResponses(ctx, resp, s.log)`)
	s.P(`    respContent, err = s.`, servName, `.`, methName, `(ctx, reqContent)`)
	s.P(`  }()`)
	s.P()
	s.P(`  if err != nil {`)
	s.P(`    `, s.pkgs["blaze"], `.ServerWriteError(ctx, resp, err, s.log)`)
	s.P(`    return`)
	s.P(`  }`)
	s.P(`  if respContent == nil {`)
	s.P(`    `, s.pkgs["blaze"], `.ServerWriteError(ctx, resp, `, s.pkgs["blaze"], `.ErrorInternal("received a nil *`, s.goTypeName(method.GetOutputType()), ` and nil error while calling `, methName, `. nil responses are not supported"), s.log)`)
	s.P(`    return`)
	s.P(`  }`)
	s.P()
	s.P(`  respBytes, err := `, s.pkgs["proto"], `.Marshal(respContent)`)
	s.P(`  if err != nil {`)
	s.P(`    `, s.pkgs["blaze"], `.ServerWriteError(ctx, resp, `, s.pkgs["blaze"], `.ErrorInternalWith(err, "failed to marshal proto response"), s.log)`)
	s.P(`    return`)
	s.P(`  }`)
	s.P()
	s.P(`  resp.Header().Set("Content-Type", "application/protobuf")`)
	s.P(`  resp.Header().Set("Content-Length", strconv.Itoa(len(respBytes)))`)
	s.P(`  resp.WriteHeader(`, s.pkgs["http"], `.StatusOK)`)
	s.P(`  if n, err := resp.Write(respBytes); err != nil {`)
	s.P(`    msg := fmt.Sprintf("failed to write response, %d of %d bytes written: %s", n, len(respBytes), err.Error())`)
	s.P(`    blerr := `, s.pkgs["blaze"], `.ErrorInternal(msg)`)
	s.P(`    s.log.Error(blerr, msg)`)
	s.P(`  }`)
	s.P(`}`)
	s.P()
}
func (s *blaze) generateSignature(method *descriptor.MethodDescriptorProto) string {
	methName := methodName(method)
	inputType := s.goTypeName(method.GetInputType())
	outputType := s.goTypeName(method.GetOutputType())
	return fmt.Sprintf(`	%s(%s.Context, *%s) (*%s, error)`, methName, s.pkgs["context"], inputType, outputType)
}

func (s *blaze) generateFileDescriptor(file *descriptor.FileDescriptorProto) {
	// Copied straight of of protoc-gen-go, which trims out comments.
	pb := proto.Clone(file).(*descriptor.FileDescriptorProto)
	pb.SourceCodeInfo = nil

	b, err := proto.Marshal(pb)
	if err != nil {
		s.log.Error(err, "file descriptor marshal failed")
	}

	var buf bytes.Buffer
	w, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	w.Write(b)
	w.Close()
	b = buf.Bytes()

	v := s.serviceMetadataVarName()
	s.P()
	s.P("var ", v, " = []byte{")
	s.P("	// ", fmt.Sprintf("%d", len(b)), " bytes of a gzipped FileDescriptorProto")
	for len(b) > 0 {
		n := 16
		if n > len(b) {
			n = len(b)
		}

		str := ""
		for _, c := range b[:n] {
			str += fmt.Sprintf("0x%02x,", c)
		}
		s.P(`	`, str)

		b = b[n:]
	}
	s.P("}")
}

func (s *blaze) generateImplementInterface(file *descriptor.FileDescriptorProto, service *descriptor.ServiceDescriptorProto) {
	servStruct := serviceStruct(service)
	s.P(`func (s *`, servStruct, `) Mux() *chi.Mux {`)
	s.P(`	return s.mux`)
	s.P(`}`)
	s.P()
	s.P(`func (s *`, servStruct, `) MountPath() string {`)
	s.P(`	return s.mountPath`)
	s.P(`}`)
}

// valid names: 'JSON', 'Protobuf'
func (s *blaze) generateClient(name string, file *descriptor.FileDescriptorProto, service *descriptor.ServiceDescriptorProto) {
	servName := serviceName(service)
	pathPrefixConst := servName + "PathPrefix"
	structName := unexported(servName) + name + "Client"
	newClientFunc := "New" + servName + name + "Client"

	methCnt := strconv.Itoa(len(service.Method))
	s.P(`type `, structName, ` struct {`)
	s.P(`  client `, s.pkgs["blaze"], `.HTTPClient`)
	s.P(`  urls   [`, methCnt, `]string`)
	s.P(`  opts `, s.pkgs["blaze"], `.ClientOptions`)
	s.P(`  trace `, s.pkgs["blazetrace"], `.ClientTracer`)
	s.P(`}`)
	s.P()
	s.P(`// `, newClientFunc, ` creates a `, name, ` client that implements the `, servName, ` interface.`)
	s.P(`// It communicates using `, name, ` and can be configured with a custom HTTPClient.`)
	s.P(`func `, newClientFunc, `(addr string, client `, s.pkgs["blaze"], `.HTTPClient, opts ...`, s.pkgs["blaze"], `.ClientOption) `, servName, ` {`)
	s.P(`  if c, ok := client.(*`, s.pkgs["http"], `.Client); ok {`)
	s.P(`    client = `, s.pkgs["blaze"], `.WithoutRedirects(c)`)
	s.P(`  }`)
	s.P()
	s.P(`  clientOpts := `, s.pkgs["blaze"], `.ClientOptions{`)
	s.P(`    Trace: `, s.pkgs["blazetrace"], `.NewClientTracer(),`)
	s.P(`  }`)
	s.P(`  for _, o := range opts {`)
	s.P(`    o(&clientOpts)`)
	s.P(`  }`)
	s.P()
	if len(service.Method) > 0 {
		s.P(`  prefix := `, s.pkgs["blaze"], `.UrlBase(addr) + `, pathPrefixConst)
	}
	s.P(`  urls := [`, methCnt, `]string{`)
	for _, method := range service.Method {
		s.P(`    	prefix +"/"+ "`, methodName(method), `",`)
	}
	s.P(`  }`)
	s.P()
	s.P(`  return &`, structName, `{`)
	s.P(`    client: client,`)
	s.P(`    urls:   urls,`)
	s.P(`    opts: clientOpts,`)
	s.P(`    trace: clientOpts.Trace,`)
	s.P(`  }`)
	s.P(`}`)
	s.P()

	for i, method := range service.Method {
		methName := methodName(method)
		//pkgName := pkgName(file)
		servName := serviceName(service)
		inputType := s.goTypeName(method.GetInputType())
		outputType := s.goTypeName(method.GetOutputType())

		s.P(`func (s *`, structName, `) `, methName, `(ctx `, s.pkgs["context"], `.Context, in *`, inputType, `) (*`, outputType, `, error) {`)
		s.P(`  ctx, span := s.trace.StartSpan(ctx, "`, methName, `", `, s.pkgs["blazetrace"], `.WithAttributes(`, s.pkgs["blazetrace"], `.ClientName.String("`, servName, `")))`)
		s.P(`  defer s.trace.EndSpan(span)`)
		s.P(`  out := new(`, outputType, `)`)
		s.P(`  err := s.do`, name, `Request(ctx, s.client, s.urls[`, strconv.Itoa(i), `], in, out)`)
		s.P(`  if err != nil {`)
		s.P(`    blerr, ok := err.(`, s.pkgs["blaze"], `.Error)`)
		s.P(`    if !ok {`)
		s.P(`      blerr = `, s.pkgs["blaze"], `.ErrorInternalWith(err,"")`)
		s.P(`    }`)
		s.P(`  span.SetStatus(`, s.pkgs["blaze"], `.GrpcCodeFromErrorType(blerr))`)
		s.P(`    return nil, blerr`)
		s.P(`  }`)
		s.P(`  span.SetStatus(`, s.pkgs["blaze"], `.GrpcCodeFromErrorType(nil))`)
		s.P(`  return out, nil`)
		s.P(`}`)
		s.P()
	}
	if name == "Protobuf" {
		s.P(`// doProtobufRequest makes a Protobuf request to the remote Blaze service.`)
		s.P(`func (s *`, structName, `) doProtobufRequest(ctx `, s.pkgs["context"], `.Context, client `, s.pkgs["blaze"], `.HTTPClient, url string, in, out `, s.pkgs["proto"], `.Message) (err error) {`)
		s.P(`  reqBodyBytes, err := `, s.pkgs["proto"], `.Marshal(in)`)
		s.P(`  if err != nil {`)
		s.P(`    return `, s.pkgs["blaze"], `.ErrorInternalWith(err, "failed to marshal proto request")`)
		s.P(`  }`)
		s.P(`  reqBody := `, s.pkgs["bytes"], `.NewBuffer(reqBodyBytes)`)
		s.P(`  if err = ctx.Err(); err != nil {`)
		s.P(`    return `, s.pkgs["blaze"], `.ErrorInternalWith(err, "aborted because context was done")`)
		s.P(`  }`)
		s.P()
		s.P(`  req, err := `, s.pkgs["blaze"], `.NewHTTPRequest(ctx, url, reqBody, "application/protobuf","`, Version, `")`)
		s.P(`  if err != nil {`)
		s.P(`    return `, s.pkgs["blaze"], `.ErrorInternalWith(err, "could not build request")`)
		s.P(`  }`)
		s.P()
		s.P(`  ctx, req = s.trace.Inject(ctx, req)`)
		s.P(`  req = req.WithContext(ctx)`)
		s.P(`  resp, err := client.Do(req)`)
		s.P(`  if err != nil {`)
		s.P(`    return `, s.pkgs["blaze"], `.ErrorInternalWith(err, "failed to do request")`)
		s.P(`  }`)
		s.P()
		s.P(`  defer func() {`)
		s.P(`    cerr := resp.Body.Close()`)
		s.P(`    if err == nil && cerr != nil {`)
		s.P(`      err = `, s.pkgs["blaze"], `.ErrorInternalWith(cerr, "failed to close response body")`)
		s.P(`    }`)
		s.P(`  }()`)
		s.P()
		s.P(`  if err = ctx.Err(); err != nil {`)
		s.P(`    return `, s.pkgs["blaze"], `.ErrorInternalWith(err, "aborted because context was done")`)
		s.P(`  }`)
		s.P()
		s.P(`  if resp.StatusCode != 200 {`)
		s.P(`    return `, s.pkgs["blaze"], `.ErrorFromResponse(resp)`)
		s.P(`  }`)
		s.P()
		s.P(`  respBodyBytes, err := `, s.pkgs["ioutil"], `.ReadAll(resp.Body)`)
		s.P(`  if err != nil {`)
		s.P(`    return `, s.pkgs["blaze"], `.ErrorInternalWith(err, "failed to read response body")`)
		s.P(`  }`)
		s.P(`  if err = ctx.Err(); err != nil {`)
		s.P(`    return `, s.pkgs["blaze"], `.ErrorInternalWith(err, "aborted because context was done")`)
		s.P(`  }`)
		s.P()
		s.P(`  if err = `, s.pkgs["proto"], `.Unmarshal(respBodyBytes, out); err != nil {`)
		s.P(`    return `, s.pkgs["blaze"], `.ErrorInternalWith(err, "failed to unmarshal proto response")`)
		s.P(`  }`)
		s.P(`  return nil`)
		s.P(`}`)
		s.P()
	}

	if name == "JSON" {
		s.P(`// doJSONRequest makes a JSON request to the remote Blaze service.`)
		s.P(`func (s *`, structName, `) doJSONRequest(ctx `, s.pkgs["context"], `.Context, client `, s.pkgs["blaze"], `.HTTPClient, url string, in, out `, s.pkgs["proto"], `.Message) (err error) {`)
		s.P(`  reqBody := `, s.pkgs["bytes"], `.NewBuffer(nil)`)
		s.P(`  marshaler := &`, s.pkgs["jsonpb"], `.Marshaler{OrigName: true}`)
		s.P(`  if err = marshaler.Marshal(reqBody, in); err != nil {`)
		s.P(`    return `, s.pkgs["blaze"], `.ErrorInternalWith(err, "failed to marshal json request")`)
		s.P(`  }`)
		s.P(`  if err = ctx.Err(); err != nil {`)
		s.P(`    return `, s.pkgs["blaze"], `.ErrorInternalWith(err, "aborted because context was done")`)
		s.P(`  }`)
		s.P()
		s.P(`  req, err := `, s.pkgs["blaze"], `.NewHTTPRequest(ctx, url, reqBody, "application/json","`, Version, `")`)
		s.P(`  if err != nil {`)
		s.P(`    return `, s.pkgs["blaze"], `.ErrorInternalWith(err, "could not build request")`)
		s.P(`  }`)
		s.P()
		s.P(`  ctx, req = s.trace.Inject(ctx, req)`)
		s.P(`  req = req.WithContext(ctx)`)
		s.P(`  resp, err := client.Do(req)`)
		s.P(`  if err != nil {`)
		s.P(`    return `, s.pkgs["blaze"], `.ErrorInternalWith(err, "failed to do request")`)
		s.P(`  }`)
		s.P()
		s.P(`  defer func() {`)
		s.P(`    cerr := resp.Body.Close()`)
		s.P(`    if err == nil && cerr != nil {`)
		s.P(`      err = `, s.pkgs["blaze"], `.ErrorInternalWith(cerr, "failed to close response body")`)
		s.P(`    }`)
		s.P(`  }()`)
		s.P()
		s.P(`  if err = ctx.Err(); err != nil {`)
		s.P(`    return `, s.pkgs["blaze"], `.ErrorInternalWith(err, "aborted because context was done")`)
		s.P(`  }`)
		s.P()
		s.P(`  if resp.StatusCode != 200 {`)
		s.P(`    return `, s.pkgs["blaze"], `.ErrorFromResponse(resp)`)
		s.P(`  }`)
		s.P()
		s.P(`  unmarshaler := `, s.pkgs["jsonpb"], `.Unmarshaler{AllowUnknownFields: true}`)
		s.P(`  if err = unmarshaler.Unmarshal(resp.Body, out); err != nil {`)
		s.P(`    return `, s.pkgs["blaze"], `.ErrorInternalWith(err, "failed to unmarshal json response")`)
		s.P(`  }`)
		s.P(`  if err = ctx.Err(); err != nil {`)
		s.P(`    return `, s.pkgs["blaze"], `.ErrorInternalWith(err, "aborted because context was done")`)
		s.P(`  }`)
		s.P(`  return nil`)
		s.P(`}`)
	}
}

// serviceMetadataVarName is the variable name used in generated code to refer
// to the compressed bytes of this descriptor. It is not exported, so it is only
// valid inside the generated package.
//
// protoc-gen-go writes its own version of this file, but so does
// protoc-gen-gogo - with a different name! Blaze aims to be compatible with
// both; the simplest way forward is to write the file descriptor again as
// another variable that we control.
func (s *blaze) serviceMetadataVarName() string {
	return fmt.Sprintf("blazeFileDescriptor_%s", s.filesHash)
}

func (s *blaze) formattedOutput() string {
	// Reformat generated code.
	fset := token.NewFileSet()
	raw := s.output.Bytes()
	ast, err := parser.ParseFile(fset, "", raw, parser.ParseComments)
	if err != nil {
		// Print out the bad code with line numbers.
		// This should never happen in practice, but it can while changing generated code,
		// so consider this a debugging aid.
		var src bytes.Buffer
		sc := bufio.NewScanner(bytes.NewReader(raw))
		for line := 1; sc.Scan(); line++ {
			fmt.Fprintf(&src, "%5d\t%s\n", line, sc.Bytes())
		}
		fail("bad Go source code was generated:", err.Error(), "\n"+src.String())
		s.log.Error(err, "bad Go source code was generated")

	}

	out := bytes.NewBuffer(nil)
	err = (&printer.Config{Mode: printer.TabIndent | printer.UseSpaces, Tabwidth: 8}).Fprint(out, fset, ast)
	if err != nil {
		s.log.Error(err, "failed to format go code")
	}

	return out.String()
}

func (s *blaze) printComments(comments typemap.DefinitionComments) bool {
	text := strings.TrimSuffix(comments.Leading, "\n")
	if len(strings.TrimSpace(text)) == 0 {
		return false
	}
	split := strings.Split(text, "\n")
	for _, line := range split {
		s.P("// ", strings.TrimPrefix(line, " "))
	}
	return len(split) > 0
}

// Given a protobuf name for a Message, return the Go name we will use for that
// type, including its package prefix.
func (s *blaze) goTypeName(protoName string) string {
	def := s.reg.MessageDefinition(protoName)
	if def == nil {
		s.log.Error(errors.New("message not found"), "name", protoName)
	}

	var prefix string
	if pkg := s.goPackageName(def.File); pkg != s.genPkgName {
		prefix = pkg + "."
	}

	var name string
	for _, parent := range def.Lineage() {
		name += stringutils.CamelCase(parent.Descriptor.GetName()) + "_"
	}
	name += stringutils.CamelCase(def.Descriptor.GetName())
	return prefix + name
}

func unexported(s string) string { return strings.ToLower(s[:1]) + s[1:] }

func fullServiceName(file *descriptor.FileDescriptorProto, service *descriptor.ServiceDescriptorProto) string {
	name := stringutils.CamelCase(service.GetName())
	if pkg := pkgName(file); pkg != "" {
		name = pkg + "." + name
	}
	return name
}

func pkgName(file *descriptor.FileDescriptorProto) string {
	return file.GetPackage()
}

func serviceName(service *descriptor.ServiceDescriptorProto) string {
	return stringutils.CamelCase(service.GetName())
}

func serviceStruct(service *descriptor.ServiceDescriptorProto) string {
	return unexported(serviceName(service)) + "Service"
}

func methodName(method *descriptor.MethodDescriptorProto) string {
	return stringutils.CamelCase(method.GetName())
}

func fileDescSliceContains(slice []*descriptor.FileDescriptorProto, f *descriptor.FileDescriptorProto) bool {
	for _, sf := range slice {
		if f == sf {
			return true
		}
	}
	return false
}

func fail(msgs ...string) {
	s := strings.Join(msgs, " ")
	log.Print("error:", s)
	os.Exit(1)
}
