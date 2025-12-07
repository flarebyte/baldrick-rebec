package server

import (
    "context"
    "errors"
    "fmt"
    "net/http"
    "net"
    "os"
    "os/signal"
    "path/filepath"
    "syscall"

	"github.com/flarebyte/baldrick-rebec/internal/config"
	pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
	toolingdao "github.com/flarebyte/baldrick-rebec/internal/dao/tooling"
	"github.com/flarebyte/baldrick-rebec/internal/paths"
	promptsvc "github.com/flarebyte/baldrick-rebec/internal/server/prompt"
	responsesvc "github.com/flarebyte/baldrick-rebec/internal/service/responses"
	factorypkg "github.com/flarebyte/baldrick-rebec/internal/service/responses/factory"
	"google.golang.org/grpc"
    "google.golang.org/grpc/reflection"
    "golang.org/x/net/http2"
    "golang.org/x/net/http2/h2c"
)

func DefaultPIDPath() string {
	h := paths.Home()
	_ = os.MkdirAll(h, 0o755)
	return filepath.Join(h, "server.pid")
}

func RunForeground(addr, pidPath string) error {
	if err := writePID(pidPath); err != nil {
		return err
	}
	defer removePID(pidPath)

    lis, err := net.Listen("tcp", addr)
    if err != nil {
        return fmt.Errorf("listen: %w", err)
    }
    // Create gRPC server, and an HTTP mux for Connect endpoints
    gs := grpc.NewServer()
    reflection.Register(gs)
    mux := http.NewServeMux()

    // Register PromptService backed by DAOs and services
    if cfg, err := config.Load(); err == nil {
        // Open DB with default timeout
        // Note: keep pool for process lifetime; server will close on shutdown.
        if db, e := pgdao.OpenApp(context.Background(), cfg); e == nil {
            svc := &promptsvc.Service{
                ToolDAO:          toolingdao.NewPGToolDAOAdapter(db),
                VaultDAO:         toolingdao.NewVaultDAOAdapter(),
                LLMFactory:       factorypkg.New(),
                ResponsesService: responsesvc.New(),
            }
            svc.Register(gs)
            mux.Handle("/prompt.v1.PromptService/Run", svc.ConnectHandler())
        }
    }

    // Graceful shutdown on SIGTERM/SIGINT and config reload on SIGHUP
    sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	go func() {
		for {
			sig := <-sigCh
			switch sig {
            case syscall.SIGTERM, syscall.SIGINT:
                gs.GracefulStop()
                return
			case syscall.SIGHUP:
				// Reload config; dynamic settings (like ports) require restart; we just refresh values.
				if _, err := config.Load(); err != nil {
					fmt.Fprintf(os.Stderr, "server: reload-config failed: %v\n", err)
				} else {
					fmt.Fprintln(os.Stderr, "server: config reloaded")
				}
			}
		}
	}()

    // Serve a single HTTP/2 cleartext server that routes gRPC vs HTTP based on content-type
    handler := h2c.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ct := r.Header.Get("Content-Type")
        if r.ProtoMajor == 2 && ct != "" && (ct == "application/grpc" || (len(ct) >= len("application/grpc") && ct[:len("application/grpc")] == "application/grpc")) {
            gs.ServeHTTP(w, r)
            return
        }
        mux.ServeHTTP(w, r)
    }), &http2.Server{})

    if err := http.Serve(lis, handler); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
        return err
    }
    return nil
}

func writePID(pidPath string) error {
	if _, err := os.Stat(pidPath); err == nil {
		// existing pid file
		return fmt.Errorf("pid file exists: %s", pidPath)
	}
	pid := os.Getpid()
	f, err := os.OpenFile(pidPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%d", pid)
	return err
}

func removePID(pidPath string) {
	_ = os.Remove(pidPath)
}

func ReadPID(pidPath string) (int, error) {
	b, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, err
	}
	var pid int
	if _, err := fmt.Sscanf(string(b), "%d", &pid); err != nil {
		return 0, err
	}
	return pid, nil
}

// DetachAttr returns platform-specific attributes to detach a process.
func DetachAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
