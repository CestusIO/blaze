package internal_gengo

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"code.cestus.io/blaze/internal/generation/fieldnum"
	"github.com/go-logr/logr"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/runtime/protoimpl"

	"google.golang.org/protobuf/types/pluginpb"
)

// SupportedFeatures reports the set of supported protobuf language features.
var SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)

// GenerateVersionMarkers specifies whether to generate version markers.
var GenerateVersionMarkers = true

// Standard library dependencies.
const (
	mathPackage    = protogen.GoImportPath("math")
	reflectPackage = protogen.GoImportPath("reflect")
	syncPackage    = protogen.GoImportPath("sync")
)

// Protobuf library dependencies.
//
// These are declared as an interface type so that they can be more easily
// patched to support unique build environments that impose restrictions
// on the dependencies of generated source code.
var (
	protoPackage        goImportPath = protogen.GoImportPath("google.golang.org/protobuf/proto")
	protoifacePackage   goImportPath = protogen.GoImportPath("google.golang.org/protobuf/runtime/protoiface")
	protoimplPackage    goImportPath = protogen.GoImportPath("google.golang.org/protobuf/runtime/protoimpl")
	protoreflectPackage goImportPath = protogen.GoImportPath("google.golang.org/protobuf/reflect/protoreflect")
	bytesPackage        goImportPath = protogen.GoImportPath("bytes")
	stringsPackage      goImportPath = protogen.GoImportPath("strings")
	fmtPackage          goImportPath = protogen.GoImportPath("fmt")
	strconvPackage      goImportPath = protogen.GoImportPath("strconv")
	contextPackage      goImportPath = protogen.GoImportPath("context")
	ioutilPackage       goImportPath = protogen.GoImportPath("io/ioutil")
	httpPackage         goImportPath = protogen.GoImportPath("net/http")
	protoJSONPackage    goImportPath = protogen.GoImportPath("google.golang.org/protobuf/encoding/protojson")
	logrPackage         goImportPath = protogen.GoImportPath("github.com/go-logr/logr")
	chiPackage          goImportPath = protogen.GoImportPath("github.com/go-chi/chi")
	blazePackage        goImportPath = protogen.GoImportPath("code.cestus.io/blaze")
	blazetracePackage   goImportPath = protogen.GoImportPath("code.cestus.io/blaze/pkg/blazetrace")
)

// s.P(`import `, s.pkgs["bytes"], ` "bytes"`)
// // some packages is only used in generated method code.
// if !allServicesEmpty {
// 	s.P(`import `, s.pkgs["strings"], ` "strings"`)
// 	s.P(`import `, s.pkgs["fmt"], ` "fmt"`)
// 	s.P(`import `, s.pkgs["strconv"], ` "strconv"`)
// }
// s.P(`import `, s.pkgs["context"], ` "context"`)
// s.P(`import `, s.pkgs["ioutil"], ` "io/ioutil"`)
// s.P(`import `, s.pkgs["http"], ` "net/http"`)
// s.P()
// s.P(`import `, s.pkgs["jsonpb"], ` "github.com/golang/protobuf/jsonpb"`)
// s.P(`import `, s.pkgs["proto"], ` "github.com/golang/protobuf/proto"`)
// s.P(`import `, s.pkgs["logr"], ` "github.com/go-logr/logr"`)
// s.P(`import `, s.pkgs["chi"], ` "github.com/go-chi/chi"`)
// s.P(`import `, s.pkgs["blaze"], ` "code.cestus.io/blaze"`)
// s.P(`import `, s.pkgs["blazetrace"], ` "code.cestus.io/blaze/pkg/blazetrace"`)
// s.P()
type goImportPath interface {
	String() string
	Ident(string) protogen.GoIdent
}

type Blaze struct {
	log     logr.Logger
	version string
}

// NewGenerator creates a new generator
func NewGenerator(log logr.Logger, version string) *Blaze {
	s := &Blaze{
		log:     log,
		version: version,
	}
	return s
}

// GenerateFile generates the contents of a .pb.go file.
func (s *Blaze) GenerateFile(gen *protogen.Plugin, file *protogen.File) *protogen.GeneratedFile {
	if len(file.Services) == 0 {
		return nil
	}
	filename := file.GeneratedFilenamePrefix + ".blaze.go"
	g := gen.NewGeneratedFile(filename, file.GoImportPath)
	f := newFileInfo(file)
	s.genStandaloneComments(g, f, fieldnum.FileDescriptorProto_Syntax)
	s.genGeneratedHeader(gen, g, f)
	s.genStandaloneComments(g, f, fieldnum.FileDescriptorProto_Package)
	g.P("package ", f.GoPackageName)
	g.P()
	// Emit a static check that enforces a minimum version of the proto package.
	if GenerateVersionMarkers {
		g.P("const (")
		g.P("// Verify that this generated code is sufficiently up-to-date.")
		g.P("_ = ", protoimplPackage.Ident("EnforceVersion"), "(", protoimpl.GenVersion, " - ", protoimplPackage.Ident("MinVersion"), ")")
		g.P("// Verify that runtime/protoimpl is sufficiently up-to-date.")
		g.P("_ = ", protoimplPackage.Ident("EnforceVersion"), "(", protoimplPackage.Ident("MaxVersion"), " - ", protoimpl.GenVersion, ")")
		g.P(")")
		g.P()
	}

	for i, imps := 0, f.Desc.Imports(); i < imps.Len(); i++ {
		s.genImport(gen, g, f, imps.Get(i))
	}

	for _, service := range f.Services {
		s.genService(gen, f, g, service)
	}
	//genReflectFileDescriptor(gen, g, f)
	return g
}

