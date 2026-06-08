package main

import (
	"context"
	"flag"
	"net/http"
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
	"mobilint-device-plugin/pkg/plugin/metrics"
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()
	defer klog.Flush()

	socket := filepath.Join(pluginapi.DevicePluginPath, config.PluginSocketName)
	p := plugin.New(socket)

	if err := p.Start(); err != nil {
		klog.Fatalf("failed to start device plugin: %v", err)
	}
	defer p.Stop()

	metricsSrv := &http.Server{
		Addr:              config.MetricsAddr,
		Handler:           metrics.NewHandler(p.IsRegistered),
		ReadHeaderTimeout: time.Duration(config.MetricsReadHeaderTimeoutSeconds) * time.Second,
		ReadTimeout:       time.Duration(config.MetricsReadTimeoutSeconds) * time.Second,
		WriteTimeout:      time.Duration(config.MetricsWriteTimeoutSeconds) * time.Second,
		IdleTimeout:       time.Duration(config.MetricsIdleTimeoutSeconds) * time.Second,
	}
	go func() {
		klog.Infof("metrics server listening addr=%s", config.MetricsAddr)
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.Errorf("metrics server: %v", err)
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
			klog.Warningf("metrics server shutdown: %v", err)
		}
	}()

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

	registerWithBackoff(ctx, p)

	kubeletSock := filepath.Base(pluginapi.KubeletSocket)

	for {
		select {
		case <-ctx.Done():
			return

		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			// When kubelet socket is recreated, re-register the plugin.
			if event.Op&fsnotify.Create != 0 && filepath.Base(event.Name) == kubeletSock {
				klog.Infof("kubelet socket recreated, re-registering")
				p.SetRegistered(false)
				registerWithBackoff(ctx, p)
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			klog.Errorf("fsnotify watcher error: %v", err)
		}
	}
}

// Retry registration forever with exponential backoff; stay NotReady instead of crashing.
func registerWithBackoff(ctx context.Context, p *plugin.MobilintDevicePlugin) {
	backoff := time.Duration(config.RegisterBackoffInitialSeconds) * time.Second
	maxBackoff := time.Duration(config.RegisterBackoffMaxSeconds) * time.Second
	attempts := 0

	for {
		if err := p.Register(ctx); err == nil {
			klog.Infof("registered %s with kubelet", config.ResourceName)
			p.SetRegistered(true)
			return
		} else {
			attempts++
			if attempts >= config.RegisterFailureLogThreshold {
				klog.Errorf("register failing (attempt %d, retrying every %s): %v", attempts, backoff, err)
			} else {
				klog.Warningf("register attempt %d failed, retry in %s: %v", attempts, backoff, err)
			}
		}

		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		backoff = min(backoff*2, maxBackoff)
	}
}
