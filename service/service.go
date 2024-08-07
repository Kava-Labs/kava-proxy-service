// package service provides functions and methods
// for creating and running the api of the proxy service
package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/kava-labs/kava-proxy-service/clients/cache"
	"github.com/kava-labs/kava-proxy-service/clients/database"
	"github.com/kava-labs/kava-proxy-service/clients/database/migrations"
	"github.com/kava-labs/kava-proxy-service/config"
	"github.com/kava-labs/kava-proxy-service/logging"
	"github.com/kava-labs/kava-proxy-service/service/batchmdw"
	"github.com/kava-labs/kava-proxy-service/service/cachemdw"
)

// ProxyService represents an instance of the proxy service API
type ProxyService struct {
	Database  *database.PostgresClient
	Cache     *cachemdw.ServiceCache
	httpProxy *http.Server
	evmClient *ethclient.Client
	*logging.ServiceLogger
}

// New returns a new ProxyService with the specified config and error (if any)
func New(ctx context.Context, config config.Config, serviceLogger *logging.ServiceLogger) (ProxyService, error) {
	service := ProxyService{}

	// create database client
	db, err := createDatabaseClient(ctx, config, serviceLogger)
	if err != nil {
		return ProxyService{}, err
	}

	// create evm api client
	evmClient, err := ethclient.Dial(config.EvmQueryServiceURL)
	if err != nil {
		return ProxyService{}, err
	}

	// create cache client
	serviceCache, err := createServiceCache(ctx, config, serviceLogger, evmClient)
	if err != nil {
		return ProxyService{}, err
	}

	// create an http router for registering handlers for a given route
	mux := http.NewServeMux()
	if config.EnablePprof {
		registerPprofHandlers(config, mux)
	}

	// AfterProxyFinalizer will run after the proxy middleware handler and is
	// the final function called after all other middleware
	// allowing it to access values added to the request context
	// to do things like metric the response and cache the response
	afterProxyFinalizer := createAfterProxyFinalizer(&service, config)

	// set up before and after request interceptors (a.k.a. raptors 🦖🦖)

	// CachingMiddleware caches request in case of:
	//   - request isn't already cached
	//   - request is cacheable
	//   - response is present in context
	cacheAfterProxyMiddleware := serviceCache.CachingMiddleware(afterProxyFinalizer)

	// ProxyRequestMiddleware responds to the client with
	// - cached data if present in the context
	// - a forwarded request to the appropriate backend
	// Backend is decided by the Proxies configuration for a particular host.
	proxyMiddleware := createProxyRequestMiddleware(cacheAfterProxyMiddleware, config, serviceLogger, []RequestInterceptor{}, []RequestInterceptor{})

	// IsCachedMiddleware works in the following way:
	// - tries to get response from the cache
	//   - if present sets cached response in context, marks as cached in context and forwards to next middleware
	//   - if not present marks as uncached in context and forwards to next middleware
	cacheMiddleware := serviceCache.IsCachedMiddleware(proxyMiddleware)

	// BatchProcessingMiddleware separates a batch into multiple requests and routes each one
	// through the single request middleware sequence.
	// This allows the sub-requests of a batch to leverage the cache & metric recording.
	// Expects non-zero length batch to be in the context.
	batchMdwConfig := batchmdw.BatchMiddlewareConfig{
		ServiceLogger:                  serviceLogger,
		ContextKeyDecodedRequestBatch:  DecodedBatchRequestContextKey,
		ContextKeyDecodedRequestSingle: DecodedRequestContextKey,
		MaximumBatchSize:               config.ProxyMaximumBatchSize,
	}
	batchProcessingMiddleware := batchmdw.CreateBatchProcessingMiddleware(cacheMiddleware, &batchMdwConfig)

	// DecodeRequestMiddleware captures the request start time & attempts to decode the request body.
	// If successful, the decoded request is put into the request context:
	// - if decoded as a single EVM request: it forwards it to the single request middleware sequence
	// - if decoded as a batch EVM request: it forwards it to the batchProcessingMiddleware
	// - if fails to decode: it passes to single request middleware sequence which will proxy the request
	// When requests fail to decode, no context value is set.
	decodeRequestMiddleware := createDecodeRequestMiddleware(cacheMiddleware, batchProcessingMiddleware, serviceLogger)

	// register healthcheck handler that can be used during deployment and operations
	// to determine if the service is ready to receive requests
	mux.HandleFunc("/healthcheck", createHealthcheckHandler(&service))

	// register healthcheck handler that can be used during deployment and operations
	// to determine if the service is ready to receive requests
	mux.HandleFunc("/servicecheck", createServicecheckHandler(&service))

	// register middleware chain as the default handler for any request to the proxy service
	mux.HandleFunc("/", decodeRequestMiddleware)

	// create an http server for the caller to start on demand with a call to ProxyService.Run()
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", config.ProxyServicePort),
		Handler:      mux,
		WriteTimeout: time.Duration(config.HTTPWriteTimeoutSeconds) * time.Second,
		ReadTimeout:  time.Duration(config.HTTPReadTimeoutSeconds) * time.Second,
	}

	// register database status handler
	// for responding to requests for the status
	// of database related operations such as
	// proxied request metrics compaction and
	// partitioning
	mux.HandleFunc("/status/database", createDatabaseStatusHandler(&service, db))

	service = ProxyService{
		httpProxy:     server,
		ServiceLogger: serviceLogger,
		Database:      db,
		Cache:         serviceCache,
		evmClient:     evmClient,
	}

	return service, nil
}

