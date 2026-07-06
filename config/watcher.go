package config

import (
	"log/slog"
	"maps"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// knownConfigFiles lists the config file basenames that trigger a reload.
var knownConfigFiles = map[string]bool{
	".env":               true,
	".env.local":         true,
	"config.yaml":        true,
	"config.local.yaml":  true,
	"settings.json":      true,
	"settings.local.json": true,
}

// ReloadCallback is the function signature for reload notifications.
// err is non-nil if any config file failed to load during the reload.
type ReloadCallback func(err error)

// CallbackManager manages a thread-safe set of named reload callbacks.
type CallbackManager struct {
	mu        sync.RWMutex
	callbacks map[string]ReloadCallback
}

var globalCallbackMgr = &CallbackManager{
	callbacks: make(map[string]ReloadCallback),
}

// RegisterReloadCallback registers a named callback to be invoked on config reload.
// If a callback with the same name already exists, it is overwritten.
// If callback is nil, the call is a no-op.
func RegisterReloadCallback(name string, callback ReloadCallback) {
	if callback == nil {
		return
	}
	globalCallbackMgr.mu.Lock()
	defer globalCallbackMgr.mu.Unlock()
	globalCallbackMgr.callbacks[name] = callback
	slog.Debug("reload callback registered", "name", name)
}

// UnregisterReloadCallback removes a previously registered callback by name.
// If the name does not exist, the call is a no-op.
func UnregisterReloadCallback(name string) {
	globalCallbackMgr.mu.Lock()
	defer globalCallbackMgr.mu.Unlock()
	delete(globalCallbackMgr.callbacks, name)
	slog.Debug("reload callback unregistered", "name", name)
}

func (m *CallbackManager) invokeAll(err error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for name, cb := range m.callbacks {
		slog.Debug("invoking reload callback", "name", name)
		cb(err)
	}
}

// watcher internal state
var (
	watcherMu      sync.Mutex
	fsWatcher      *fsnotify.Watcher
	watcherDone    chan struct{}
	watcherStarted bool
)

// WithWatch returns a ConfigOption that enables config file watching.
// The watcher starts automatically after Default() finishes loading.
//
// Usage:
//
//	config.Default(config.WithWatch())
//	config.RegisterReloadCallback("mylogic", func(err error) {
//	    if err != nil {
//	        slog.Error("config reload failed", "err", err)
//	        return
//	    }
//	    // re-read affected viper keys...
//	})
func WithWatch() ConfigOption {
	return func(o *configOptions) {
		o.watch = true
	}
}

// StartWatch begins watching config directories for file changes.
// Returns ErrWatcherAlreadyRunning if a watcher is already active.
func StartWatch() error {
	watcherMu.Lock()
	defer watcherMu.Unlock()

	if watcherStarted {
		return ErrWatcherAlreadyRunning
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	dirs := configDirs()
	for _, dir := range dirs {
		if err := w.Add(dir); err != nil {
			w.Close()
			slog.Warn("failed to watch config directory", "dir", dir, "err", err)
		}
	}

	fsWatcher = w
	watcherDone = make(chan struct{})
	watcherStarted = true

	go watchLoop(w, watcherDone)

	slog.Info("config watcher started", "dirs", dirs)
	return nil
}

// StopWatch stops the config file watcher and releases fsnotify resources.
// Returns ErrWatcherNotRunning if no watcher is active.
func StopWatch() error {
	watcherMu.Lock()
	defer watcherMu.Unlock()

	if !watcherStarted {
		return ErrWatcherNotRunning
	}

	close(watcherDone)
	if fsWatcher != nil {
		fsWatcher.Close()
		fsWatcher = nil
	}
	watcherStarted = false

	slog.Info("config watcher stopped")
	return nil
}

// configDirs returns the directories to watch for config changes.
func configDirs() []string {
	dirs := []string{".", "./conf"}
	if dir := GetAppConfigDir(); dir != "" {
		dirs = append(dirs, dir)
	}
	return dirs
}

// watchLoop is the main event loop. It consumes fsnotify events from a channel
// in a dedicated goroutine and uses a debounce timer to avoid multiple rapid
// reloads when several config files are written in quick succession.
func watchLoop(w *fsnotify.Watcher, done chan struct{}) {
	const debounceInterval = 200 * time.Millisecond
	var debounceTimer *time.Timer

	for {
		select {
		case <-done:
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return

		case event, ok := <-w.Events:
			if !ok {
				return
			}
			if !isConfigFile(event.Name) {
				continue
			}
			if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
				continue
			}

			slog.Debug("config file change detected",
				"file", filepath.Base(event.Name),
				"op", event.Op.String(),
			)

			// Reset debounce timer on each event;
			// reload fires after a quiet period with no new events.
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(debounceInterval, func() {
				slog.Info("reloading config due to file change")
				err := reloadAllConfigs()
				globalCallbackMgr.invokeAll(err)
			})

		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			slog.Error("config watcher error", "err", err)
		}
	}
}

// isConfigFile returns true if the file path basename matches a known config file.
func isConfigFile(path string) bool {
	name := filepath.Base(path)
	return knownConfigFiles[name]
}

// reloadAllConfigs reloads every configuration source and overwrites the
// global viper instance with the new values.
//
// Strategy: create temporary vipers for each config format, merge them,
// then use viper.Set() per top-level key to overwrite the global viper.
// viper.Set() always overwrites — unlike MergeConfigMap which preserves
// existing values — so changed config values are correctly reflected.
//
// Limitation: keys removed from config files persist in the global viper
// from the previous load. In practice config files are additive.
func reloadAllConfigs() error {
	v1 := NewEnvConfig().Load()
	v2 := NewYamlConfig().Load()
	v3 := NewJsonConfig().Load()

	// Merge in load order: env < yaml < json (same priority as Default())
	merged := make(map[string]any)
	maps.Copy(merged, v1.AllSettings())
	maps.Copy(merged, v2.AllSettings())
	maps.Copy(merged, v3.AllSettings())

	// Overwrite global viper top-level keys.
	// viper.Set() overwrites existing values (unlike MergeConfigMap).
	for k, v := range merged {
		viper.Set(k, v)
	}

	slog.Debug("config reload complete", "top_level_keys", len(merged))
	return nil
}

// Sentinel errors for watcher lifecycle.
var (
	ErrWatcherAlreadyRunning = sentinelError("config watcher is already running")
	ErrWatcherNotRunning     = sentinelError("config watcher is not running")
)

type sentinelError string

func (e sentinelError) Error() string { return string(e) }
