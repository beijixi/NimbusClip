package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	agentclient "clipflow/internal/ui/client/agent"
	uiconfig "clipflow/internal/ui/config"
	"clipflow/internal/ui/hotkey"
	uiserver "clipflow/internal/ui/server"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfgPath := uiconfig.DefaultPath()
	cfg, err := uiconfig.LoadOrDefault(cfgPath)
	if err != nil {
		log.Printf("failed to load UI config: %v", err)
		cfg = uiconfig.Default()
	}
	client, err := agentclient.NewClient(cfg.AgentEndpoint)
	if err != nil {
		log.Fatalf("invalid agent endpoint %s: %v", cfg.AgentEndpoint, err)
	}

	hk, err := hotkey.NewManager()
	if err != nil {
		log.Printf("hotkey manager unavailable: %v", err)
		hk = nil
	}

	addr := "127.0.0.1:8787"
	srv, err := uiserver.New(addr, cfg, cfgPath, client, hk, func() {
		go openBrowser(fmt.Sprintf("http://%s", addr))
	})
	if err != nil {
		log.Fatalf("failed to create UI server: %v", err)
	}
	if err := srv.Start(); err != nil {
		log.Fatalf("failed to start UI server: %v", err)
	}
	go func() {
		// wait for listener before opening the browser
		for {
			conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
			if err == nil {
				_ = conn.Close()
				break
			}
			time.Sleep(150 * time.Millisecond)
		}
		openBrowser(fmt.Sprintf("http://%s", addr))
	}()

	<-ctx.Done()
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	if err := srv.Stop(shutdownCtx); err != nil {
		log.Printf("failed to stop UI server: %v", err)
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("failed to open browser: %v", err)
	}
}
