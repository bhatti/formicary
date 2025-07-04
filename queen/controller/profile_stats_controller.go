package controller

import (
	"net/http"
	"net/http/pprof"
	"plexobject.com/formicary/internal/types"
	"runtime"
	"time"

	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/web"

	log "github.com/sirupsen/logrus"
)

// ProfileStatsController structure
type ProfileStatsController struct {
	cfg *types.CommonConfig
}

// NewProfileStatsController controller - everything on same port as your APIs
func NewProfileStatsController(
	cfg *types.CommonConfig,
	webserver web.Server) *ProfileStatsController {

	// Enable more detailed GC and memory stats
	runtime.SetMutexProfileFraction(1)
	runtime.SetBlockProfileRate(1)

	ctr := &ProfileStatsController{
		cfg: cfg,
	}

	// Add your existing stats endpoint
	webserver.GET("/debug/stats", ctr.stats, acl.NewPermission(acl.ProfileStats, acl.View)).Name = "profile_stats"

	// Add all pprof endpoints to the SAME port as your APIs
	ctr.registerPprofEndpoints(webserver)

	log.WithFields(log.Fields{
		"Port": cfg.HTTPPort,
	}).Info("pprof endpoints registered on main HTTP port")

	return ctr
}

// registerPprofEndpoints adds pprof endpoints to your existing web server
func (ctr *ProfileStatsController) registerPprofEndpoints(webserver web.Server) {
	// Use the same ACL permission as your stats endpoint
	profilePermission := acl.NewPermission(acl.ProfileStats, acl.View)

	// Main pprof index - shows all available profiles
	webserver.GET("/debug/pprof/", ctr.wrapPprofHandler(pprof.Index), profilePermission).Name = "pprof_index"
	webserver.GET("/debug/pprof", ctr.wrapPprofHandler(pprof.Index), profilePermission).Name = "pprof_index_alt"

	// CPU Profile - this is the main one you'll use
	webserver.GET("/debug/pprof/profile", ctr.wrapPprofHandler(pprof.Profile), profilePermission).Name = "pprof_profile"

	// Memory Profiles
	webserver.GET("/debug/pprof/heap", ctr.wrapPprofHandler(pprof.Handler("heap").ServeHTTP), profilePermission).Name = "pprof_heap"
	webserver.GET("/debug/pprof/allocs", ctr.wrapPprofHandler(pprof.Handler("allocs").ServeHTTP), profilePermission).Name = "pprof_allocs"

	// Goroutine Profile
	webserver.GET("/debug/pprof/goroutine", ctr.wrapPprofHandler(pprof.Handler("goroutine").ServeHTTP), profilePermission).Name = "pprof_goroutine"

	// Block and Mutex Profiles (for concurrency issues)
	webserver.GET("/debug/pprof/block", ctr.wrapPprofHandler(pprof.Handler("block").ServeHTTP), profilePermission).Name = "pprof_block"
	webserver.GET("/debug/pprof/mutex", ctr.wrapPprofHandler(pprof.Handler("mutex").ServeHTTP), profilePermission).Name = "pprof_mutex"

	// Other useful profiles
	webserver.GET("/debug/pprof/threadcreate", ctr.wrapPprofHandler(pprof.Handler("threadcreate").ServeHTTP), profilePermission).Name = "pprof_threadcreate"
	webserver.GET("/debug/pprof/cmdline", ctr.wrapPprofHandler(pprof.Cmdline), profilePermission).Name = "pprof_cmdline"
	webserver.GET("/debug/pprof/symbol", ctr.wrapPprofHandler(pprof.Symbol), profilePermission).Name = "pprof_symbol"
	webserver.GET("/debug/pprof/trace", ctr.wrapPprofHandler(pprof.Trace), profilePermission).Name = "pprof_trace"

	log.Info("All pprof endpoints registered with ACL protection on main port")
}

// wrapPprofHandler wraps a pprof handler for your custom web framework
func (ctr *ProfileStatsController) wrapPprofHandler(handler func(http.ResponseWriter, *http.Request)) web.HandlerFunc {
	return func(c web.APIContext) error {
		// Convert your web framework's context to standard http request/response
		req := c.Request()
		resp := c.Response()

		// Call the pprof handler directly
		handler(resp, req)
		return nil
	}
}

// ********************************* HTTP Handlers ***********************************
func (ctr *ProfileStatsController) stats(c web.APIContext) error {
	res := make(map[string]interface{})
	web.RenderDBUserFromSession(c, res)
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	stats := map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"memory": map[string]interface{}{
			"alloc_mb":       bToMb(m.Alloc),
			"total_alloc_mb": bToMb(m.TotalAlloc),
			"sys_mb":         bToMb(m.Sys),
			"num_gc":         m.NumGC,
			"heap_alloc_mb":  bToMb(m.HeapAlloc),
			"heap_sys_mb":    bToMb(m.HeapSys),
			"heap_idle_mb":   bToMb(m.HeapIdle),
			"heap_inuse_mb":  bToMb(m.HeapInuse),
			"heap_objects":   m.HeapObjects,
			"stack_inuse_mb": bToMb(m.StackInuse),
			"stack_sys_mb":   bToMb(m.StackSys),
		},
		"runtime": map[string]interface{}{
			"num_goroutines": runtime.NumGoroutine(),
			"num_cpu":        runtime.NumCPU(),
			"go_version":     runtime.Version(),
		},
		"profiling": map[string]interface{}{
			"available": true,
			"port":      ctr.cfg.HTTPPort,
			"endpoints": map[string]string{
				"index":             "/debug/pprof/",
				"cpu_profile":       "/debug/pprof/profile?seconds=30",
				"heap_profile":      "/debug/pprof/heap",
				"goroutine_profile": "/debug/pprof/goroutine",
				"block_profile":     "/debug/pprof/block",
				"mutex_profile":     "/debug/pprof/mutex",
				"allocs_profile":    "/debug/pprof/allocs",
			},
		},
	}

	c.Response().Header().Set("Content-Type", "application/json")
	return c.Render(http.StatusOK, "stats", stats)
}

func bToMb(b uint64) float64 {
	return float64(b) / 1024 / 1024
}
