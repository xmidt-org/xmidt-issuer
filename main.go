// SPDX-FileCopyrightText: 2019 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/InVisionApp/go-health"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/xmidt-org/candlelight"
	"github.com/xmidt-org/sallust"
	"github.com/xmidt-org/themis/config"
	"github.com/xmidt-org/themis/key"
	"github.com/xmidt-org/themis/random"
	"github.com/xmidt-org/themis/token"
	"github.com/xmidt-org/themis/xhealth"
	"github.com/xmidt-org/themis/xhttp/xhttpclient"
	"github.com/xmidt-org/themis/xhttp/xhttpserver"
	"github.com/xmidt-org/themis/xmetrics/xmetricshttp"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

const (
	applicationName = "themis"
)

var (
	GitCommit = "undefined"
	Version   = "undefined"
	BuildTime = "undefined"
)

func setupFlagSet(fs *pflag.FlagSet) error {
	fs.StringP("file", "f", "", "the configuration file to use.  Overrides the search path.")
	fs.Bool("dev", false, "development mode")
	fs.String("iss", "", "the name of the issuer to put into claims.  Overrides configuration.")
	fs.BoolP("debug", "d", false, "enables debug logging.  Overrides configuration.")
	fs.BoolP("version", "v", false, "print version and exit")

	return nil
}

func setupViper(in config.ViperIn, v *viper.Viper) (err error) {
	if printVersion, _ := in.FlagSet.GetBool("version"); printVersion {
		printVersionInfo()
	}
	if dev, _ := in.FlagSet.GetBool("dev"); dev {
		v.SetConfigType("yaml")
		err = v.ReadConfig(strings.NewReader(devMode))
	} else if file, _ := in.FlagSet.GetString("file"); len(file) > 0 {
		v.SetConfigFile(file)
		err = v.ReadInConfig()
	} else {
		v.SetConfigName(string(in.Name))
		v.AddConfigPath(fmt.Sprintf("/etc/%s", in.Name))
		v.AddConfigPath(".")
		v.AddConfigPath(fmt.Sprintf("$HOME/.%s", in.Name))
		err = v.ReadInConfig()
	}

	if err != nil {
		return
	}

	if iss, _ := in.FlagSet.GetString("iss"); len(iss) > 0 {
		v.Set("issuer.claims.iss", iss)
	}

	if debug, _ := in.FlagSet.GetBool("debug"); debug {
		v.Set("log.level", "DEBUG")
	}

	return
}

func main() {
	app := fx.New(
		sallust.WithLogger(),
		config.CommandLine{Name: applicationName}.Provide(setupFlagSet),
		provideMetrics(),
		fx.Provide(
			config.ProvideViper(setupViper),
			func(u config.Unmarshaller) (c sallust.Config, err error) {
				err = u.UnmarshalKey("log", &c)
				return
			},
			xhealth.Unmarshal("health"),
			random.Provide,
			key.Provide,
			token.Unmarshal("token"),
			xmetricshttp.Unmarshal("prometheus", promhttp.HandlerOpts{}),
			provideClientChain,
			provideServerChainFactory,
			xhttpclient.Unmarshal{Key: "client"}.Provide,
			xhttpserver.Unmarshal{Key: "servers.key", Optional: true}.Annotated(),
			xhttpserver.Unmarshal{Key: "servers.issuer", Optional: true}.Annotated(),
			xhttpserver.Unmarshal{Key: "servers.claims", Optional: true}.Annotated(),
			xhttpserver.Unmarshal{Key: "servers.metrics", Optional: true}.Annotated(),
			xhttpserver.Unmarshal{Key: "servers.health", Optional: true}.Annotated(),
			xhttpserver.Unmarshal{Key: "servers.pprof", Optional: true}.Annotated(),
			candlelight.New,
			func(u config.Unmarshaller) (candlelight.Config, error) {
				var config candlelight.Config
				err := u.UnmarshalKey("tracing", &config)
				if err != nil {
					return candlelight.Config{}, err
				}
				config.ApplicationName = applicationName
				return config, nil
			},
		),
		fx.Invoke(
			xhealth.ApplyChecks(
				&health.Config{
					Name:     applicationName,
					Interval: 24 * time.Hour,
					Checker: xhealth.NopCheckable{
						Details: map[string]interface{}{
							"StartTime": time.Now().UTC().Format(time.RFC3339),
						},
					},
				},
			),
			BuildKeyRoutes,
			BuildIssuerRoutes,
			BuildClaimsRoutes,
			BuildMetricsRoutes,
			BuildHealthRoutes,
			BuildPprofRoutes,
			CheckServerRequirements,
		),
	)
	err := app.Err()
	if errors.Is(err, pflag.ErrHelp) {
		return
	} else if errors.Is(err, nil) {
		app.Run()
	} else {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func printVersionInfo() {
	fmt.Fprintf(os.Stdout, "%s:\n", applicationName)
	fmt.Fprintf(os.Stdout, "  version: \t%s\n", Version)
	fmt.Fprintf(os.Stdout, "  go version: \t%s\n", runtime.Version())
	fmt.Fprintf(os.Stdout, "  built time: \t%s\n", BuildTime)
	fmt.Fprintf(os.Stdout, "  git commit: \t%s\n", GitCommit)
	fmt.Fprintf(os.Stdout, "  os/arch: \t%s/%s\n", runtime.GOOS, runtime.GOARCH)
	os.Exit(0)
}
