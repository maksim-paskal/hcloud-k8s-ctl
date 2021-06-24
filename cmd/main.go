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
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/maksim-paskal/hcloud-k8s-ctl/pkg/api"
	"github.com/maksim-paskal/hcloud-k8s-ctl/pkg/config"
	log "github.com/sirupsen/logrus"
)

//nolint:gochecknoglobals
var (
	gitVersion  = "dev"
	versionFlag = flag.Bool("version", false, "version")
)

func main() { //nolint:cyclop
	flag.Parse()

	if *versionFlag {
		fmt.Println(gitVersion) //nolint:forbidigo
		os.Exit(0)
	}

	log.Infof("Starting %s...", gitVersion)

	log.SetReportCaller(true)

	applicationConfig := config.NewApplicationConfig()

	err := applicationConfig.Load()
	if err != nil {
		log.WithError(err).Fatal("error loading config")
	}

	logLevel, err := log.ParseLevel(*applicationConfig.Get().CliArgs.LogLevel)
	if err != nil {
		log.WithError(err).Fatal()
	}

	log.SetLevel(logLevel)

	log.Infof("Loaded config:\n%s\n", applicationConfig.String())

	applicationAPI, err := api.NewApplicationAPI(applicationConfig)
	if err != nil {
		log.Fatal(err)
	}

	switch strings.ToLower(*applicationConfig.Get().CliArgs.Action) {
	case "create":
		err = applicationAPI.NewCluster()
		if err != nil {
			log.WithError(err).Fatal()
		}
	case "delete":
		applicationAPI.DeleteCluster()
	case "list-configurations":
		applicationAPI.ListConfigurations()
	case "patch-cluster":
		applicationAPI.PatchClusterDeployment()
	default:
		log.Fatal("unknown action")
	}
}