// genStandaloneComments prints all leading comments for a FileDescriptorProto
// location identified by the field number n.
func (s *Blaze) genStandaloneComments(g *protogen.GeneratedFile, f *fileInfo, n int32) {
	for _, loc := range f.Proto.GetSourceCodeInfo().GetLocation() {
		if len(loc.Path) == 1 && loc.Path[0] == n {
			for _, s := range loc.GetLeadingDetachedComments() {
				g.P(protogen.Comments(s))
				g.P()
			}
			if s := loc.GetLeadingComments(); s != "" {
				g.P(protogen.Comments(s))
				g.P()
			}
		}
	}
}

func (s *Blaze) genGeneratedHeader(gen *protogen.Plugin, g *protogen.GeneratedFile, f *fileInfo) {
	g.P("// Code generated by protoc-gen-blaze. DO NOT EDIT.")

	if GenerateVersionMarkers {
		g.P("// versions:")
		protocGenBlazeVersion := s.version
		protocVersion := "(unknown)"
		if v := gen.Request.GetCompilerVersion(); v != nil {
			protocVersion = fmt.Sprintf("v%v.%v.%v", v.GetMajor(), v.GetMinor(), v.GetPatch())
		}
		g.P("// \tblaze-gen-go ", protocGenBlazeVersion)
		g.P("// \tprotoc        ", protocVersion)
	}

	if f.Proto.GetOptions().GetDeprecated() {
		g.P("// ", f.Desc.Path(), " is a deprecated file.")
	} else {
		g.P("// source: ", f.Desc.Path())
	}
	g.P()
}

func (s *Blaze) genImport(gen *protogen.Plugin, g *protogen.GeneratedFile, f *fileInfo, imp protoreflect.FileImport) {
	impFile, ok := gen.FilesByPath[imp.Path()]
	if !ok {
		return
	}
	if impFile.GoImportPath == f.GoImportPath {
		// Don't generate imports or aliases for types in the same Go package.
		return
	}
	// Generate imports for all non-weak dependencies, even if they are not
	// referenced, because other code and tools depend on having the
	// full transitive closure of protocol buffer types in the binary.
	if !imp.IsWeak {
		g.Import(impFile.GoImportPath)
	}
	if !imp.IsPublic {
		return
	}

	// Generate public imports by generating the imported file, parsing it,
	// and extracting every symbol that should receive a forwarding declaration.
	impGen := s.GenerateFile(gen, impFile)
	impGen.Skip()
	b, err := impGen.Content()
	if err != nil {
		gen.Error(err)
		return
	}
	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, "", b, parser.ParseComments)
	if err != nil {
		gen.Error(err)
		return
	}
	genForward := func(tok token.Token, name string, expr ast.Expr) {
		// Don't import unexported symbols.
		r, _ := utf8.DecodeRuneInString(name)
		if !unicode.IsUpper(r) {
			return
		}
		// Don't import the FileDescriptor.
		if name == impFile.GoDescriptorIdent.GoName {
			return
		}
		// Don't import decls referencing a symbol defined in another package.
		// i.e., don't import decls which are themselves public imports:
		//
		//	type T = somepackage.T
		if _, ok := expr.(*ast.SelectorExpr); ok {
			return
		}
		g.P(tok, " ", name, " = ", impFile.GoImportPath.Ident(name))
	}
	g.P("// Symbols defined in public import of ", imp.Path(), ".")
	g.P()
	for _, decl := range astFile.Decls {
		switch decl := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					genForward(decl.Tok, spec.Name.Name, spec.Type)
				case *ast.ValueSpec:
					for i, name := range spec.Names {
						var expr ast.Expr
						if i < len(spec.Values) {
							expr = spec.Values[i]
						}
						genForward(decl.Tok, name.Name, expr)
					}
				case *ast.ImportSpec:
				default:
					panic(fmt.Sprintf("can't generate forward for spec type %T", spec))
				}
			}
		}
	}
	g.P()
}

// Big header comments to makes it easier to visually parse a generated file.
func (s *Blaze) sectionComment(g *protogen.GeneratedFile, sectionTitle string) {
	g.P()
	g.P(`// `, strings.Repeat("=", len(sectionTitle)))
	g.P(`// `, sectionTitle)
	g.P(`// `, strings.Repeat("=", len(sectionTitle)))
	g.P()
}

