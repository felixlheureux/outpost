package githubcli

import (
	"context"
	"errors"
	"time"

	"github.com/felixlheureux/outpost/internal/hostcmd"
)

func readAPI(ctx context.Context, endpoint string) ([]byte, error) {
	var result []byte
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		result, err = hostcmd.Run(ctx, "", "gh", nil, "api", "--hostname", "github.com", "--method", "GET", endpoint)
		if err == nil {
			return result, nil
		}
		var command *hostcmd.CommandError
		if !errors.As(err, &command) || command.Kind != "unavailable" || attempt == 2 {
			return nil, err
		}
		timer := time.NewTimer(time.Duration(1<<attempt) * 500 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, err
}
