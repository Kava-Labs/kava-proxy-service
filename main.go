// package main reads & validates configuration for the proxy service
// and if the config is valid starts and monitors an instance of the proxy service
package main

import (
	"errors"
	"fmt"

	"github.com/kava-labs/kava-proxy-service/config"
	"github.com/kava-labs/kava-proxy-service/logging"
	"github.com/kava-labs/kava-proxy-service/service"
)

var (
	serviceConfig config.Config
	serviceLogger logging.ServiceLogger
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

	service, err := service.New(serviceConfig, &serviceLogger)

	if err != nil {
		serviceLogger.Panic().Msg(fmt.Sprintf("%v", errors.Unwrap(err)))
	}

	service.Run()
}
