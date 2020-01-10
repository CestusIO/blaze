package main

import (
	"errors"
	"reflect"
	"testing"
)

func TestParseCommandLineParams(t *testing.T) {
	tests := []struct {
		name      string
		parameter string
		params    *commandLineParams
		err       error
	}{
		{
			"no parameters",
			"",
			&commandLineParams{
				importMap: map[string]string{},
			},
			nil,
		},
		{
			"unknown parameter",
			"k=v",
			nil,
			errors.New(`unknown parameter "k"`),
		},
		{
			"empty parameter value - no equals sign",
			"import_prefix",
			nil,
			errors.New(`invalid parameter "import_prefix": expected format of parameter to be k=v`),
		},
		{
			"empty parameter value - no value",
			"import_prefix=",
			nil,
			errors.New(`invalid parameter "import_prefix": expected format of parameter to be k=v`),
		},
		{
			"import_prefix parameter",
			"import_prefix=github.com/example/repo",
			&commandLineParams{
				importMap:    map[string]string{},
				importPrefix: "github.com/example/repo",
			},
			nil,
		},
		{
			"single import parameter starting with 'M'",
			"Mrpcutil/empty.proto=github.com/example/rpcutil",
			&commandLineParams{
				importMap: map[string]string{
					"rpcutil/empty.proto": "github.com/example/rpcutil",
				},
			},
			nil,
		},
		{
			"multiple import parameters starting with 'M'",
			"Mrpcutil/empty.proto=github.com/example/rpcutil,Mrpc/haberdasher/service.proto=github.com/example/rpc/haberdasher",
			&commandLineParams{
				importMap: map[string]string{
					"rpcutil/empty.proto":           "github.com/example/rpcutil",
					"rpc/haberdasher/service.proto": "github.com/example/rpc/haberdasher",
				},
			},
			nil,
		},
		{
			"single import parameter starting with 'go_import_mapping@'",
			"go_import_mapping@rpcutil/empty.proto=github.com/example/rpcutil",
			&commandLineParams{
				importMap: map[string]string{
					"rpcutil/empty.proto": "github.com/example/rpcutil",
				},
			},
			nil,
		},
		{
			"multiple import parameters starting with 'go_import_mapping@'",
			"go_import_mapping@rpcutil/empty.proto=github.com/example/rpcutil,go_import_mapping@rpc/haberdasher/service.proto=github.com/example/rpc/haberdasher",
			&commandLineParams{
				importMap: map[string]string{
					"rpcutil/empty.proto":           "github.com/example/rpcutil",
					"rpc/haberdasher/service.proto": "github.com/example/rpc/haberdasher",
				},
			},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, err := parseCommandLineParams(tt.parameter)
			switch {
			case err != nil:
				if tt.err == nil {
					t.Fatal(err)
				}
				if err.Error() != tt.err.Error() {
					t.Errorf("got error = %v, want %v", err, tt.err)
				}
			case err == nil:
				if tt.err != nil {
					t.Errorf("got error = %v, want %v", err, tt.err)
				}
			}
			if !reflect.DeepEqual(params, tt.params) {
				t.Errorf("got params = %v, want %v", params, tt.params)
			}
		})
	}
}
