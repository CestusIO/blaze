package blaze_test

import (
	"errors"
	"fmt"
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"code.cestus.io/blaze"
	//. "code.cestus.io/blaze"
)

var _ = Describe("Error", func() {
	Context("WithMeta", func() {
		It("does not mutate the map", func() {
			err := blaze.NewError(nil, "msg")
			err = err.WithMeta("k1", "v1")

			var wg sync.WaitGroup
			for i := 0; i < 1000; i++ {
				wg.Add(1)
				go func(i int) {
					_ = err.WithMeta(fmt.Sprintf("k-%d", i), "v")
					wg.Done()
				}(i)
			}

			wg.Wait()
			Expect(len(err.MetaMap())).To(Equal(1))
		})
	})
	Context("GetMeta", func() {
		It("does not allow to mutate the map", func() {
			err := blaze.NewError(nil, "msg")
			err = err.WithMeta("k1", "v1")
			Expect(err.Meta("k1")).To(Equal("v1"))
			mm := err.MetaMap()
			mm["k1"] = "v2"
			Expect(err.Meta("k1")).To(Equal("v1"))
		})
	})

	Context("Validating assumptions about go 1.13 errors", func() {
		Specify("As() only returns true when it is of the correct type", func() {
			err := blaze.ErrorCanceled()
			_, ok := err.(blaze.Error)
			Expect(ok).To(BeTrue())
			var e *blaze.CanceledErrorType
			Expect(errors.As(err, &e)).To(BeTrue())
			var e2 blaze.Error
			Expect(errors.As(err, &e2)).To(BeTrue())
			var e3 *blaze.InternalErrorType
			Expect(errors.As(err, &e3)).To(BeFalse())
		})
		Specify("Is() only returns true when it is an value wise equal error", func() {
			err := blaze.ErrorCanceled()
			_, ok := err.(blaze.Error)
			Expect(ok).To(BeTrue())
			Expect(errors.Is(err, blaze.ErrorCanceled()))
		})
	})
	Context("Internal", func() {
		Specify("errors.Is() returns true on equivalent errors", func() {
			err := blaze.ErrorInternal("")
			Expect(errors.Is(err, blaze.ErrorInternal(""))).To(BeTrue())
		})
		Specify("errors.Is returns false on not equivalent errors", func() {
			err := blaze.ErrorInternalWith(errors.New("an error"), "")
			Expect(errors.Is(err, blaze.ErrorInternalWith(errors.New("an error"), ""))).To(BeTrue())
			err2 := blaze.ErrorInternalWith(errors.New("an error"), "")
			Expect(errors.Is(err2, blaze.ErrorInternalWith(errors.New("an error2"), ""))).To(BeFalse())
		})
	})
	Context("ServerHTTPStatusFromErrorType", func() {
		var _ = DescribeTable("Error translation ",
			func(err error, code int) {
				tr := blaze.ServerHTTPStatusFromErrorType(err)
				Expect(tr).To(Equal(code))
			},
			Entry("CanceledErrorType",
				blaze.ErrorCanceled(), 408),
			Entry("InvalidArgumentErrorType",
				blaze.ErrorInvalidArgument("", ""), 400),
			Entry("MalformedErrorType",
				blaze.ErrorMalformed(""), 400),
			Entry("DeadlineExceededErrorType",
				blaze.ErrorDeadlineExeeded(), 408),
			Entry("NotFoundErrorType:",
				blaze.ErrorNotFound(), 404),
			Entry("BadRouteErrorType:",
				blaze.ErrorBadRoute(""), 404),
			Entry("AlreadyExistsErrorType:",
				blaze.ErrorAlreadyExists(), 409),
			Entry("PermissionDeniedErrorType",
				blaze.ErrorPermissionDenied(), 403),
			Entry("UnauthenticatedErrorType",
				blaze.ErrorUnauthenticated(), 401),
			Entry("ResourceExhaustedErrorType",
				blaze.ErrorResourceExhausted(), 403),
			Entry("FailedPreconditionErrorType",
				blaze.ErrorFailedPrecondition(), 412),
			Entry("AbortedErrorType",
				blaze.ErrorAborted(), 409),
			Entry("OutOfRangeErrorType",
				blaze.ErrorOutOfRange(), 400),
			Entry("UnimplementedErrorType",
				blaze.ErrorUnimplemented(), 501),
			Entry("InternalErrorType",
				blaze.ErrorInternal(""), 500),
			Entry("InternalErrorType",
				blaze.ErrorInternalWith(errors.New("msg"), ""), 500),
			Entry("UnavailableErrorType",
				blaze.ErrorUnavailable(), 503),
			Entry("DataLossErrorType",
				blaze.ErrorDataLoss(), 500),
		)
	})
	// It("Returns the correct cause", func() {
	// 	rootCause := fmt.Errorf("this is only a test")
	// 	blerr := blaze.InternalErrorWith(rootCause)
	// 	cause := errors.Cause(blerr)
	// 	Expect(cause).To(Equal(rootCause))
	// })
})
