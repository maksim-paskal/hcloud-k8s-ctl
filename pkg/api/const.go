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
package api

import (
	"io/fs"
	"time"
)

const commonExecCommand = `#!/bin/bash
set -ex

export DEBIAN_FRONTEND=noninteractive

cd /root
rm -rf *
curl -sSL -o scripts.tar.gz \
%s

tar -xvf scripts.tar.gz
mv ./%s/scripts ./scripts

# make scripts executables
chmod +x /root/scripts/*.sh

# initial config
echo "%s" | base64 -d > /root/values.yaml

# prepare scripts
/root/scripts/prepare-scripts.sh
`

const kubeconfigFileMode = fs.FileMode(0o600)

const (
	hcloudLoadBalancerInterval = 15 * time.Second
	hcloudLoadBalancerTimeout  = 10 * time.Second
	hcloudLoadBalancerRetries  = 3
)