func (s *Blaze) generateBlazeInterface(g *protogen.GeneratedFile, file *fileInfo, service *protogen.Service) {
	servName := service.GoName
	g.Annotate(service.GoName, service.Location)
	g.P(service.Comments.Leading, `type `, servName, ` interface {`)
	for _, method := range service.Methods {
		s.generateMethodSignature(g, method)
	}
	g.P(`}`)
}
func (s *Blaze) generateMethodSignature(g *protogen.GeneratedFile, method *protogen.Method) {
	var reqArgs []string
	reqArgs = append(reqArgs, g.QualifiedGoIdent(contextPackage.Ident("Context")))
	reqArgs = append(reqArgs, fmt.Sprint("*", g.QualifiedGoIdent(method.Input.GoIdent)))
	ret := "(*" + g.QualifiedGoIdent(method.Output.GoIdent) + ", error)"
	g.P(method.Comments.Leading, method.GoName+"("+strings.Join(reqArgs, ", ")+") "+ret)
}

// appendDeprecationSuffix optionally appends a deprecation notice as a suffix.
func (s *Blaze) appendDeprecationSuffix(prefix protogen.Comments, deprecated bool) protogen.Comments {
	if !deprecated {
		return prefix
	}
	if prefix != "" {
		prefix += "\n"
	}
	return prefix + " Deprecated: Do not use.\n"
}

func (s *Blaze) genService(gen *protogen.Plugin, file *fileInfo, g *protogen.GeneratedFile, service *protogen.Service) {
	servName := service.GoName
	s.sectionComment(g, servName+` Interface`)
	s.generateBlazeInterface(g, file, service)
	s.sectionComment(g, servName+` Protobuf Client`)
	s.generateClient("Protobuf", g, file, service)
	s.sectionComment(g, servName+` JSON Client`)
	s.generateClient("JSON", g, file, service)
	// Service
	s.sectionComment(g, servName+` Service`)
	s.generateServer(g, file, service)
	s.generateImplementInterface(g, file, service)
}

func (s *Blaze) generateImplementInterface(g *protogen.GeneratedFile, file *fileInfo, service *protogen.Service) {
	servStruct := serviceStruct(service)
	g.P(`func (s *`, servStruct, `) Mux() *`, g.QualifiedGoIdent(chiPackage.Ident("Mux")), ` {`)
	g.P(`	return s.mux`)
	g.P(`}`)
	g.P()
	g.P(`func (s *`, servStruct, `) MountPath() string {`)
	g.P(`	return s.mountPath`)
	g.P(`}`)
}

