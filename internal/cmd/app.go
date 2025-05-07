// Copyright 2025 Metrostar Systems, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"errors"
	"sync"

	"github.com/urfave/cli/v3"
	"go.uber.org/fx"
)

// AppService represents the main application service that manages the CLI command execution
// and the lifecycle of the application.
type AppService struct {
	app *cli.Command   // The CLI command to execute.
	sd  fx.Shutdowner  // The shutdown mechanism for the application.
	wg  sync.WaitGroup // WaitGroup to manage goroutines.
	err error          // Stores any error encountered during execution.
}

// NewAppService creates a new instance of AppService with the provided CLI command and shutdowner.
func NewAppService(app *cli.Command, sd fx.Shutdowner) *AppService {
	return &AppService{
		app: app,
		sd:  sd,
	}
}

// Start begins the execution of the CLI application. It manages the lifecycle of the application
// and listens for shutdown signals or errors during execution.
func (svc *AppService) Start(args []string) error {
	done := make(chan error)                                // Channel to signal when the application is done.
	ctx, cancel := context.WithCancel(context.Background()) // Context to manage cancellation.

	svc.wg.Add(1)
	go func() {
		defer svc.wg.Done()

		for {
			select {
			case <-ctx.Done():
				err := ctx.Err()
				// Handle context cancellation.
				if !errors.Is(err, context.Canceled) {
					svc.err = err
				}
				if err = svc.sd.Shutdown(); err != nil {
					svc.err = errors.Join(svc.err, err)
				}
				return
			case err := <-done:
				if err != nil {
					// Handle error during execution and exit with a non-zero code.
					svc.err = err
					if err = svc.sd.Shutdown(fx.ExitCode(1)); err != nil {
						svc.err = errors.Join(svc.err, err)
					}
					return
				}

				// No error, trigger normal shutdown.
				cancel()
			}
		}
	}()

	svc.wg.Add(1)
	go func() {
		defer svc.wg.Done()
		defer close(done)
		defer cancel()

		if err := svc.app.Run(ctx, args); err != nil {
			done <- err
		}
	}()

	return nil
}

// Stop waits for all goroutines to finish and returns any error encountered during execution.
func (svc *AppService) Stop() error {
	svc.wg.Wait()

	if svc.err != nil {
		return svc.err
	}

	return nil
}
