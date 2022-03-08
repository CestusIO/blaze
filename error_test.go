package blaze_test

import (
	"errors"
	"fmt"
	"sync"

	. "github.com/onsi/ginkgo/v2"
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
			err := blaze.ErrorCanceled("msg")
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
			err := blaze.ErrorCanceled("msg")
			_, ok := err.(blaze.Error)
			Expect(ok).To(BeTrue())
			Expect(errors.Is(err, blaze.ErrorCanceled("msg")))
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
				blaze.ErrorCanceled(""), 408),
			Entry("InvalidArgumentErrorType",
				blaze.ErrorInvalidArgument("", ""), 400),
			Entry("MalformedErrorType",
				blaze.ErrorMalformed(""), 400),
			Entry("DeadlineExceededErrorType",
				blaze.ErrorDeadlineExeeded("msg"), 408),
			Entry("NotFoundErrorType:",
				blaze.ErrorNotFound(""), 404),
			Entry("BadRouteErrorType:",
				blaze.ErrorBadRoute(""), 404),
			Entry("AlreadyExistsErrorType:",
				blaze.ErrorAlreadyExists(""), 409),
			Entry("PermissionDeniedErrorType",
				blaze.ErrorPermissionDenied(""), 403),
			Entry("UnauthenticatedErrorType",
				blaze.ErrorUnauthenticated(""), 401),
			Entry("ResourceExhaustedErrorType",
				blaze.ErrorResourceExhausted(""), 429),
			Entry("FailedPreconditionErrorType",
				blaze.ErrorFailedPrecondition(""), 412),
			Entry("AbortedErrorType",
				blaze.ErrorAborted(""), 409),
			Entry("OutOfRangeErrorType",
				blaze.ErrorOutOfRange(""), 400),
			Entry("UnimplementedErrorType",
				blaze.ErrorUnimplemented(""), 501),
			Entry("InternalErrorType",
				blaze.ErrorInternal(""), 500),
			Entry("InternalErrorType",
				blaze.ErrorInternalWith(errors.New("msg"), ""), 500),
			Entry("UnavailableErrorType",
				blaze.ErrorUnavailable(""), 503),
			Entry("DataLossErrorType",
				blaze.ErrorDataLoss(""), 500),
		)
	})
	Context("ErrorRegistry", func() {
		It("can construct objects", func() {
			oe := blaze.ErrorRequiredArgument("arg")
			se, err := blaze.ErrorToErrorJSON(oe)
			ej := blaze.ErrorJSON{
				Code: "400",
				Msg:  "arg is_required",
				Type: "*blaze.InvalidArgumentErrorType",
				Meta: map[string]string{"argument": "arg"},
			}
			Expect(err).To(BeNil())
			Expect(se).To(Equal(ej))
			ue, err := blaze.ErrorJSONToError(ej)
			Expect(err).To(BeNil())
			Expect(ue).To(Equal(oe))
		})
		It("can construct Internal error with", func() {
			oe := blaze.ErrorInternalWith(errors.New("internal error"), "arg")
			se, err := blaze.ErrorToErrorJSON(oe)
			ej := blaze.ErrorJSON{
				Code: "500",
				Msg:  "arg",
				Type: "*blaze.InternalErrorType",
				Meta: map[string]string{"wrappedInternalError": "internal error"},
			}
			Expect(err).To(BeNil())
			Expect(se).To(Equal(ej))
			ue, err := blaze.ErrorJSONToError(ej)
			Expect(err).To(BeNil())
			Expect(ue).To(Equal(oe))
		})
		var _ = DescribeTable("Error Serialisation ",
			func(e blaze.Error) {
				se, err := blaze.ErrorToErrorJSON(e)
				Expect(err).To(BeNil())
				ue, err := blaze.ErrorJSONToError(se)
				Expect(err).To(BeNil())
				Expect(ue).To(Equal(e))
			},
			Entry("CanceledErrorType",
				blaze.ErrorCanceled("msg")),
			Entry("InvalidArgumentErrorType",
				blaze.ErrorInvalidArgument("arg", "msg")),
			Entry("MalformedErrorType",
				blaze.ErrorMalformed("msg")),
			Entry("DeadlineExceededErrorType",
				blaze.ErrorDeadlineExeeded("msg")),
			Entry("NotFoundErrorType:",
				blaze.ErrorNotFound("msg")),
			Entry("BadRouteErrorType:",
				blaze.ErrorBadRoute("msg")),
			Entry("AlreadyExistsErrorType:",
				blaze.ErrorAlreadyExists("msg")),
			Entry("PermissionDeniedErrorType",
				blaze.ErrorPermissionDenied("msg")),
			Entry("UnauthenticatedErrorType",
				blaze.ErrorUnauthenticated("msg")),
			Entry("ResourceExhaustedErrorType",
				blaze.ErrorResourceExhausted("msg")),
			Entry("FailedPreconditionErrorType",
				blaze.ErrorFailedPrecondition("msg")),
			Entry("AbortedErrorType",
				blaze.ErrorAborted("msg")),
			Entry("OutOfRangeErrorType",
				blaze.ErrorOutOfRange("msg")),
			Entry("UnimplementedErrorType",
				blaze.ErrorUnimplemented("msg")),
			Entry("InternalErrorType",
				blaze.ErrorInternal("msg")),
			Entry("InternalErrorType With",
				blaze.ErrorInternalWith(errors.New("msg"), "msg")),
			Entry("UnavailableErrorType",
				blaze.ErrorUnavailable("msg")),
			Entry("DataLossErrorType",
				blaze.ErrorDataLoss("msg")),
		)
	})
})
