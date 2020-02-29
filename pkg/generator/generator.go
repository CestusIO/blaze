package generator

import (
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
)

// Generator is the generator interface
type Generator interface {
	Generate(in *plugin.CodeGeneratorRequest) *plugin.CodeGeneratorResponse
}

//Generate is the entrypoint for the application
func Generate(g Generator) {
	req := readGenRequest(os.Stdin)
	resp := g.Generate(req)
	writeResponse(os.Stdout, resp)
}

// FilesToGenerate parses the files from a CodeGenerationReqest
func FilesToGenerate(req *plugin.CodeGeneratorRequest) []*descriptor.FileDescriptorProto {
	genFiles := make([]*descriptor.FileDescriptorProto, 0)
Outer:
	for _, name := range req.FileToGenerate {
		for _, f := range req.ProtoFile {
			if f.GetName() == name {
				genFiles = append(genFiles, f)
				continue Outer
			}
		}
		lfail("could not find file named", name)
	}

	return genFiles
}

func readGenRequest(r io.Reader) *plugin.CodeGeneratorRequest {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		lerror(err, "reading input")
	}

	req := new(plugin.CodeGeneratorRequest)
	if err = proto.Unmarshal(data, req); err != nil {
		lerror(err, "parsing input proto")
	}

	if len(req.FileToGenerate) == 0 {
		lfail("no files to generate")
	}

	return req
}

func writeResponse(w io.Writer, resp *plugin.CodeGeneratorResponse) {
	data, err := proto.Marshal(resp)
	if err != nil {
		lerror(err, "marshaling response")
	}
	_, err = w.Write(data)
	if err != nil {
		lerror(err, "writing response")
	}
}

func lfail(msgs ...string) {
	s := strings.Join(msgs, " ")
	log.Print("error:", s)
	os.Exit(1)
}

func lerror(err error, msgs ...string) {
	s := strings.Join(msgs, " ") + ":" + err.Error()
	log.Print("error:", s)
	os.Exit(1)
}