// valid names: 'JSON', 'Protobuf'
func (s *Blaze) generateClient(name string, g *protogen.GeneratedFile, file *fileInfo, service *protogen.Service) {
	servName := service.GoName
	pathPrefixConst := servName + "PathPrefix"
	structName := unexported(servName) + name + "Client"
	newClientFunc := "New" + servName + name + "Client"
	methCnt := strconv.Itoa(len(service.Methods))
	g.P(`type `, structName, ` struct {`)
	g.P(`client `, g.QualifiedGoIdent(blazePackage.Ident("HTTPClient")))
	g.P(`urls   [`, methCnt, `]string`)
	g.P(`opts `, g.QualifiedGoIdent(blazePackage.Ident("ClientOptions")))
	g.P(`trace `, g.QualifiedGoIdent(blazetracePackage.Ident("ClientTracer")))
	g.P(`}`)
	g.P(`// `, newClientFunc, ` creates a `, name, ` client that implements the `, servName, ` interface.`)
	g.P(`// It communicates using `, name, ` and can be configured with a custom HTTPClient.`)
	g.P(`func `, newClientFunc, `(addr string, client `, g.QualifiedGoIdent(blazePackage.Ident("HTTPClient")), `, opts ...`, g.QualifiedGoIdent(blazePackage.Ident("ClientOption")), `) `, servName, ` {`)
	g.P(`  if c, ok := client.(*`, g.QualifiedGoIdent(httpPackage.Ident("Client")), `); ok {`)
	g.P(`    client = `, g.QualifiedGoIdent(blazePackage.Ident("WithoutRedirects")), `(c)`)
	g.P(`  }`)
	g.P()
	g.P(`  clientOpts := `, g.QualifiedGoIdent(blazePackage.Ident("ClientOptions")), `{`)
	g.P(`    Trace: `, g.QualifiedGoIdent(blazetracePackage.Ident("NewClientTracer")), `(),`)
	g.P(`  }`)
	g.P(`  for _, o := range opts {`)
	g.P(`    o(&clientOpts)`)
	g.P(`  }`)
	g.P()
	if len(service.Methods) > 0 {
		g.P(`  prefix := `, g.QualifiedGoIdent(blazePackage.Ident("UrlBase")), `(addr) + `, pathPrefixConst)
	}
	g.P(`  urls := [`, methCnt, `]string{`)
	for _, method := range service.Methods {
		g.P(`    	prefix +"/"+ "`, method.GoName, `",`)
	}
	g.P(`  }`)
	g.P()
	g.P(`  return &`, structName, `{`)
	g.P(`    client: client,`)
	g.P(`    urls:   urls,`)
	g.P(`    opts: clientOpts,`)
	g.P(`    trace: clientOpts.Trace,`)
	g.P(`  }`)
	g.P(`}`)
	g.P()
	for i, method := range service.Methods {
		methName := method.GoName
		//pkgName := pkgName(file)
		servName := service.GoName
		// inputType := s.goTypeName(method.Input.GoIdent.)
		// outputType := s.goTypeName(method.GetOutputType())

		g.P(`func (s *`, structName, `) `, methName, `(ctx `, g.QualifiedGoIdent(contextPackage.Ident("Context")), `, in *`, g.QualifiedGoIdent(method.Input.GoIdent), `) (*`, g.QualifiedGoIdent(method.Output.GoIdent), `, error) {`)
		g.P(`  ctx, span := s.trace.StartSpan(ctx, "`, methName, `", `, g.QualifiedGoIdent(blazetracePackage.Ident("WithAttributes")), `(`, g.QualifiedGoIdent(blazetracePackage.Ident("ClientName")), `.String("`, servName, `")))`)
		g.P(`  defer s.trace.EndSpan(span)`)
		g.P(`  out := new(`, g.QualifiedGoIdent(method.Output.GoIdent), `)`)
		g.P(`  err := s.do`, name, `Request(ctx, s.client, s.urls[`, strconv.Itoa(i), `], in, out)`)
		g.P(`  if err != nil {`)
		g.P(`    blerr, ok := err.(`, g.QualifiedGoIdent(blazePackage.Ident("Error")), `)`)
		g.P(`    if !ok {`)
		g.P(`      blerr = `, g.QualifiedGoIdent(blazePackage.Ident("ErrorInternalWith")), `(err,"")`)
		g.P(`    }`)
		g.P(`  span.SetStatus(`, g.QualifiedGoIdent(blazePackage.Ident("GrpcCodeFromErrorType")), `(blerr))`)
		g.P(`    return nil, blerr`)
		g.P(`  }`)
		g.P(`  span.SetStatus(`, g.QualifiedGoIdent(blazePackage.Ident("GrpcCodeFromErrorType")), `(nil))`)
		g.P(`  return out, nil`)
		g.P(`}`)
		g.P()
	}
	if name == "Protobuf" {
		g.P(`// doProtobufRequest makes a Protobuf request to the remote Blaze service.`)
		g.P(`func (s *`, structName, `) doProtobufRequest(ctx `, g.QualifiedGoIdent(contextPackage.Ident("Context")), `, client `, g.QualifiedGoIdent(blazePackage.Ident("HTTPClient")), `, url string, in, out `, g.QualifiedGoIdent(protoPackage.Ident("Message")), `) (err error) {`)
		g.P(`  reqBodyBytes, err := `, g.QualifiedGoIdent(protoPackage.Ident("Marshal")), `(in)`)
		g.P(`  if err != nil {`)
		g.P(`    return `, g.QualifiedGoIdent(blazePackage.Ident("ErrorInternalWith")), `(err, "failed to marshal proto request")`)
		g.P(`  }`)
		g.P(`  reqBody := `, g.QualifiedGoIdent(bytesPackage.Ident("NewBuffer")), `(reqBodyBytes)`)
		g.P(`  if err = ctx.Err(); err != nil {`)
		g.P(`    return `, g.QualifiedGoIdent(blazePackage.Ident("ErrorInternalWith")), `(err, "aborted because context was done")`)
		g.P(`  }`)
		g.P()
		g.P(`  req, err := `, g.QualifiedGoIdent(blazePackage.Ident("NewHTTPRequest")), `(ctx, url, reqBody, "application/protobuf","`, s.version, `")`)
		g.P(`  if err != nil {`)
		g.P(`    return `, g.QualifiedGoIdent(blazePackage.Ident("ErrorInternalWith")), `(err, "could not build request")`)
		g.P(`  }`)
		g.P()
		g.P(`  ctx, req = s.trace.Inject(ctx, req)`)
		g.P(`  req = req.WithContext(ctx)`)
		g.P(`  resp, err := client.Do(req)`)
		g.P(`  if err != nil {`)
		g.P(`    return `, g.QualifiedGoIdent(blazePackage.Ident("ErrorInternalWith")), `(err, "failed to do request")`)
		g.P(`  }`)
		g.P()
		g.P(`  defer func() {`)
		g.P(`    cerr := resp.Body.Close()`)
		g.P(`    if err == nil && cerr != nil {`)
		g.P(`      err = `, g.QualifiedGoIdent(blazePackage.Ident("ErrorInternalWith")), `(cerr, "failed to close response body")`)
		g.P(`    }`)
		g.P(`  }()`)
		g.P()
		g.P(`  if err = ctx.Err(); err != nil {`)
		g.P(`    return `, g.QualifiedGoIdent(blazePackage.Ident("ErrorInternalWith")), `(err, "aborted because context was done")`)
		g.P(`  }`)
		g.P()
		g.P(`  if resp.StatusCode != 200 {`)
		g.P(`    return `, g.QualifiedGoIdent(blazePackage.Ident("ErrorFromResponse")), `(resp)`)
		g.P(`  }`)
		g.P()
		g.P(`  respBodyBytes, err := `, g.QualifiedGoIdent(ioutilPackage.Ident("ReadAll")), `(resp.Body)`)
		g.P(`  if err != nil {`)
		g.P(`    return `, g.QualifiedGoIdent(blazePackage.Ident("ErrorInternalWith")), `(err, "failed to read response body")`)
		g.P(`  }`)
		g.P(`  if err = ctx.Err(); err != nil {`)
		g.P(`    return `, g.QualifiedGoIdent(blazePackage.Ident("ErrorInternalWith")), `(err, "aborted because context was done")`)
		g.P(`  }`)
		g.P()
		g.P(`  if err = `, g.QualifiedGoIdent(protoPackage.Ident("Unmarshal")), `(respBodyBytes, out); err != nil {`)
		g.P(`    return `, g.QualifiedGoIdent(blazePackage.Ident("ErrorInternalWith")), `(err, "failed to unmarshal proto response")`)
		g.P(`  }`)
		g.P(`  return nil`)
		g.P(`}`)
		g.P()
	}

	if name == "JSON" {
		g.P(`// doJSONRequest makes a JSON request to the remote Blaze service.`)
		g.P(`func (s *`, structName, `) doJSONRequest(ctx `, g.QualifiedGoIdent(contextPackage.Ident("Context")), `, client `, g.QualifiedGoIdent(blazePackage.Ident("HTTPClient")), `, url string, in, out `, g.QualifiedGoIdent(protoPackage.Ident("Message")), `) (err error) {`)
		g.P(`var buf []byte`)
		g.P(`marshaler := &`, g.QualifiedGoIdent(protoJSONPackage.Ident("MarshalOptions")), `{UseProtoNames: true}`)
		g.P(`if buf, err = marshaler.Marshal(in); err != nil {`)
		g.P(`    return `, g.QualifiedGoIdent(blazePackage.Ident("ErrorInternalWith")), `(err, "failed to marshal json request")`)
		g.P(`  }`)
		g.P(`  if err = ctx.Err(); err != nil {`)
		g.P(`    return `, g.QualifiedGoIdent(blazePackage.Ident("ErrorInternalWith")), `(err, "aborted because context was done")`)
		g.P(`  }`)
		g.P()
		g.P(`  req, err := `, g.QualifiedGoIdent(blazePackage.Ident("NewHTTPRequest")), `(ctx, url,`, g.QualifiedGoIdent(bytesPackage.Ident("NewReader")), `(buf), "application/json","`, s.version, `")`)
		g.P(`  if err != nil {`)
		g.P(`    return `, g.QualifiedGoIdent(blazePackage.Ident("ErrorInternalWith")), `(err, "could not build request")`)
		g.P(`  }`)
		g.P()
		g.P(`  ctx, req = s.trace.Inject(ctx, req)`)
		g.P(`  req = req.WithContext(ctx)`)
		g.P(`  resp, err := client.Do(req)`)
		g.P(`  if err != nil {`)
		g.P(`    return `, g.QualifiedGoIdent(blazePackage.Ident("ErrorInternalWith")), `(err, "failed to do request")`)
		g.P(`  }`)
		g.P()
		g.P(`  defer func() {`)
		g.P(`    cerr := resp.Body.Close()`)
		g.P(`    if err == nil && cerr != nil {`)
		g.P(`      err = `, g.QualifiedGoIdent(blazePackage.Ident("ErrorInternalWith")), `(cerr, "failed to close response body")`)
		g.P(`    }`)
		g.P(`  }()`)
		g.P()
		g.P(`  if err = ctx.Err(); err != nil {`)
		g.P(`    return `, g.QualifiedGoIdent(blazePackage.Ident("ErrorInternalWith")), `(err, "aborted because context was done")`)
		g.P(`  }`)
		g.P()
		g.P(`  if resp.StatusCode != 200 {`)
		g.P(`    return `, g.QualifiedGoIdent(blazePackage.Ident("ErrorFromResponse")), `(resp)`)
		g.P(`  }`)
		g.P()
		g.P(`respByte, err :=`, g.QualifiedGoIdent(ioutilPackage.Ident("ReadAll")), `(resp.Body)`)
		g.P(`if err != nil {`)
		g.P(`return `, g.QualifiedGoIdent(blazePackage.Ident("ErrorInternalWith")), `(err, "failed to read response body")`)
		g.P(`}`)
		g.P(`  unmarshaler := `, g.QualifiedGoIdent(protoJSONPackage.Ident("UnmarshalOptions")), `{DiscardUnknown: true}`)
		g.P(`  if err = unmarshaler.Unmarshal(respByte, out); err != nil {`)
		g.P(`    return `, g.QualifiedGoIdent(blazePackage.Ident("ErrorInternalWith")), `(err, "failed to unmarshal json response")`)
		g.P(`  }`)
		g.P(`  if err = ctx.Err(); err != nil {`)
		g.P(`    return `, g.QualifiedGoIdent(blazePackage.Ident("ErrorInternalWith")), `(err, "aborted because context was done")`)
		g.P(`  }`)
		g.P(`  return nil`)
		g.P(`}`)
	}
}

