package application

import (
	"context"
	"errors"
	"time"
)

// Close shuts down application resources with a bounded background context and
// joins any shutdown error into the caller's result.
func Close(closeApp func(context.Context) error, result *error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if closeErr := closeApp(ctx); closeErr != nil {
		*result = errors.Join(*result, closeErr)
	}
}