// Register pprof handlers with authentication middleware
func registerPprofHandlers(config config.Config, mux *http.ServeMux) {
	pprofHandlers := map[string]http.HandlerFunc{
		// Provides a web interface for various pprof profiling tools
		// It serves as a convenient starting point to access all profiling data in one place.
		// http://localhost:7777/debug/pprof/
		"/debug/pprof/": pprof.Index,
		// Shows the command-line invocation of the running program.
		// Helps understand how the program was started, which can be useful for reproducing issues and
		// understanding runtime configurations.
		// http://localhost:7777/debug/pprof/cmdline
		"/debug/pprof/cmdline": pprof.Cmdline,
		// Generates a CPU profile
		// Allows developers to analyze CPU usage, identify bottlenecks, and optimize performance.
		// curl http://localhost:7777/debug/pprof/profile?seconds=30 > cpu_profile.prof
		// go tool pprof -http :8081 cpu_profile.prof
		"/debug/pprof/profile": pprof.Profile,
		// Provides a text-based symbol lookup service for profiling data.
		// Helps map program counters (addresses in the code) to function names, which is crucial for interpreting profiling data.
		// http://localhost:7777/debug/pprof/symbol
		"/debug/pprof/symbol": pprof.Symbol,
		// Provides a trace of the execution of the program, which includes information about goroutine activity and blocking.
		// Useful for detailed execution analysis, understanding how goroutines are scheduled, and diagnosing concurrency issues.
		// curl http://localhost:7777/debug/pprof/trace?seconds=30 > trace_profile.trace
		// go tool trace trace_profile.trace
		"/debug/pprof/trace": pprof.Trace,
		// Generates a heap profile that shows memory allocation details.
		// Helps identify memory usage patterns, detect memory leaks, and optimize memory usage. Use go tool pprof to analyze the heap profile.
		// curl http://localhost:7777/debug/pprof/heap?seconds=30 > heap_profile.prof
		// go tool pprof -http :8081 heap_profile.prof
		"/debug/pprof/heap": pprof.Handler("heap").ServeHTTP,
		// Shows goroutine stack traces.
		// curl http://localhost:7777/debug/pprof/goroutine?seconds=30 > goroutine_profile.prof
		// Useful for diagnosing deadlocks, understanding goroutine states, and analyzing concurrent execution.
		// go tool pprof -http :8081 goroutine_profile.prof
		"/debug/pprof/goroutine": pprof.Handler("goroutine").ServeHTTP,
		// Shows stack traces of created threads.
		// curl http://localhost:7777/debug/pprof/threadcreate?seconds=30 > threadcreate_profile.prof
		// Helps diagnose issues related to thread creation and understand the threading behavior of the program.
		// go tool pprof -http :8081 threadcreate_profile.prof
		"/debug/pprof/threadcreate": pprof.Handler("threadcreate").ServeHTTP,
		// Shows stack traces of goroutines that are blocked on synchronization primitives.
		// Useful for diagnosing blocking issues, such as deadlocks or contention on mutexes and other synchronization primitives.
		// curl http://localhost:7777/debug/pprof/block?seconds=30 > block_profile.prof
		// go tool pprof -http :8081 block_profile.prof
		"/debug/pprof/block": pprof.Handler("block").ServeHTTP,
	}

	for path, handler := range pprofHandlers {
		mux.Handle(path, authMiddleware(config, handler))
	}
}

func authMiddleware(config config.Config, next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != config.PprofUsername || pass != config.PprofPassword {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	}
}

