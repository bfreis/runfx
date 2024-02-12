package runfx

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"go.uber.org/fx"
)

type FxOpts interface {
	Fx() fx.Option
}

// RunAndExit runs the application and exits with the appropriate exit code.
// It does not return.
// The given context is used to start and stop the application.
func RunAndExit(ctx context.Context, fxOpts FxOpts) {
	err := Run(ctx, fxOpts)
	if err != nil {
		// retrieve exit code from err
		var exitErr ExitError
		if ok := errors.As(err, &exitErr); ok {
			log.Printf("exit: code=%d signal=%s", exitErr.ExitCode, exitErr.Signal)
			os.Exit(exitErr.ExitCode)
		}

		log.Fatal(err)
	}

	os.Exit(0)
}

// Run runs the application and returns an error indicating failure in
// any of the steps: settings defaults, validation, starting the fx app,
// stopping the fx app, or any exit code from the fx app.
// The given context is used to start and stop the application.
func Run(ctx context.Context, fxOpts FxOpts) error {
	if defSetter, ok := fxOpts.(SetDefaulter); ok {
		err := defSetter.SetDefaults()
		if err != nil {
			return fmt.Errorf("set defaults: %w", err)
		}
	}

	if validator, ok := fxOpts.(Validator); ok {
		err := validator.Validate()
		if err != nil {
			return fmt.Errorf("validate: %w", err)
		}
	}

	fxApp := fx.New(fxOpts.Fx())
	if fxApp.Err() != nil {
		return fmt.Errorf("fx.New: %w", fxApp.Err())
	}

	startCtx, startCancel := context.WithTimeout(ctx, fxApp.StartTimeout())
	defer startCancel()

	err := fxApp.Start(startCtx)
	if err != nil {
		return fmt.Errorf("fx.Start: %w", err)
	}

	sig := <-fxApp.Wait()

	stopCtx, stopCancel := context.WithTimeout(ctx, fxApp.StopTimeout())
	defer stopCancel()

	err = fxApp.Stop(stopCtx)
	if err != nil {
		return fmt.Errorf("fx.Stop: %w", err)
	}

	if sig.ExitCode != 0 {
		return ExitError{
			ExitCode: sig.ExitCode,
			Signal:   sig.Signal,
		}
	}

	return nil
}

// SetDefaulter is an interface that can be implemented by the FxOpts
// to set default values. This is called before the application is started.
type SetDefaulter interface {
	SetDefaults() error
}

// Validator is an interface that can be implemented by the FxOpts
// to validate the configuration. This is called before the application is started,
// after the defaults are set.
type Validator interface {
	Validate() error
}

// ExitError is an error type that indicates the application exited with a non-zero exit code.
// The ExitCode is the exit code of the application and Signal is the signal that caused the application to exit.
type ExitError struct {
	ExitCode int
	Signal   os.Signal
}

func (e ExitError) Error() string {
	return fmt.Sprintf("exit: code=%d signal=%s", e.ExitCode, e.Signal)
}
