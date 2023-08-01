// Copyright 2022 The AmazeChain Authors
// This file is part of the AmazeChain library.
//
// The AmazeChain library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The AmazeChain library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the AmazeChain library. If not, see <http://www.gnu.org/licenses/>.

package node

import (
	"compress/gzip"
	"context"
	"fmt"
	"github.com/amazechain/amc/log"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/amazechain/amc/conf"
	"github.com/amazechain/amc/modules/rpc/jsonrpc"
)

type httpConfig struct {
	Modules            []string
	CorsAllowedOrigins []string
	Vhosts             []string
	prefix             string
	jwtSecret          []byte // optional JWT secret
}

// wsConfig is the JSON-RPC/Websocket configuration
type wsConfig struct {
	Origins   []string
	Modules   []string
	prefix    string // path prefix on which to mount ws handler
	jwtSecret []byte // optional JWT secret
}

type rpcHandler struct {
	http.Handler
	server *jsonrpc.Server
}

type httpServer struct {
	mux      http.ServeMux
	mu       sync.Mutex
	server   *http.Server
	listener net.Listener

	httpConfig  httpConfig
	httpHandler atomic.Value

	// WebSocket handler things.
	wsConfig  wsConfig
	wsHandler atomic.Value // *rpcHandler

	endpoint string
	host     string
	port     int

	handlerNames map[string]string
}

func newHTTPServer() *httpServer {
	h := &httpServer{handlerNames: make(map[string]string)}

	h.httpHandler.Store((*rpcHandler)(nil))
	h.wsHandler.Store((*rpcHandler)(nil))
	return h
}

func (h *httpServer) setListenAddr(host string, port int) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.listener != nil && (host != h.host || port != h.port) {
		return fmt.Errorf("HTTP server already running on %s", h.endpoint)
	}

	h.host, h.port = host, port
	h.endpoint = fmt.Sprintf("%s:%d", host, port)
	return nil
}

func (h *httpServer) listenAddr() string {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.listener != nil {
		return h.listener.Addr().String()
	}
	return h.endpoint
}

func (h *httpServer) start() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.endpoint == "" || h.listener != nil {
		return nil // already running or not configured
	}

	h.server = &http.Server{Handler: h}

	//todo
	h.server.ReadTimeout = time.Duration(60 * time.Second)
	h.server.WriteTimeout = time.Duration(60 * time.Second)
	h.server.IdleTimeout = time.Duration(60 * time.Second)

	listener, err := net.Listen("tcp", h.endpoint)
	if err != nil {
		h.disableRPC()
		h.disableWS()
		return err
	}
	h.listener = listener
	go h.server.Serve(listener)

	if h.wsAllowed() {
		url := fmt.Sprintf("ws://%v", listener.Addr())
		if h.wsConfig.prefix != "" {
			url += h.wsConfig.prefix
		}
		log.Info("WebSocket enabled", "url", url)
	}

	if !h.rpcAllowed() {
		return nil
	}
	log.Info("HTTP server started",
		"endpoint", listener.Addr(),
		"prefix", h.httpConfig.prefix,
		"cors", strings.Join(h.httpConfig.CorsAllowedOrigins, ","),
		"vhosts", strings.Join(h.httpConfig.Vhosts, ","),
	)

	var paths []string
	for path := range h.handlerNames {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	logged := make(map[string]bool, len(paths))
	for _, path := range paths {
		name := h.handlerNames[path]
		if !logged[name] {
			log.Info(name+" enabled", "url", "http://"+listener.Addr().String()+path)
			logged[name] = true
		}
	}
	return nil
}

