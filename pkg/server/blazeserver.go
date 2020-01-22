package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"code.cestus.io/blaze"
	"github.com/go-chi/chi"
	"github.com/go-logr/logr"
)

// Option is a functional option for extending a Blaze server.
type Option func(*Options)

// Options encapsulate the configurable parameters on a Blaze server.
type Options struct {
	// Uses a specific mux instead of chi.NewRouter()
	Mux *chi.Mux
}

// WithMux allows to set the chi mux to use by a server
func WithMux(mux *chi.Mux) Option {
	return func(o *Options) {
		o.Mux = mux
	}
}

//BlazeServerBuilder is a Builder for blaze servers
type BlazeServerBuilder interface {
	//Add Middleware adds one or many middlewares to the server
	AddMiddleware(middlewares ...func(http.Handler) http.Handler)
	//AddServerMount Adds Mountpoints to the server
	AddServerMount(svm ...BlazeServiceMount)
	//Build creates the server
	Build() BlazeServer
}

//NewServerBuilder creates a new server Builder
func NewServerBuilder(listenAddr string, logger logr.Logger, opts ...Option) BlazeServerBuilder {
	serviceOptions := Options{}
	for _, o := range opts {
		o(&serviceOptions)
	}
	if serviceOptions.Mux == nil {
		serviceOptions.Mux = chi.NewRouter()
	}
	sb := blazeServerBuilder{
		serviceOptions: serviceOptions,
		log:            logger,
		listenAddr:     listenAddr,
	}
	return &sb
}

type blazeServerBuilder struct {
	middlewares    []func(http.Handler) http.Handler
	serverMounts   []BlazeServiceMount
	serviceOptions Options
	log            logr.Logger
	listenAddr     string
}

func (s *blazeServerBuilder) AddMiddleware(middlewares ...func(http.Handler) http.Handler) {
	s.middlewares = append(s.middlewares, middlewares...)
}

func (s *blazeServerBuilder) AddServerMount(svm ...BlazeServiceMount) {
	s.serverMounts = append(s.serverMounts, svm...)
}

func (s *blazeServerBuilder) Build() BlazeServer {
	r := s.serviceOptions.Mux
	r.Use(s.middlewares...)
	for _, bsm := range s.serverMounts {
		for _, mp := range bsm.Mounts() {
			r.Mount(mp+bsm.Service().MountPath(), bsm.Service().Mux())
		}
	}
	srv := http.Server{
		Addr:         s.listenAddr,
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	return &blazeServer{s.log, &srv}
}

//BlazeServiceMount defines a Service and a list of mountpoints e.g "/", "prefix"
type BlazeServiceMount interface {
	Service() blaze.Service
	Mounts() []string
}

type blazeServiceMount struct {
	service    blaze.Service
	mountPaths []string
}

func (s *blazeServiceMount) Service() blaze.Service {
	return s.service
}

func (s *blazeServiceMount) Mounts() []string {
	return s.mountPaths
}

//NewBlazeServiceMount creates a new ServiceMount
func NewBlazeServiceMount(srv blaze.Service, paths ...string) BlazeServiceMount {
	s := blazeServiceMount{
		service: srv,
	}
	s.mountPaths = append(s.mountPaths, paths...)
	return &s
}

//BlazeServer defines a blaze server
type BlazeServer interface {
	//Start starts a server which waits on close of the interrupt channel and modifies the wg
	Start(interrupt chan struct{}, wg *sync.WaitGroup)
	//Walk prints the registered routes
	Walk()
}
type blazeServer struct {
	l logr.Logger
	*http.Server
}

func (s *blazeServer) Start(interrupt chan struct{}, wg *sync.WaitGroup) {
	wg.Add(1)
	go func(interrupt chan struct{}, wg *sync.WaitGroup) {
		defer wg.Done()
		s.l.V(1).Info("Starting server", "addr", s.Addr)
		go func() {
			if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				s.l.Error(err, "Could not listen on", "addr", s.Addr)
			}
		}()
		select {
		case <-interrupt:
		}
		s.gracefullShutdown()
	}(interrupt, wg)
}

func (s *blazeServer) gracefullShutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s.SetKeepAlivesEnabled(false)
	if err := s.Shutdown(ctx); err != nil {
		s.l.Error(err, "Could not gracefully shutdown the server")
	}
	s.l.V(1).Info("Server stopped", "addr", s.Addr)
}

func (s *blazeServer) Walk() {
	log := s.l.WithValues("addr", s.Addr)
	walkFunc := func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		route = strings.Replace(route, "/*/", "/", -1)
		log.V(2).Info("Serving Route", "Route", fmt.Sprintf("%s %s", method, route))
		return nil
	}
	if r, ok := s.Handler.(*chi.Mux); ok {
		if err := chi.Walk(r, walkFunc); err != nil {
			log.Error(err, "Cannot walk route")
		}
	}
}

//NewInterruptNotifier returns a channel wich is closed on a received interrupt
func NewInterruptNotifier(log logr.Logger) chan struct{} {
	f := make(chan struct{})
	go func(notif chan struct{}) {
		quit := make(chan os.Signal, 1)

		signal.Notify(quit, os.Interrupt)
		signal.Notify(quit, syscall.SIGTERM)
		sig := <-quit
		log.V(1).Info("Shutting down", "reason", sig.String())
		close(notif)
	}(f)
	return f
}

//NewTerminatedNotifier creates a channel which can be used to listen to the termination of the server group
func NewTerminatedNotifier() chan struct{} {
	return make(chan struct{})
}

//BlazeServerGroup defines a BlazeServerGroup
type BlazeServerGroup interface {
	//Start starts all registered servers
	Start()
	//GetTerminationNotfier returns a channel which will be closed on termination of all servers
	GetTerminationNotfier() chan struct{}
	//Wait blocks until all servers are terminated
	Wait()
}

type blazeServerGroup struct {
	interrupt  chan struct{}
	terminated chan struct{}
	waitGroup  sync.WaitGroup
	servers    []BlazeServer
}

func (s *blazeServerGroup) Start() {
	for _, srv := range s.servers {
		srv.Start(s.interrupt, &s.waitGroup)
	}
}

func (s *blazeServerGroup) GetTerminationNotfier() chan struct{} {
	go func(t chan struct{}) {
		s.waitGroup.Wait()
		close(t)
	}(s.terminated)
	return s.terminated
}

func (s *blazeServerGroup) Wait() {
	select {
	case <-s.GetTerminationNotfier():
	}
}

//NewBlazeServerGroup creates a new server group
func NewBlazeServerGroup(interrupt chan struct{}, terminated chan struct{}, servers ...BlazeServer) BlazeServerGroup {
	sg := blazeServerGroup{
		interrupt:  interrupt,
		terminated: terminated,
	}
	sg.servers = append(sg.servers, servers...)
	return &sg
}

//NewServerGroupFromBuilders creates a server group from builders as a convenience function
func NewServerGroupFromBuilders(interrupt chan struct{}, terminated chan struct{}, printRoutes bool, builder ...BlazeServerBuilder) BlazeServerGroup {
	sg := blazeServerGroup{
		interrupt:  interrupt,
		terminated: terminated,
	}
	for _, b := range builder {
		svr := b.Build()
		if printRoutes {
			svr.Walk()
		}
		sg.servers = append(sg.servers, svr)
	}
	return &sg
}
