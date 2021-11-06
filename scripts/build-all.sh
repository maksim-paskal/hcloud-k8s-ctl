#!/usr/bin/env bash

# Copyright paskal.maksim@gmail.com
#
# Licensed under the Apache License, Version 2.0 (the "License")
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -euo pipefail

export CGO_ENABLED=0
export GO111MODULE=on
export TAGS=""
export GOFLAGS="-trimpath"
export LDFLAGS="-X main.gitVersion=$(git describe --tags $(git rev-list --tags --max-count=1))-$(date +%Y%m%d%H%M%S)-$(git log -n1 --pretty='%h')"
export TARGETS="darwin/amd64 linux/amd64"
export BINNAME="hcloud-k8s-ctl"
export GOX="go run github.com/mitchellh/gox"

rm -rf _dist

go get github.com/mitchellh/gox

$GOX -parallel=3 -output="_dist/$BINNAME-{{.OS}}-{{.Arch}}" -osarch="$TARGETS" -tags "$TAGS" -ldflags "$LDFLAGS" ./cmd

shasum -a 256 ./_dist/$BINNAME* > ./_dist/sha256.txt