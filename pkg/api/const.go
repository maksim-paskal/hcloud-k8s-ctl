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

const commonExecCommand = `#!/bin/bash
set -ex

export DEBIAN_FRONTEND=noninteractive

cd /root
rm -rf *
curl -sSL -o scripts.tar.gz \
https://github.com/maksim-paskal/hcloud-k8s-ctl/archive/refs/heads/main.tar.gz

tar -xvf scripts.tar.gz
mv ./hcloud-k8s-ctl-main/scripts ./scripts
`
