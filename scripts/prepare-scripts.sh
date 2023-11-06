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

GO_TEMPLATE_VERSION="0.1.2"
OS_ARCH=$(dpkg --print-architecture)

# use go-binary to template script files
curl -sSL -o /usr/local/bin/go-template "https://github.com/maksim-paskal/go-template/releases/download/v${GO_TEMPLATE_VERSION}/go-template_${GO_TEMPLATE_VERSION}_linux_${OS_ARCH}"
chmod +x /usr/local/bin/go-template

# create checksum file
touch /tmp/checksum
if [ "${OS_ARCH}" == "amd64" ]; then
    echo "65d6b1e296dafb062c785c5a14135beeca953b11c577a70da74e5071793a4120  /usr/local/bin/go-template" >> /tmp/checksum
fi

if [ "${OS_ARCH}" == "arm64" ]; then
    echo "c70ad5472a7a4db5ee9fd2593ebbad1437f345c9ee4a0fda3ba688199350e277  /usr/local/bin/go-template" >> /tmp/checksum
fi

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
