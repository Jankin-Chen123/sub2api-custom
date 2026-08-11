package service

import (
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// DedicatedImageRuntimeController applies mutable admin settings to the
// process-local worker lifecycle. Request-path gates read the same atomic
// settings from Config, so a save becomes effective without a restart.
type DedicatedImageRuntimeController struct {
	cfg     *config.Config
	worker  *ImageGenerationWorkerRuntime
	cleanup *ImageGenerationCleanupService
	mu      sync.Mutex
}

func NewDedicatedImageRuntimeController(
	cfg *config.Config,
	worker *ImageGenerationWorkerRuntime,
	cleanup *ImageGenerationCleanupService,
) *DedicatedImageRuntimeController {
	return &DedicatedImageRuntimeController{cfg: cfg, worker: worker, cleanup: cleanup}
}

func (c *DedicatedImageRuntimeController) Apply(settings config.DedicatedImageRuntimeSettings) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg != nil {
		c.cfg.SetDedicatedImageRuntime(settings)
	}
	if settings.Enabled && settings.WorkerEnabled {
		if c.worker != nil {
			c.worker.Start()
		}
		if c.cleanup != nil {
			c.cleanup.Start()
		}
		return
	}
	if c.cleanup != nil {
		c.cleanup.Stop()
	}
	if c.worker != nil {
		c.worker.Stop()
	}
}

func ProvideDedicatedImageRuntimeController(
	settings *SettingService,
	worker *ImageGenerationWorkerRuntime,
	cleanup *ImageGenerationCleanupService,
	cfg *config.Config,
) *DedicatedImageRuntimeController {
	controller := NewDedicatedImageRuntimeController(cfg, worker, cleanup)
	if settings != nil {
		settings.SetDedicatedImageRuntimeApplier(controller.Apply)
	}
	if cfg != nil {
		controller.Apply(cfg.DedicatedImageRuntime())
	}
	return controller
}
