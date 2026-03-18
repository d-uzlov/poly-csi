package configwatcher

import (
	"context"
	"time"

	"github.com/fsnotify/fsnotify"
	slogctx "github.com/veqryn/slog-context"
)

func WatchConfigChanges(ctx context.Context, path string, action func()) {
	logger := slogctx.FromCtx(ctx)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err.Error())
	}
	defer watcher.Close()

	for {
		if ctx.Err() != nil {
			logger.Error("config watcher thread finished (context done)")
			return
		}
		err = watcher.Add(path)
		if err != nil {
			logger.Error("could not watch for file changes", "path", path, "error", err.Error())
			select {
			case <-ctx.Done():
				logger.Error("config watcher thread finished (context done)")
				return
			case <-time.NewTimer(time.Second * 10).C:
				continue
			}
		}
		logger.Info("added a new file to watch")
		action()
		watcherOk := true
		for watcherOk {
			select {
			case <-ctx.Done():
				logger.Error("config watcher thread finished (context done)")
				return
			case event, ok := <-watcher.Events:
				if !ok {
					logger.Error("config watcher thread finished (event channel closed)")
					return
				}
				logger.Debug("got file event", "event-type", event.Op.String())
				if event.Op.Has(fsnotify.Write) {
					// TODO switch to xUnportableCloseWrite ??
					logger.Info("detected config file write")
					action()
				}
				if event.Op.Has(fsnotify.Remove) || event.Op.Has(fsnotify.Rename) {
					logger.Info("detected config file removal, will restart watcher")
					watcherOk = false
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					logger.Error("config watcher thread finished (error channel closed)")
					return
				}
				logger.Error("config watcher received error", "error", err.Error())
			}
		}
	}
}
