package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"k8s.io/klog/v2"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	"mobilint-device-plugin/pkg/config"
	"mobilint-device-plugin/pkg/plugin"
)

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()

	socket := filepath.Join(pluginapi.DevicePluginPath, config.PluginSocketName)
	p := plugin.New(socket)

	if err := p.Start(); err != nil {
		klog.Fatalf("failed to start device plugin: %v", err)
	}
	defer p.Stop()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go runRegister(ctx, stop, p)

	select {
	case <-ctx.Done():
	case err := <-p.Err():
		if err != nil {
			klog.Errorf("device plugin server stopped: %v", err)
		}
	}

	klog.Infof("shutting down mobilint device plugin")
}

func runRegister(ctx context.Context, stop context.CancelFunc, p *plugin.MobilintDevicePlugin) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		klog.Errorf("failed to create fsnotify watcher: %v", err)
		stop()
		return
	}
	defer watcher.Close()

	if err := watcher.Add(pluginapi.DevicePluginPath); err != nil {
		klog.Errorf("failed to watch %s: %v", pluginapi.DevicePluginPath, err)
		stop()
		return
	}

	if err := registerWithRetry(ctx, p); err != nil {
		klog.Errorf("initial registration aborted: %v", err)
		stop()
		return
	}

	kubeletSock := filepath.Base(pluginapi.KubeletSocket)

	for {
		select {
		case <-ctx.Done():
			return

		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			//when kubelet socket recreated
			if event.Op&fsnotify.Create != 0 && filepath.Base(event.Name) == kubeletSock {
				klog.Infof("kubelet socket recreated, re-registering")
				if err := registerWithRetry(ctx, p); err != nil {
					klog.Errorf("re-registration aborted: %v", err)
					stop()
					return
				}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			klog.Errorf("fsnotify watcher error: %v", err)
		}
	}
}

func registerWithRetry(ctx context.Context, p *plugin.MobilintDevicePlugin) error {
	ticker := time.NewTicker(time.Duration(config.RegisterRetrySeconds) * time.Second)
	defer ticker.Stop()

	for attempt := 1; attempt <= config.RegisterMaxAttempts; attempt++ {
		if err := p.Register(ctx); err == nil {
			klog.Infof("registered %s with kubelet", config.ResourceName)
			return nil
		} else {
			klog.Errorf("register attempt %d/%d failed: %v", attempt, config.RegisterMaxAttempts, err)
		}

		if attempt >= config.RegisterMaxAttempts {
			break
		}

		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
		}
	}
	return fmt.Errorf("register exhausted after %d attempts", config.RegisterMaxAttempts)
}
