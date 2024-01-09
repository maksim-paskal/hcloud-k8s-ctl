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
package version

import (
	"context"
	"net/http"
	"strings"
	"time"

	semver "github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

var versionClient = http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

const (
	httpTimeout         = 10 * time.Second
	githubURL           = "https://github.com/maksim-paskal/hcloud-k8s-ctl"
	githubVersionLatest = githubURL + "/releases/latest"
	githubVersionPrefix = githubURL + "/releases/tag/"
)

func CheckLatest(ctx context.Context, myVersion string) error {
	ctx, cancel := context.WithTimeout(ctx, httpTimeout)
	defer cancel()

	currentVersion, err := semver.NewSemver(myVersion)
	if err != nil {
		return errors.Wrap(err, "error parse version")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubVersionLatest, nil)
	if err != nil {
		return errors.Wrap(err, "error create request")
	}

	resp, err := versionClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "error do request")
	}

	defer resp.Body.Close()

	versionURL := resp.Header.Get("Location")

	latestTag := strings.TrimPrefix(versionURL, githubVersionPrefix)

	versionGithub, err := semver.NewSemver(latestTag)
	if err != nil {
		return errors.Wrap(err, "error parse version")
	}

	if currentVersion.Core().LessThan(versionGithub) {
		log.Infof("new version available: %s", versionURL)
	}

	return nil
}
