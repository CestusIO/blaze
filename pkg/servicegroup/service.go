package servicegroup

/*  A service manager
example usage:
With a service like:
type sv1 struct {
	name         string
	shutdownTime time.Duration
}

func (s *sv1) Start() error {
	return nil
}
func (s *sv1) Shutdown(ctx context.Context) error {
	time.Sleep(s.shutdownTime)
	return nil
}
func (s *sv1) String() string {
	return s.name
}

//NewService1 some comment
func NewService1(name string, shutdown time.Duration) servicegroup.Service {
	s := &sv1{
		name:         name,
		shutdownTime: shutdown,
	}
	return s
}
You can use:
sv1 := services.NewService1("sv1", 5*time.Second)
	sv2 := services.NewService1("sv2", 5*time.Second)
	sv1t := servicegroup.NewTerminatedNotifier()
	sv2t := servicegroup.NewTerminatedNotifier()
	msv1 := servicegroup.NewManagedService(log, sv1, sv1t, nil)
	msv2 := servicegroup.NewManagedService(log, sv2, sv2t, sv1t)
	//msgt := servicegroup.NewTerminatedNotifier()
	msg := servicegroup.NewManagedServiceGroup(interrupt, nil , msv1, msv2)
	msg.Start()
	msg.Wait()

Services are created with a dependency. sv2 waits for sv1 to shutdown.
Shutdown of the group starts when interrupt gets closed
*/
import (
	"context"
	"sync"
	"time"

	"github.com/go-logr/logr"
)

//Service defines a Service which can be Started and Shutdown
type Service interface {
	Start() error
	Shutdown(ctx context.Context) error
	String() string
}

//NewTerminatedNotifier creates a channel which can be used to listen to its being closed
func NewTerminatedNotifier() chan struct{} {
	return make(chan struct{})
}

//ManagedService is a service which can be lifecycle managed
type ManagedService interface {
	//Start starts a server which waits on close of the interrupt channel and modifies the wg
	Start(interrupt chan struct{}, wg *sync.WaitGroup)
	//GetTerminationNotifier returns a channel which will be closed on termination of the service
	GetTerminationNotifier() chan struct{}

	//GetInterrupt returns a interrupt channel set during construction
	GetInterrupt() chan struct{}
}

type managedService struct {
	l          logr.Logger
	service    Service
	terminated chan struct{}
	interrupt  chan struct{}
}

func (s *managedService) Start(interrupt chan struct{}, wg *sync.WaitGroup) {
	wg.Add(1)
	go func(interrupt chan struct{}, wg *sync.WaitGroup) {
		defer wg.Done()
		s.l.V(1).Info("Starting service", "ident", s.service.String())
		go func() {
			if err := s.service.Start(); err != nil {
				s.l.Error(err, "Service failed", "ident", s.service.String())
			}
		}()
		select {
		case <-interrupt:
		}
		s.gracefullShutdown()
		close(s.terminated)
	}(interrupt, wg)
}

func (s *managedService) GetTerminationNotifier() chan struct{} {
	return s.terminated
}

func (s *managedService) GetInterrupt() chan struct{} {
	return s.interrupt
}

func (s *managedService) gracefullShutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.service.Shutdown(ctx); err != nil {
		s.l.Error(err, "Could not gracefully shutdown the service", "ident", s.service.String())
	}
	s.l.V(1).Info("Service stopped", "ident", s.service.String())
}

//NewManagedService creates a managed service with an optional interrupt channel
func NewManagedService(log logr.Logger, service Service, terminated chan struct{}, interrupt chan struct{}) ManagedService {
	ms := &managedService{
		l:          log,
		service:    service,
		terminated: terminated,
		interrupt:  interrupt,
	}
	if ms.terminated == nil {
		ms.terminated = make(chan struct{})
	}
	return ms
}

//ManagedServiceGroup defines a ManagedServiceGroup
type ManagedServiceGroup interface {
	//Start starts all registered services
	Start()
	//GetTerminationNotfier returns a channel which will be closed on termination of all services
	GetTerminationNotfier() chan struct{}
	//Wait blocks until all services are terminated
	Wait()
}

type managedServiceGroup struct {
	interrupt  chan struct{}
	terminated chan struct{}
	waitGroup  sync.WaitGroup
	services   []ManagedService
}

//NewManagedServiceGroup creates a new service group
func NewManagedServiceGroup(interrupt chan struct{}, terminated chan struct{}, services ...ManagedService) ManagedServiceGroup {
	sg := &managedServiceGroup{
		interrupt:  interrupt,
		terminated: terminated,
	}
	sg.services = append(sg.services, services...)
	if sg.terminated == nil {
		sg.terminated = make(chan struct{})
	}
	return sg
}
func (s *managedServiceGroup) Start() {
	for _, srv := range s.services {
		if srv.GetInterrupt() == nil {
			srv.Start(s.interrupt, &s.waitGroup)
		} else {
			srv.Start(srv.GetInterrupt(), &s.waitGroup)
		}
	}
	go func(t chan struct{}) {
		s.waitGroup.Wait()
		close(t)
	}(s.terminated)
}

func (s *managedServiceGroup) GetTerminationNotfier() chan struct{} {
	return s.terminated
}

func (s *managedServiceGroup) Wait() {
	select {
	case <-s.GetTerminationNotfier():
	}
}
