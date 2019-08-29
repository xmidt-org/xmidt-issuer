// Copyright 2019 Comcast Cable Communications Management, LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/xmidt-org/themis/config"
	"github.com/xmidt-org/themis/key"
	"github.com/xmidt-org/themis/random"
	"github.com/xmidt-org/themis/token"
	"github.com/xmidt-org/themis/xhealth"
	"github.com/xmidt-org/themis/xhttp/xhttpclient"
	"github.com/xmidt-org/themis/xhttp/xhttpserver"
	"github.com/xmidt-org/themis/xlog"
	"github.com/xmidt-org/themis/xlog/xloghttp"
	"github.com/xmidt-org/themis/xmetrics/xmetricshttp"

	"github.com/spf13/pflag"
	"go.uber.org/fx"
)

const (
	applicationName    = "themis"
	applicationVersion = "0.0.3"
)

func initialize(e config.Environment) error {
	var (
		file = e.FlagSet.StringP("file", "f", "", "the configuration file to use.  Overrides the search path.")
		dev  = e.FlagSet.Bool("dev", false, "development mode")
		iss  = e.FlagSet.String("iss", "", "the name of the issuer to put into claims.  Overrides configuration.")
	)

	e.FlagSet.BoolP("enable-app-logging", "e", false, "enables logging output from the uber/fx App")

	err := e.FlagSet.Parse(e.Arguments)
	if err != nil {
		return err
	}

	switch {
	case *dev:
		e.Viper.SetConfigType("yaml")
		err = e.Viper.ReadConfig(strings.NewReader(devMode))

	case len(*file) > 0:
		e.Viper.SetConfigFile(*file)
		err = e.Viper.ReadInConfig()

	default:
		e.Viper.SetConfigName(e.Name)
		e.Viper.AddConfigPath(".")
		e.Viper.AddConfigPath(fmt.Sprintf("$HOME/.%s", e.Name))
		e.Viper.AddConfigPath(fmt.Sprintf("/etc/%s", e.Name))
		err = e.Viper.ReadInConfig()
	}

	if err != nil {
		return err
	}

	if len(*iss) > 0 {
		e.Viper.Set("issuer.claims.iss", *iss)
	}

	return nil
}

func createPrinter(logger log.Logger, e config.Environment) fx.Printer {
	if v, err := e.FlagSet.GetBool("enable-app-logging"); v && err == nil {
		return xlog.Printer{Logger: logger}
	}

	return config.DiscardPrinter{}
}

func main() {
	app := fx.New(
		config.Bootstrap{
			Name: applicationName,
		}.Provide(
			initialize,
			xlog.Unmarshaller("log", createPrinter),
			config.IfKeySet("servers.key",
				fx.Provide(
					fx.Annotated{
						Name:   "servers.key",
						Target: xhttpserver.Unmarshal("servers.key"),
					},
				),
				fx.Invoke(BuildKeyRoutes),
			),
			config.IfKeySet("servers.issuer",
				fx.Provide(
					fx.Annotated{
						Name:   "servers.issuer",
						Target: xhttpserver.Unmarshal("servers.issuer"),
					},
				),
				fx.Invoke(BuildIssuerRoutes),
			),
			config.IfKeySet("servers.claims",
				fx.Provide(
					fx.Annotated{
						Name:   "servers.claims",
						Target: xhttpserver.Unmarshal("servers.claims"),
					},
				),
				fx.Invoke(BuildClaimsRoutes),
			),
			config.IfKeySet("servers.metrics",
				fx.Provide(
					fx.Annotated{
						Name:   "servers.metrics",
						Target: xhttpserver.Unmarshal("servers.metrics"),
					},
				),
				fx.Invoke(BuildMetricsRoutes),
			),
			config.IfKeySet("servers.health",
				fx.Provide(
					fx.Annotated{
						Name:   "servers.health",
						Target: xhttpserver.Unmarshal("servers.health"),
					},
				),
				fx.Invoke(BuildHealthRoutes),
			),
		),
		provideMetrics(),
		fx.Provide(
			xhealth.Unmarshal("health"),
			random.Provide,
			key.Provide,
			token.Unmarshal("token"),
			xmetricshttp.Unmarshal("prometheus", promhttp.HandlerOpts{}),
			xloghttp.ProvideStandardBuilders,
			provideClientChain,
			provideServerChainFactory,
			xhttpclient.Unmarshal("client"),
		),
	)

	if err := app.Err(); err != nil {
		if err == pflag.ErrHelp {
			return
		}

		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	app.Run()
}
