module code.cestus.io/blaze

go 1.13

require (
	github.com/go-chi/chi v4.0.2+incompatible
	github.com/go-logr/logr v0.1.0
	github.com/go-logr/zapr v0.1.2-0.20191218045755-6759ef05ca25
	github.com/golang/protobuf v1.3.2
	github.com/onsi/ginkgo v1.10.3
	github.com/onsi/gomega v1.7.1
	github.com/pkg/errors v0.8.1
	github.com/stretchr/testify v1.4.0
	go.uber.org/zap v1.13.0
	golang.org/x/net v0.0.0-20191105084925-a882066a44e0 // indirect
	golang.org/x/sys v0.0.0-20191105231009-c1f44814a5cd // indirect
	golang.org/x/text v0.3.2 // indirect
	gopkg.in/yaml.v2 v2.2.5 // indirect
)

replace github.com/go-logr/zapr => github.com/magicmoose/zapr v0.1.2-0.20191219183849-db2ecee9d58a
