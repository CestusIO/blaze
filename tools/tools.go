// +build tools

package tools

import (
	_ "github.com/google/wire/cmd/wire"
	_ "github.com/matryer/moq"
	_ "github.com/myitcv/gobin"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
)