func (s *Blaze) generateServer(g *protogen.GeneratedFile, file *fileInfo, service *protogen.Service) {
	servName := service.GoName

	// Server implementation.
	servStruct := serviceStruct(service)
	g.P(`type `, servStruct, ` struct {`)
	g.P(`  `, servName)
	g.P(`  `, "log ", g.QualifiedGoIdent(logrPackage.Ident("Logger")))
	g.P(`  `, "mux *", g.QualifiedGoIdent(chiPackage.Ident("Mux")))
	g.P(`  `, "mountPath string")
	g.P(`     serviceTracer `, g.QualifiedGoIdent(blazetracePackage.Ident("ServiceTracer")))
	g.P(`     serviceOptions `, g.QualifiedGoIdent(blazePackage.Ident("ServiceOptions")))
	g.P(`}`)
	g.P()

	// Constructor for service implementation
	g.P(`func New`, servName, `Service(svc `, servName, `, log `, g.QualifiedGoIdent(logrPackage.Ident("Logger")), `, opts ...`, g.QualifiedGoIdent(blazePackage.Ident("ServiceOption")), `) `, g.QualifiedGoIdent(blazePackage.Ident("Service")), ` {`)
	g.P(`	serviceOptions := `, g.QualifiedGoIdent(blazePackage.Ident("ServiceOptions")), `{`)
	g.P(`   Trace: `, g.QualifiedGoIdent(blazetracePackage.Ident("NewServiceTracer")), `(),`)
	g.P(`   }`)
	g.P(`	for _, o := range opts {`)
	g.P(`		o(&serviceOptions)`)
	g.P(`	}`)
	g.P(`	var r *`, g.QualifiedGoIdent(chiPackage.Ident("Mux")))
	g.P(`	if serviceOptions.Mux != nil {`)
	g.P(`		r = serviceOptions.Mux`)
	g.P(`	} else {`)
	g.P(`		r = `, g.QualifiedGoIdent(chiPackage.Ident("NewRouter")), `()`)
	g.P(`	}`)
	g.P(``)
	g.P(`   service := `, servStruct, `{`)
	g.P(`       log:           log,`)
	g.P(`       mux:           r,`)
	g.P(`		serviceOptions: serviceOptions,`)
	g.P(`		mountPath:     `, servName, `PathPrefix,`)
	g.P(`   	serviceTracer:  serviceOptions.Trace,`)
	g.P(`       `, servName, `: svc,`)
	g.P(`}`)
	for _, method := range service.Methods {
		methName := "serve" + method.GoName
		g.P(`r.Post("/`, method.GoName, `", service.`, methName, `)`)
	}
	g.P(`return &service`)
	g.P(`}`)
	g.P()
	// Getting the mount path as package/version
	ss := strings.ReplaceAll(string(file.GoPackageName), "_", "/")
	g.P(`const `, servName, `PathPrefix = "/`, ss, `"`)

	g.P(`// Methods.`)
	for _, method := range service.Methods {
		s.generateServerMethod(g, service, method)
	}
}