// createDatabaseClient creates a connection to the database
// using the specified config and runs migrations async
// (only if migration flag in config is true) returning the
// returning the database connection and error (if any)
func createDatabaseClient(ctx context.Context, config config.Config, logger *logging.ServiceLogger) (*database.PostgresClient, error) {
	databaseConfig := database.PostgresDatabaseConfig{
		DatabaseName:                     config.DatabaseName,
		DatabaseEndpointURL:              config.DatabaseEndpointURL,
		DatabaseUsername:                 config.DatabaseUserName,
		DatabasePassword:                 config.DatabasePassword,
		SSLEnabled:                       config.DatabaseSSLEnabled,
		QueryLoggingEnabled:              config.DatabaseQueryLoggingEnabled,
		ReadTimeoutSeconds:               config.DatabaseReadTimeoutSeconds,
		WriteTimeousSeconds:              config.DatabaseWriteTimeoutSeconds,
		DatabaseMaxIdleConnections:       config.DatabaseMaxIdleConnections,
		DatabaseConnectionMaxIdleSeconds: config.DatabaseConnectionMaxIdleSeconds,
		DatabaseMaxOpenConnections:       config.DatabaseMaxOpenConnections,
		Logger:                           logger,
		RunDatabaseMigrations:            config.RunDatabaseMigrations,
	}

	serviceDatabase, err := database.NewPostgresClient(databaseConfig)

	if err != nil {
		logger.Error().Msg(fmt.Sprintf("error %s creating database using config %+v", err, databaseConfig))

		return &database.PostgresClient{}, err
	}

	if !databaseConfig.RunDatabaseMigrations {
		logger.Debug().Msg("skipping attempting to run migrations on database since RUN_DATABASE_MIGRATIONS was false")
		return &serviceDatabase, nil
	}

	// run migrations async so waiting for the database to
	// be reachable doesn't block the ability of the proxy service
	// to degrade gracefully and continue to proxy requests even
	// without it's database
	go func() {
		// wait for database to be reachable
		var databaseOnline bool
		for !databaseOnline {
			err = serviceDatabase.HealthCheck()

			if err != nil {
				logger.Debug().Msg("unable to connect to database, will retry in 1 second")

				time.Sleep(1 * time.Second)

				continue
			}

			logger.Debug().Msg("connected to database")

			databaseOnline = true
		}

		logger.Debug().Msg("running migrations on database")

		migrations, err := database.Migrate(ctx, serviceDatabase.DB, *migrations.Migrations, logger)

		if err != nil {
			// TODO: retry based on config
			logger.Error().Msg(fmt.Sprintf("error %s running migrations on database", err))
		}

		logger.Debug().Msg(fmt.Sprintf("run migrations %+v \n last group %+v \n unapplied %+v", migrations.Applied(), migrations.LastGroup(), migrations.Unapplied()))
	}()

	return &serviceDatabase, err
}

func createServiceCache(
	ctx context.Context,
	config config.Config,
	logger *logging.ServiceLogger,
	evmclient *ethclient.Client,
) (*cachemdw.ServiceCache, error) {
	cfg := cache.RedisConfig{
		Address:  config.RedisEndpointURL,
		Password: config.RedisPassword,
		DB:       0,
	}
	redisCache, err := cache.NewRedisCache(
		&cfg,
		logger,
	)
	if err != nil {
		logger.Error().Msg(fmt.Sprintf("error %s creating cache using endpoint %+v", err, config.RedisEndpointURL))
		return nil, err
	}

	cacheConfig := cachemdw.Config{
		CacheMethodHasBlockNumberParamTTL: config.CacheMethodHasBlockNumberParamTTL,
		CacheMethodHasBlockHashParamTTL:   config.CacheMethodHasBlockHashParamTTL,
		CacheStaticMethodTTL:              config.CacheStaticMethodTTL,
		CacheMethodHasTxHashParamTTL:      config.CacheMethodHasTxHashParamTTL,
	}

	serviceCache := cachemdw.NewServiceCache(
		redisCache,
		evmclient,
		DecodedRequestContextKey,
		config.CachePrefix,
		config.CacheEnabled,
		config.WhitelistedHeaders,
		config.DefaultAccessControlAllowOriginValue,
		config.HostnameToAccessControlAllowOriginValueMap,
		&cacheConfig,
		logger,
	)

	return serviceCache, nil
}

// Run runs the proxy service, returning error (if any) in the event
// the proxy service stops
func (p *ProxyService) Run() error {
	return p.httpProxy.ListenAndServe()
}
