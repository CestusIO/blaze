// Code generated by fabricator-generate-tool-go
//
// Modifications in code regions will be lost during regeneration!

package blaze

// region CODE_REGION(version)
import (
	_ "embed"

	"code.cestus.io/libs/buildinfo"
)

//go:embed version.yml
var version string

func init() {
	buildinfo.GenerateVersionFromVersionYaml(GetVersionYaml(), "protoc-gen-blaze")
}

func GetVersionYaml() []byte {
	return []byte(version)
}

//endregion