func (s *Blaze) generateServerMethod(g *protogen.GeneratedFile, service *protogen.Service, method *protogen.Method) {
	methName := method.GoName
	servStruct := serviceStruct(service)
	servName := service.GoName
	g.P(`func (s *`, servStruct, `) serve`, methName, `(resp `, g.QualifiedGoIdent(httpPackage.Ident("ResponseWriter")), `, req *`, g.QualifiedGoIdent(httpPackage.Ident("Request")), `) {`)
	g.P(`  ctx, req, psd := s.serviceTracer.Extract(req)`)
	g.P(`  ctx, span := s.serviceTracer.StartSpan(ctx, "`, methName, `", psd,  `, g.QualifiedGoIdent(blazetracePackage.Ident("WithAttributes")), `( `, g.QualifiedGoIdent(blazetracePackage.Ident("ServiceName")), `.String("`, servName, `")))`)
	g.P(`  defer s.serviceTracer.EndSpan(span)`)
	g.P(`  header := req.Header.Get("Content-Type")`)
	g.P(`  i := `, g.QualifiedGoIdent(stringsPackage.Ident("Index")), `(header, ";")`)
	g.P(`  if i == -1 {`)
	g.P(`    i = len(header)`)
	g.P(`  }`)
	g.P(`  switch `, g.QualifiedGoIdent(stringsPackage.Ident("TrimSpace")), `(`, g.QualifiedGoIdent(stringsPackage.Ident("ToLower")), `(header[:i])) {`)
	g.P(`  case "application/json":`)
	g.P(`    s.serve`, methName, `JSON(ctx, resp, req)`)
	g.P(`  case "application/protobuf":`)
	g.P(`    s.serve`, methName, `Protobuf(ctx, resp, req)`)
	g.P(`  default:`)
	g.P(`    msg := `, g.QualifiedGoIdent(fmtPackage.Ident("Sprintf")), `("unexpected Content-Type: %q", req.Header.Get("Content-Type"))`)
	g.P(`    blerr := `, g.QualifiedGoIdent(blazePackage.Ident("ServerInvalidRequestError")), `("Content-Type",msg, req.Method, req.URL.Path)`)
	g.P(g.QualifiedGoIdent(blazePackage.Ident("ServerWriteError")), `(ctx, resp, blerr, s.log)`)
	g.P(`  }`)
	g.P(`}`)
	g.P()
	s.generateServerJSONMethod(g, service, method)
	s.generateServerProtobufMethod(g, service, method)
}

