package mountplugins

import (
	"context"
	"time"
)

type MounterPlugin interface {
	StartServe(ctx context.Context, shutdownTimeout time.Duration, doneCallback func(error))
}
