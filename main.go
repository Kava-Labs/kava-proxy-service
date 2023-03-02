// package main reads & validates configuration for the proxy service
// and if the config is valid starts and monitors an instance of the proxy service
package main

import (
	"fmt"
	"time"

	"github.com/kava-labs/kava-proxy-service/config"
	"github.com/kava-labs/kava-proxy-service/logging"
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

	for {
		serviceLogger.Info().Msg("There and back again")

		time.Sleep(2 * time.Second)
	}
}