func (s *Blaze) generateServerJSONMethod(g *protogen.GeneratedFile, service *protogen.Service, method *protogen.Method) {
	methName := method.GoName
	servStruct := serviceStruct(service)
	servName := service.GoName
	g.P(`func (s *`, servStruct, `) serve`, methName, `JSON(ctx `, g.QualifiedGoIdent(contextPackage.Ident("Context")), `, resp `, g.QualifiedGoIdent(httpPackage.Ident("ResponseWriter")), `, req *`, g.QualifiedGoIdent(httpPackage.Ident("Request")), `) {`)
	g.P(`  var err error`)
	g.P()
	g.P(`  reqContent := new(`, g.QualifiedGoIdent(method.Input.GoIdent), `)`)
	g.P(``)
	g.P(`respByte, err := `, g.QualifiedGoIdent(ioutilPackage.Ident("ReadAll")), `(req.Body)`)
	g.P(`  if err != nil {`)
	g.P(`    `, g.QualifiedGoIdent(blazePackage.Ident("ServerWriteError")), `(ctx, resp, `, g.QualifiedGoIdent(blazePackage.Ident("ErrorMalformed")), `("the json request could not be decoded"), s.log)`)
	g.P(`    return`)
	g.P(`  }`)
	g.P(`  unmarshaler := `, g.QualifiedGoIdent(protoJSONPackage.Ident("UnmarshalOptions")), `{DiscardUnknown: true}`)
	g.P(`  if err = unmarshaler.Unmarshal(respByte, reqContent); err != nil {`)
	g.P(`    `, g.QualifiedGoIdent(blazePackage.Ident("ServerWriteError")), `(ctx, resp, `, g.QualifiedGoIdent(blazePackage.Ident("ErrorMalformed")), `("the json request could not be decoded"), s.log)`)
	g.P(`    return`)
	g.P(`  }`)
	g.P()
	g.P(`  // Call service method`)
	g.P(`  var respContent *`, g.QualifiedGoIdent(method.Output.GoIdent))
	g.P(`  func() {`)
	g.P(`    defer `, g.QualifiedGoIdent(blazePackage.Ident("ServerEnsurePanicResponses")), `(ctx, resp, s.log)`)
	g.P(`    respContent, err = s.`, servName, `.`, methName, `(ctx, reqContent)`)
	g.P(`  }()`)
	g.P()
	g.P(`  if err != nil {`)
	g.P(`    `, g.QualifiedGoIdent(blazePackage.Ident("ServerWriteError")), `(ctx, resp, err, s.log)`)
	g.P(`    return`)
	g.P(`  }`)
	g.P(`  if respContent == nil {`)
	g.P(`    `, g.QualifiedGoIdent(blazePackage.Ident("ServerWriteError")), `(ctx, resp, `, g.QualifiedGoIdent(blazePackage.Ident("ErrorInternal")), `("received a nil response. nil responses are not supported"), s.log)`)
	g.P(`    return`)
	g.P(`  }`)
	g.P()
	g.P()
	g.P(`  var buf []byte`)
	g.P(`  marshaler := &`, g.QualifiedGoIdent(protoJSONPackage.Ident("MarshalOptions")), `{`)
	g.P(`		UseProtoNames:    true,`)
	g.P(`		UseEnumNumbers: s.serviceOptions.JSONEnumsAsInts,`)
	g.P(`		EmitUnpopulated: s.serviceOptions.JSONEmitDefaults,`)
	g.P(`  }`)
	g.P(`  if buf, err = marshaler.Marshal(respContent); err != nil {`)
	g.P(`    `, g.QualifiedGoIdent(blazePackage.Ident("ServerWriteError")), `(ctx, resp, `, g.QualifiedGoIdent(blazePackage.Ident("ErrorInternalWith")), `(err, "failed to marshal json response"), s.log)`)
	g.P(`    return`)
	g.P(`  }`)
	g.P()
	g.P(`  resp.Header().Set("Content-Type", "application/json")`)
	g.P(`  resp.Header().Set("Content-Length",`, g.QualifiedGoIdent(strconvPackage.Ident("Itoa")), `(len(buf)))`)
	g.P(`  resp.WriteHeader(`, g.QualifiedGoIdent(httpPackage.Ident("StatusOK")), `)`)
	g.P()
	g.P(`  if n, err := resp.Write(buf); err != nil {`)
	g.P(`    msg := fmt.Sprintf("failed to write response, %d of %d bytes written: %s", n, len(buf), err.Error())`)
	g.P(`    blerr := `, g.QualifiedGoIdent(blazePackage.Ident("ErrorInternal")), `(msg)`)
	g.P(`    s.log.Error(blerr, msg)`)
	g.P(`  }`)
	g.P(`}`)
	g.P()
}
func (s *Blaze) generateServerProtobufMethod(g *protogen.GeneratedFile, service *protogen.Service, method *protogen.Method) {
	methName := method.GoName
	servStruct := serviceStruct(service)
	servName := service.GoName
	g.P(`func (s *`, servStruct, `) serve`, methName, `Protobuf(ctx `, g.QualifiedGoIdent(contextPackage.Ident("Context")), `, resp `, g.QualifiedGoIdent(httpPackage.Ident("ResponseWriter")), `, req *`, g.QualifiedGoIdent(httpPackage.Ident("Request")), `) {`)
	g.P(`  var err error`)
	g.P(`  if err != nil {`)
	g.P(`    `, g.QualifiedGoIdent(blazePackage.Ident("ServerWriteError")), `(ctx, resp, err, s.log)`)
	g.P(`    return`)
	g.P(`  }`)
	g.P()
	g.P(`  buf, err := `, g.QualifiedGoIdent(ioutilPackage.Ident("ReadAll")), `(req.Body)`)
	g.P(`  if err != nil {`)
	g.P(`    `, g.QualifiedGoIdent(blazePackage.Ident("ServerWriteError")), `(ctx, resp, `, g.QualifiedGoIdent(blazePackage.Ident("ErrorInternalWith")), `(err, "failed to read request body"), s.log)`)
	g.P(`    return`)
	g.P(`  }`)
	g.P(`  reqContent := new(`, g.QualifiedGoIdent(method.Input.GoIdent), `)`)
	g.P(`  if err = `, g.QualifiedGoIdent(protoPackage.Ident("Unmarshal")), `(buf, reqContent); err != nil {`)
	g.P(`    `, g.QualifiedGoIdent(blazePackage.Ident("ServerWriteError")), `(ctx, resp, `, g.QualifiedGoIdent(blazePackage.Ident("ErrorMalformed")), `("the protobuf request could not be decoded"), s.log)`)
	g.P(`    return`)
	g.P(`  }`)
	g.P()
	g.P(`  // Call service method`)
	g.P(`  var respContent *`, g.QualifiedGoIdent(method.Output.GoIdent))
	g.P(`  func() {`)
	g.P(`    defer  `, g.QualifiedGoIdent(blazePackage.Ident("ServerEnsurePanicResponses")), `(ctx, resp, s.log)`)
	g.P(`    respContent, err = s.`, servName, `.`, methName, `(ctx, reqContent)`)
	g.P(`  }()`)
	g.P()
	g.P(`  if err != nil {`)
	g.P(`    `, g.QualifiedGoIdent(blazePackage.Ident("ServerWriteError")), `(ctx, resp, err, s.log)`)
	g.P(`    return`)
	g.P(`  }`)
	g.P(`  if respContent == nil {`)
	g.P(`    `, g.QualifiedGoIdent(blazePackage.Ident("ServerWriteError")), `(ctx, resp, `, g.QualifiedGoIdent(blazePackage.Ident("ErrorInternal")), `("received a nil *`, g.QualifiedGoIdent(method.Output.GoIdent), ` and nil error while calling `, methName, `. nil responses are not supported"), s.log)`)
	g.P(`    return`)
	g.P(`  }`)
	g.P()
	g.P(`  respBytes, err := `, g.QualifiedGoIdent(protoPackage.Ident("Marshal")), `(respContent)`)
	g.P(`  if err != nil {`)
	g.P(`    `, g.QualifiedGoIdent(blazePackage.Ident("ServerWriteError")), `(ctx, resp, `, g.QualifiedGoIdent(blazePackage.Ident("ErrorInternalWith")), `(err, "failed to marshal proto response"), s.log)`)
	g.P(`    return`)
	g.P(`  }`)
	g.P()
	g.P(`  resp.Header().Set("Content-Type", "application/protobuf")`)
	g.P(`  resp.Header().Set("Content-Length", strconv.Itoa(len(respBytes)))`)
	g.P(`  resp.WriteHeader(`, g.QualifiedGoIdent(httpPackage.Ident("StatusOK")), `)`)
	g.P(`  if n, err := resp.Write(respBytes); err != nil {`)
	g.P(`    msg := fmt.Sprintf("failed to write response, %d of %d bytes written: %s", n, len(respBytes), err.Error())`)
	g.P(`    blerr := `, g.QualifiedGoIdent(blazePackage.Ident("ErrorInternal")), `(msg)`)
	g.P(`    s.log.Error(blerr, msg)`)
	g.P(`  }`)
	g.P(`}`)
	g.P()
}
func unexported(s string) string { return strings.ToLower(s[:1]) + s[1:] }

func serviceStruct(service *protogen.Service) string {
	return unexported(service.GoName) + "Service"
}
