/*
Copyright paskal.maksim@gmail.com
Licensed under the Apache License, Version 2.0 (the "License")
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/maksim-paskal/hcloud-k8s-ctl/internal"
	"github.com/maksim-paskal/hcloud-k8s-ctl/pkg/api"
	"github.com/maksim-paskal/hcloud-k8s-ctl/pkg/config"
	"github.com/maksim-paskal/hcloud-k8s-ctl/pkg/version"
	log "github.com/sirupsen/logrus"
)

//nolint:gochecknoglobals
var (
	gitVersion      = "dev"
	versionFlag     = flag.Bool("version", false, "version")
	checkNewVersion = flag.Bool("check-new-version", true, "check new version")
)

func main() { //nolint:cyclop,funlen
	flag.Parse()

	if *versionFlag {
		fmt.Println(gitVersion) //nolint:forbidigo
		os.Exit(0)
	}

	ctx := getInterruptionContext()

	log.Infof("Starting %s...", gitVersion)

	log.SetReportCaller(true)

	if err := internal.Init(); err != nil {
		log.WithError(err).Fatal("error loading config")
	}

	logLevel, err := log.ParseLevel(*config.Get().CliArgs.LogLevel)
	if err != nil {
		log.WithError(err).Fatal()
	}

	log.SetLevel(logLevel)

	if *checkNewVersion {
		if err := version.CheckLatest(ctx, gitVersion); err != nil {
			log.WithError(err).Warn("error check latest version")
		}
	}

	if err := config.Check(); err != nil {
		log.WithError(err).Fatal("error checking config")
	}

	applicationAPI, err := api.NewApplicationAPI(ctx)
	if err != nil {
		log.Fatal(err)
	}

	switch strings.ToLower(*config.Get().CliArgs.Action) {
	case "create":
		err = applicationAPI.NewCluster(ctx)
		if err != nil {
			log.WithError(err).Fatal()
		}
	case "delete":
		applicationAPI.DeleteCluster(ctx)
	case "list-configurations":
		applicationAPI.ListConfigurations(ctx)
	case "patch-cluster":
		err = applicationAPI.PatchClusterDeployment(ctx)
		if err != nil {
			log.WithError(err).Fatal()
		}
	case "adhoc":
		if len(*config.Get().CliArgs.AdhocCommand) == 0 {
			log.Fatal("add -adhoc.command argument")
		}

		applicationAPI.ExecuteAdHoc(
			ctx,
			*config.Get().CliArgs.AdhocUser,
			*config.Get().CliArgs.AdhocCommand,
			*config.Get().CliArgs.AdhocMasters,
			*config.Get().CliArgs.AdhocWorkers,
			*config.Get().CliArgs.AdhocCopyNewFile,
		)
	case "upgrade-controlplane":
		applicationAPI.UpgradeControlPlane(ctx)
	case "create-firewall":
		err = applicationAPI.CreateFirewall(
			ctx,
			*config.Get().CliArgs.CreateFirewallControlPlane,
			*config.Get().CliArgs.CreateFirewallWorkers,
		)
		if err != nil {
			log.Fatal(err)
		}
	case "save-full-config":
		err = config.SaveConfig(*config.Get().CliArgs.SaveConfigPath)
		if err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatal("unknown action")
	}
}

func getInterruptionContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		log.Warn("Received interruption signal, stopping...")
		cancel()

		// wait 5s for graceful shutdown
		time.Sleep(5 * time.Second) //nolint:gomnd

		os.Exit(1)
	}()

	return ctx
}
