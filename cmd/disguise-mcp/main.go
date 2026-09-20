// Command disguise-mcp bridges the disguise Designer API to the Model Context
// Protocol, letting an AI client control a disguise media server (transport /
// timeline playback and more) over MCP.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/medcelerate/disguise-mcp/internal/app"
	"github.com/medcelerate/disguise-mcp/internal/config"
	"github.com/medcelerate/disguise-mcp/internal/mcpserver"
	"github.com/medcelerate/disguise-mcp/internal/version"
	"github.com/medcelerate/disguise-mcp/internal/web"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	var (
		cfgPath     = flag.String("config", envOr("DISGUISEMCP_CONFIG", ""), "path to the YAML config file")
		showVersion = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println(version.String())
		return
	}

	logger := log.New(os.Stderr, "", log.LstdFlags)
	logf := func(format string, args ...any) { logger.Printf(format, args...) }

	if err := run(*cfgPath, logf); err != nil {
		logger.Fatalf("fatal: %v", err)
	}
}

func run(cfgPath string, logf func(string, ...any)) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	logf("%s", version.String())
	logf("disguise target=%s mcp.transport=%s", cfg.Target().BaseURL(), cfg.MCP.Transport)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a := app.New(cfg, cfgPath, logf)

	if cfg.Web.Enabled {
		wsrv := web.New(a, logf)
		go func() {
			if err := wsrv.Serve(ctx, cfg.Web.Addr); err != nil {
				logf("web console error: %v", err)
			}
		}()
	}

	mcpSrv := mcpserver.New(a)

	switch cfg.MCP.Transport {
	case config.TransportStdio:
		return runStdio(ctx, mcpSrv, logf)
	case config.TransportHTTP:
		return runHTTP(ctx, mcpSrv, cfg.MCP.HTTP.Addr, logf)
	case config.TransportBoth:
		go func() {
			if err := runHTTP(ctx, mcpSrv, cfg.MCP.HTTP.Addr, logf); err != nil {
				logf("mcp http error: %v", err)
			}
		}()
		return runStdio(ctx, mcpSrv, logf)
	default:
		return fmt.Errorf("unknown transport %q", cfg.MCP.Transport)
	}
}

func runStdio(ctx context.Context, srv *mcp.Server, logf func(string, ...any)) error {
	logf("serving MCP over stdio")
	if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil && ctx.Err() == nil {
		return fmt.Errorf("mcp stdio: %w", err)
	}
	return nil
}

func runHTTP(ctx context.Context, srv *mcp.Server, addr string, logf func(string, ...any)) error {
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
	httpSrv := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutCtx)
	}()
	logf("serving MCP over Streamable HTTP on http://%s", addr)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("mcp http: %w", err)
	}
	return nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
