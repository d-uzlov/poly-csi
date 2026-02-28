package mountplugins

import (
	"context"
	"time"
)

type ConnectorPlugin interface {
	StartServe(ctx context.Context, shutdownTimeout time.Duration, doneCallback func(error))
}