func (h *httpServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// check if ws request and serve if ws enabled
	ws := h.wsHandler.Load().(*rpcHandler)
	if ws != nil && isWebsocket(r) {
		if checkPath(r, h.wsConfig.prefix) {
			ws.ServeHTTP(w, r)
		}
		return
	}
	// if http-rpc is enabled, try to serve request

	rpc := h.httpHandler.Load().(*rpcHandler)
	if rpc != nil {
		muxHandler, pattern := h.mux.Handler(r)
		if pattern != "" {
			muxHandler.ServeHTTP(w, r)
			return
		}

		if checkPath(r, h.httpConfig.prefix) {
			rpc.ServeHTTP(w, r)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

// enableWS turns on JSON-RPC over WebSocket on the server.
func (h *httpServer) enableWS(apis []jsonrpc.API, config wsConfig) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.wsAllowed() {
		return fmt.Errorf("JSON-RPC over WebSocket is already enabled")
	}
	// Create RPC server and handler.
	srv := jsonrpc.NewServer()
	if err := RegisterApisFromWhitelist(apis, config.Modules, srv, false); err != nil {
		return err
	}
	h.wsConfig = config
	h.wsHandler.Store(&rpcHandler{
		Handler: NewWSHandlerStack(srv.WebsocketHandler(config.Origins), config.jwtSecret),
		server:  srv,
	})
	return nil
}

// stopWS disables JSON-RPC over WebSocket and also stops the server if it only serves WebSocket.
func (h *httpServer) stopWS() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.disableWS() {
		if !h.rpcAllowed() {
			h.doStop()
		}
	}
}

// disableWS disables the WebSocket handler. This is internal, the caller must hold h.mu.
func (h *httpServer) disableWS() bool {
	ws := h.wsHandler.Load().(*rpcHandler)
	if ws != nil {
		h.wsHandler.Store((*rpcHandler)(nil))
		ws.server.Stop()
	}
	return ws != nil
}

// isWebsocket checks the header of an http request for a websocket upgrade request.
func isWebsocket(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

func checkPath(r *http.Request, path string) bool {
	if path == "" {
		return r.URL.Path == "/"
	}
	return len(r.URL.Path) >= len(path) && r.URL.Path[:len(path)] == path
}

func validatePrefix(what, path string) error {
	if path == "" {
		return nil
	}
	if path[0] != '/' {
		return fmt.Errorf(`%s RPC path prefix %q does not contain leading "/"`, what, path)
	}
	if strings.ContainsAny(path, "?#") {
		// This is just to avoid confusion. While these would match correctly (i.e. they'd
		// match if URL-escaped into path), it's not easy to understand for users when
		// setting that on the command line.
		return fmt.Errorf("%s RPC path prefix %q contains URL metadata-characters", what, path)
	}
	return nil
}

func (h *httpServer) stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.doStop()
}

func (h *httpServer) doStop() {
	if h.listener == nil {
		return // not running
	}

	httpHandler := h.httpHandler.Load().(*rpcHandler)
	if httpHandler != nil {
		h.httpHandler.Store((*rpcHandler)(nil))
		httpHandler.server.Stop()
	}
	h.server.Shutdown(context.Background())
	h.listener.Close()
	log.Info("HTTP server stopped", "endpoint", h.listener.Addr())

	h.host, h.port, h.endpoint = "", 0, ""
	h.server, h.listener = nil, nil
}

func (h *httpServer) enableRPC(apis []jsonrpc.API, config httpConfig) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.rpcAllowed() {
		return fmt.Errorf("JSON-RPC over HTTP is already enabled")
	}

	srv := jsonrpc.NewServer()
	if err := RegisterApisFromWhitelist(apis, config.Modules, srv, false); err != nil {
		return err
	}
	h.httpConfig = config
	h.httpHandler.Store(&rpcHandler{
		Handler: NewHTTPHandlerStack(srv, config.CorsAllowedOrigins, config.Vhosts, config.jwtSecret),
		server:  srv,
	})
	return nil
}

func (h *httpServer) disableRPC() bool {
	handler := h.httpHandler.Load().(*rpcHandler)
	if handler != nil {
		h.httpHandler.Store((*rpcHandler)(nil))
		handler.server.Stop()
	}
	return handler != nil
}

func (h *httpServer) rpcAllowed() bool {
	return h.httpHandler.Load().(*rpcHandler) != nil
}

