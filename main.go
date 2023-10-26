// package main reads & validates configuration for the proxy service
// and if the config is valid starts and monitors an instance of the
// proxy service and any background routines
package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/kava-labs/kava-proxy-service/config"
	"github.com/kava-labs/kava-proxy-service/logging"
	"github.com/kava-labs/kava-proxy-service/routines"
	"github.com/kava-labs/kava-proxy-service/service"
)

var (
	serviceConfig  config.Config
	serviceLogger  logging.ServiceLogger
	serviceContext = context.Background()
)

func init() {
	serviceConfig = config.ReadConfig()

	err := config.Validate(serviceConfig)

	if err != nil {
		panic(err)
	}

	serviceLogger, err = logging.New(serviceConfig.LogLevel)

	if err != nil {
		panic(err)
	}
}

func startMetricPartitioningRoutine(serviceConfig config.Config, service service.ProxyService, serviceLogger logging.ServiceLogger) <-chan error {
	metricPartitioningRoutineConfig := routines.MetricPartitioningRoutineConfig{
		Interval:          serviceConfig.MetricPartitioningRoutineInterval,
		DelayFirstRun:     serviceConfig.MetricPartitioningRoutineDelayFirstRun,
		PrefillPeriodDays: serviceConfig.MetricPartitioningPrefillPeriodDays,
		Database:          service.Database,
		Logger:            serviceLogger,
	}

	metricPartitioningRoutine, err := routines.NewMetricPartitioningRoutine(metricPartitioningRoutineConfig)

	if err != nil {
		serviceLogger.Error().Msg(fmt.Sprintf("error %s creating metric partitioning routine with config %+v", err, metricPartitioningRoutineConfig))

		return nil
	}

	errChan, err := metricPartitioningRoutine.Run()

	if err != nil {
		serviceLogger.Error().Msg(fmt.Sprintf("error %s starting metric partitioning routine with config %+v", err, metricPartitioningRoutineConfig))

		return nil
	}

	serviceLogger.Debug().Msg(fmt.Sprintf("started metric partitioning routine with config %+v", metricPartitioningRoutineConfig))

	return errChan
}

func startMetricCompactionRoutine(serviceConfig config.Config, service service.ProxyService, serviceLogger logging.ServiceLogger) <-chan error {
	metricCompactionRoutineConfig := routines.MetricCompactionRoutineConfig{
		Interval: serviceConfig.MetricCompactionRoutineInterval,
		Database: service.Database,
		Logger:   serviceLogger,
	}

	metricCompactionRoutine, err := routines.NewMetricCompactionRoutine(metricCompactionRoutineConfig)

	if err != nil {
		serviceLogger.Error().Msg(fmt.Sprintf("error %s creating metric compaction routine with config %+v", err, metricCompactionRoutineConfig))

		return nil
	}

	errChan, err := metricCompactionRoutine.Run()

	if err != nil {
		serviceLogger.Error().Msg(fmt.Sprintf("error %s starting metric compaction routine with config %+v", err, metricCompactionRoutineConfig))

		return nil
	}

	serviceLogger.Debug().Msg(fmt.Sprintf("started metric compaction routine with config %+v", metricCompactionRoutineConfig))

	return errChan
}

func startMetricPruningRoutine(serviceConfig config.Config, service service.ProxyService, serviceLogger logging.ServiceLogger) <-chan error {
	if !serviceConfig.MetricPruningEnabled {
		serviceLogger.Info().Msg("skipping starting metric pruning routine since it is disabled via config")
		return make(<-chan error)
	}

	metricPruningRoutineConfig := routines.MetricPruningRoutineConfig{
		Interval:                     serviceConfig.MetricPruningRoutineInterval,
		StartDelay:                   serviceConfig.MetricPartitioningRoutineDelayFirstRun,
		MaxRequestMetricsHistoryDays: int64(serviceConfig.MetricPruningMaxRequestMetricsHistoryDays),
		Database:                     service.Database,
		Logger:                       serviceLogger,
	}

	metricPruningRoutine, err := routines.NewMetricPruningRoutine(metricPruningRoutineConfig)

	if err != nil {
		serviceLogger.Error().Msg(fmt.Sprintf("error %s creating metric pruning routine with config %+v", err, metricPruningRoutineConfig))

		return nil
	}

	errChan, err := metricPruningRoutine.Run()

	if err != nil {
		serviceLogger.Error().Msg(fmt.Sprintf("error %s starting metric pruning routine with config %+v", err, metricPruningRoutineConfig))

		return nil
	}

	serviceLogger.Debug().Msg(fmt.Sprintf("started metric pruning routine with config %+v", metricPruningRoutineConfig))

	return errChan
}

func main() {
	serviceLogger.Debug().Msg(fmt.Sprintf("initial config: %+v", serviceConfig))

	// create the main proxy service
	service, err := service.New(serviceContext, serviceConfig, &serviceLogger)

	if err != nil {
		serviceLogger.Panic().Msg(fmt.Sprintf("%v", errors.Unwrap(err)))
	}

	// configure and run background routines
	// metric partitioning routine
	go func() {
		metricPartitioningErrs := startMetricPartitioningRoutine(serviceConfig, service, serviceLogger)

		for routineErr := range metricPartitioningErrs {
			serviceLogger.Error().Msg(fmt.Sprintf("metric partitioning routine encountered error %s", routineErr))
		}
	}()

	// metric compaction routine
	go func() {
		metricCompactionErrs := startMetricCompactionRoutine(serviceConfig, service, serviceLogger)

		for routineErr := range metricCompactionErrs {
			serviceLogger.Error().Msg(fmt.Sprintf("metric compaction routine encountered error %s", routineErr))
		}
	}()

	// metric pruning routine
	go func() {
		metricPruningErrs := startMetricPruningRoutine(serviceConfig, service, serviceLogger)

		for routineErr := range metricPruningErrs {
			serviceLogger.Error().Msg(fmt.Sprintf("metric pruning routine encountered error %s", routineErr))
		}
	}()

	// run the proxy service
	finalErr := service.Run()

	if finalErr != nil {
		serviceLogger.Debug().Msg(fmt.Sprintf("service stopped with error %s", finalErr))
	}
}
