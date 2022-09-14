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
set -ex

GO_TEMPLATE_VERSION=0.1.1

# use go-binary to template script files
curl -sSL -o /usr/local/bin/go-template https://github.com/maksim-paskal/go-template/releases/download/v${GO_TEMPLATE_VERSION}/go-template_${GO_TEMPLATE_VERSION}_linux_amd64
chmod +x /usr/local/bin/go-template

# create checksum file
touch /tmp/checksum
echo "59d980c91c52b2e2f0195aadd8b21d2ce1e9e514defb66e5f5d5495e804211aa  /usr/local/bin/go-template" >> /tmp/checksum

# test downloaded script with sha256sum
sha256sum -c /tmp/checksum

# remove temp file
rm /tmp/checksum

# test binary
go-template -version

# template config files
mv /root/scripts/common-install.sh /root/scripts/common-install.sh.tmpl

# template common-install.sh script
go-template \
-file /root/scripts/common-install.sh.tmpl \
--values /root/values.yaml \
> /root/scripts/common-install.sh

# make scripts executables
chmod +x /root/scripts/*.sh
