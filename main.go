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

func main() {
	serviceLogger.Debug().Msg(fmt.Sprintf("initial config: %+v", serviceConfig))

	// create the main proxy service
	service, err := service.New(serviceContext, serviceConfig, &serviceLogger)

	if err != nil {
		serviceLogger.Panic().Msg(fmt.Sprintf("%v", errors.Unwrap(err)))
	}

	// configure and run background routines
	go func() {
		metricCompactionRoutineConfig := routines.MetricCompactionRoutineConfig{
			Interval: serviceConfig.MetricCompactionRoutineInterval,
			Database: service.Database,
			Logger:   serviceLogger,
		}

		metricCompactionRoutine, err := routines.NewMetricCompactionRoutine(metricCompactionRoutineConfig)

		if err != nil {
			serviceLogger.Error().Msg(fmt.Sprintf("error %s creating metric compaction routine with config %+v", err, metricCompactionRoutineConfig))

			return
		}

		errChan, err := metricCompactionRoutine.Run()

		if err != nil {
			serviceLogger.Error().Msg(fmt.Sprintf("error %s starting metric compaction routine with config %+v", err, metricCompactionRoutineConfig))

			return
		}

		serviceLogger.Debug().Msg(fmt.Sprintf("started metric compaction routine with config %+v", metricCompactionRoutineConfig))

		for routineErr := range errChan {
			serviceLogger.Error().Msg(fmt.Sprintf("metric compaction routine encountered error %s", routineErr))
		}
	}()

	// run the proxy service
	finalErr := service.Run()

	if finalErr != nil {
		serviceLogger.Debug().Msg(fmt.Sprintf("service stopped with error %s", finalErr))
	}
}
