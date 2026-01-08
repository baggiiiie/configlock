package service

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/kardianos/service"
)

type program struct{}

func (p *program) Start(s service.Service) error {
	// Service is starting
	return nil
}

func (p *program) Stop(s service.Service) error {
	// Service is stopping
	return nil
}

// Service represents the configlock service
type Service struct {
	svc service.Service
}

// New creates a new service instance
func New() (*Service, error) {
	// Get the path to the current executable
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	svcConfig := &service.Config{
		Name:        "configlock",
		DisplayName: "ConfigLock Daemon",
		Description: "Enforces file locking during work hours to prevent impulsive config editing",
		Executable:  execPath,
		Arguments:   []string{"daemon"},
		Option: service.KeyValue{
			// Install as user service (not system-wide)
			"UserService": true,
		},
	}

	prg := &program{}
	svc, err := service.New(prg, svcConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create service: %w", err)
	}

	return &Service{svc: svc}, nil
}

// Install installs the service
func (s *Service) Install() error {
	// Check if already installed
	status, err := s.svc.Status()
	if err == nil && status != service.StatusUnknown {
		// Already installed, uninstall first
		if err := s.svc.Uninstall(); err != nil {
			return fmt.Errorf("failed to uninstall existing service: %w", err)
		}
	}

	if err := s.svc.Install(); err != nil {
		return fmt.Errorf("failed to install service: %w", err)
	}

	// On Linux, reload systemd daemon
	if runtime.GOOS == "linux" {
		cmd := exec.Command("systemctl", "daemon-reload")
		if err := cmd.Run(); err != nil {
			// Non-fatal, continue
			fmt.Printf("Warning: failed to reload systemd daemon: %v\n", err)
		}
	}

	return nil
}

// Uninstall uninstalls the service
func (s *Service) Uninstall() error {
	// Stop first
	s.Stop()

	if err := s.svc.Uninstall(); err != nil {
		return fmt.Errorf("failed to uninstall service: %w", err)
	}

	return nil
}

// Start starts the service
func (s *Service) Start() error {
	if err := s.svc.Start(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	return nil
}

// Stop stops the service
func (s *Service) Stop() error {
	if err := s.svc.Stop(); err != nil {
		// Ignore errors if already stopped
		return nil
	}

	return nil
}

// Restart restarts the service
func (s *Service) Restart() error {
	if err := s.svc.Restart(); err != nil {
		return fmt.Errorf("failed to restart service: %w", err)
	}

	return nil
}

// Status returns the service status
func (s *Service) Status() (service.Status, error) {
	return s.svc.Status()
}