// wsAllowed returns true when JSON-RPC over WebSocket is enabled.
func (h *httpServer) wsAllowed() bool {
	return h.wsHandler.Load().(*rpcHandler) != nil
}

func NewHTTPHandlerStack(srv http.Handler, cors []string, vhosts []string, jwtSecret []byte) http.Handler {
	handler := newVHostHandler(vhosts, srv)
	if len(jwtSecret) != 0 {
		handler = newJWTHandler(jwtSecret, handler)
	}
	return newGzipHandler(handler)
}

// NewWSHandlerStack returns a wrapped ws-related handler.
func NewWSHandlerStack(srv http.Handler, jwtSecret []byte) http.Handler {
	if len(jwtSecret) != 0 {
		return newJWTHandler(jwtSecret, srv)
	}
	return srv
}

type virtualHostHandler struct {
	vhosts map[string]struct{}
	next   http.Handler
}

func newVHostHandler(vhosts []string, next http.Handler) http.Handler {
	vhostMap := make(map[string]struct{})
	for _, allowedHost := range vhosts {
		vhostMap[strings.ToLower(allowedHost)] = struct{}{}
	}
	return &virtualHostHandler{vhostMap, next}
}

func (h *virtualHostHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Host == "" {
		h.next.ServeHTTP(w, r)
		return
	}
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}
	if ipAddr := net.ParseIP(host); ipAddr != nil {
		h.next.ServeHTTP(w, r)
		return

	}
	if _, exist := h.vhosts["*"]; exist {
		h.next.ServeHTTP(w, r)
		return
	}
	if _, exist := h.vhosts[host]; exist {
		h.next.ServeHTTP(w, r)
		return
	}
	http.Error(w, "invalid host specified", http.StatusForbidden)
}

var gzPool = sync.Pool{
	New: func() interface{} {
		w := gzip.NewWriter(ioutil.Discard)
		return w
	},
}

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w *gzipResponseWriter) WriteHeader(status int) {
	w.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(status)
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func newGzipHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzPool.Get().(*gzip.Writer)
		defer gzPool.Put(gz)

		gz.Reset(w)
		defer gz.Close()

		next.ServeHTTP(&gzipResponseWriter{ResponseWriter: w, Writer: gz}, r)
	})
}

type ipcServer struct {
	endpoint string

	mu       sync.Mutex
	listener net.Listener
	srv      *jsonrpc.Server
}

func newIPCServer(config *conf.NodeConfig) *ipcServer {
	return &ipcServer{endpoint: fmt.Sprintf("%s/%s", config.DataDir, config.IPCPath)}
}

func (is *ipcServer) start(apis []jsonrpc.API) error {
	is.mu.Lock()
	defer is.mu.Unlock()

	if is.listener != nil {
		return nil // already running
	}
	listener, srv, err := jsonrpc.StartIPCEndpoint(is.endpoint, apis)
	if err != nil {
		log.Warn("IPC opening failed", "url", is.endpoint, "error", err)
		return err
	}
	log.Info("IPC endpoint opened", "url", is.endpoint)
	is.listener, is.srv = listener, srv
	return nil
}

func (is *ipcServer) stop() error {
	is.mu.Lock()
	defer is.mu.Unlock()

	if is.listener == nil {
		return nil // not running
	}
	err := is.listener.Close()
	is.srv.Stop()
	is.listener, is.srv = nil, nil
	log.Info("IPC endpoint closed", "url", is.endpoint)
	return err
}

func RegisterApisFromWhitelist(apis []jsonrpc.API, modules []string, srv *jsonrpc.Server, exposeAll bool) error {
	if bad, available := checkModuleAvailability(modules, apis); len(bad) > 0 {
		log.Error("Unavailable modules in HTTP API list", "unavailable", bad, "available", available)
	}
	whitelist := make(map[string]bool)
	for _, module := range modules {
		whitelist[module] = true
	}
	for _, api := range apis {
		if exposeAll || whitelist[api.Namespace] {
			if err := srv.RegisterName(api.Namespace, api.Service); err != nil {
				return err
			}
		}
	}
	return nil
}
