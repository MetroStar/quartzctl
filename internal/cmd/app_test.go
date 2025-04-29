package cmd

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
	"go.uber.org/fx"
)

// MockShutdowner is a mock implementation of fx.Shutdowner for testing purposes.
// It uses testify's mock package to simulate behavior and validate interactions.
type MockShutdowner struct {
	mock.Mock
}

func (m *MockShutdowner) Shutdown(exitCode ...fx.ShutdownOption) error {
	args := m.Called(exitCode)
	return args.Error(0)
}

// TestAppService_StartAndStop tests the Start and Stop methods of AppService.
// It ensures that the application starts and stops without errors and that the shutdown mechanism is invoked.
func TestAppService_StartAndStop(t *testing.T) {
	mockShutdowner := new(MockShutdowner)
	mockShutdowner.On("Shutdown", mock.Anything).Return(nil)

	app := &cli.Command{
		Name: "test-app",
		Action: func(ctx context.Context, ccmd *cli.Command) error {
			return nil
		},
	}

	svc := NewAppService(app, mockShutdowner)

	// Test Start
	err := svc.Start(nil)
	assert.NoError(t, err, "Start should not return an error")

	// Simulate app.Run completion
	svc.wg.Wait()

	// Test Stop
	err = svc.Stop()
	assert.NoError(t, err, "Stop should not return an error")

	mockShutdowner.AssertCalled(t, "Shutdown", mock.Anything)
}

// TestAppService_StartWithError tests the Start method of AppService when the CLI command returns an error.
// It ensures that the error is propagated correctly and the shutdown mechanism is invoked with an error exit code.
func TestAppService_StartWithError(t *testing.T) {
	mockShutdowner := new(MockShutdowner)
	mockShutdowner.On("Shutdown", mock.Anything).Return(nil)

	app := &cli.Command{
		Name: "test-app",
		Action: func(ctx context.Context, ccmd *cli.Command) error {
			return fmt.Errorf("simulated error")
		},
	}

	svc := NewAppService(app, mockShutdowner)

	// Test Start
	err := svc.Start(nil)
	assert.NoError(t, err, "Start should not return an error")

	// Test Stop
	err = svc.Stop()
	require.Error(t, err, "Stop should return an error")
	require.Equal(t, "simulated error", err.Error(), "Error message should match")

	mockShutdowner.AssertCalled(t, "Shutdown", []fx.ShutdownOption{fx.ExitCode(1)})
}
