package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tripsyapp/cli/internal/cli"
	"github.com/tripsyapp/cli/internal/mcpserver"
)

func main() {
	var opts mcpserver.Options
	var transport string
	var httpAddr string
	var httpPath string
	var httpStateless bool
	var httpJSON bool
	var showVersion bool

	flags := flag.NewFlagSet("tripsy-mcp", flag.ExitOnError)
	flags.StringVar(&opts.APIBase, "api-base", "", "Tripsy API base URL; defaults to TRIPSY_API_BASE, stored config, or https://api.tripsy.app")
	flags.StringVar(&opts.Token, "token", "", "Tripsy API token; defaults to TRIPSY_TOKEN or stored credentials")
	flags.StringVar(&opts.ConfigDir, "config-dir", "", "Tripsy CLI config directory; defaults to TRIPSY_CONFIG_DIR or ~/.config/tripsy-cli")
	flags.StringVar(&transport, "transport", "stdio", "MCP transport: stdio or http")
	flags.StringVar(&httpAddr, "http-addr", "127.0.0.1:8787", "listen address for --transport=http")
	flags.StringVar(&httpPath, "http-path", "/mcp", "HTTP endpoint path for --transport=http")
	flags.BoolVar(&httpStateless, "http-stateless", false, "run streamable HTTP without MCP session retention")
	flags.BoolVar(&httpJSON, "http-json-response", false, "prefer application/json responses for streamable HTTP")
	flags.BoolVar(&showVersion, "version", false, "print version and exit")
	flags.BoolVar(&showVersion, "v", false, "print version and exit")
	_ = flags.Parse(os.Args[1:])

	opts.Version = cli.Version
	if showVersion {
		fmt.Println(versionString())
		return
	}

	server, info, err := mcpserver.New(opts)
	if err != nil {
		log.Fatal(err)
	}

	switch strings.ToLower(strings.TrimSpace(transport)) {
	case "", "stdio":
		if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			log.Fatal(err)
		}
	case "http", "streamable-http":
		runHTTP(server, info, httpAddr, httpPath, httpStateless, httpJSON)
	default:
		log.Fatalf("unsupported transport %q; expected stdio or http", transport)
	}
}

func runHTTP(server *mcp.Server, info mcpserver.RuntimeInfo, addr, path string, stateless, jsonResponse bool) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{
		Stateless:      stateless,
		JSONResponse:   jsonResponse,
		Logger:         slog.New(slog.NewTextHandler(os.Stderr, nil)),
		SessionTimeout: 30 * time.Minute,
	})

	mux := http.NewServeMux()
	mux.Handle(path, handler)
	log.Printf("Tripsy MCP listening on http://%s%s (api_base=%s auth_backend=%s has_token=%t)", addr, path, info.APIBase, info.AuthBackend, info.HasToken)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func versionString() string {
	value := "tripsy-mcp version " + cli.Version
	if cli.Commit != "" {
		value += " (" + cli.Commit + ")"
	}
	if cli.Date != "" {
		value += " " + cli.Date
	}
	return value
}
