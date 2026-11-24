package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/relayx-tech/panel/internal/upgrader"
)

func main() {
	socketPath := flag.String("socket", "/run/relayx/upgrader.sock", "UDS socket path")
	tokenFile := flag.String("token-file", "/etc/relayx/upgrader.token", "shared token file")
	stateFile := flag.String("state-file", "/var/lib/relayx-upgrader/state.json", "state file")
	baseDir := flag.String("base-dir", "/opt/relayx", "release root with current symlink")
	services := flag.String("services", "relayx-panel.service,relayx-core-control.service", "services to restart")
	flag.Parse()

	token, err := os.ReadFile(*tokenFile)
	if err != nil {
		log.Fatalf("read token: %v", err)
	}

	store := &upgrader.FileStore{Path: *stateFile}
	applier := &upgrader.FileApplier{
		BaseDir:  *baseDir,
		Services: splitList(*services),
		Runner:   upgrader.SystemctlRunner{},
		Linker:   upgrader.SymlinkLinker{},
		Store:    store,
		HealthCheck: func(ctx context.Context) error {
			return upgrader.WaitPanelHealth(ctx, "http://127.0.0.1:8888/readyz", 60*time.Second)
		},
	}
	server := &upgrader.Server{Applier: applier, Store: store, Token: strings.TrimSpace(string(token))}

	if err := os.Remove(*socketPath); err != nil && !os.IsNotExist(err) {
		log.Fatalf("remove stale socket: %v", err)
	}
	listener, err := net.Listen("unix", *socketPath)
	if err != nil {
		log.Fatalf("listen %s: %v", *socketPath, err)
	}
	if err := os.Chmod(*socketPath, 0o660); err != nil {
		log.Printf("chmod socket: %v", err)
	}

	httpServer := &http.Server{Handler: server.Handler()}
	go func() {
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Fatalf("serve: %v", err)
		}
	}()
	fmt.Printf("relayx-upgrader listening on %s\n", *socketPath)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	_ = httpServer.Close()
}

func splitList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
