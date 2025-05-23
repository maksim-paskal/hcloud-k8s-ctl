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

export KUBERNETES_VERSION="{{ .Values.serverComponents.kubernetes.version }}"
export DOCKER_VERSION="{{ .Values.serverComponents.docker.version }}"
export CONTAINERD_VERSION="{{ .Values.serverComponents.containerd.version }}"
export PAUSE_CONTAINER="{{ .Values.serverComponents.containerd.pausecontainer }}"

# https://containerd.io/releases/#kubernetes-support
# to select all available versions, run
# make make ubuntu-versions

export DEBIAN_FRONTEND=noninteractive
export HOME=/root/

# uninstall old versions if exists
dpkg --purge docker docker-engine docker.io containerd runc

# remove old kubernetes sources
rm -rf /etc/apt/sources.list.d/kubernetes-*

apt update
apt install -y \
apt-transport-https \
ca-certificates \
curl \
software-properties-common \
nfs-common \
linux-headers-generic \
lsb-release

# create new user to ssh into server
hcloud_user="{{ .Values.serverComponents.ubuntu.username }}"
if ! id -u "$hcloud_user" > /dev/null 2>&1; then
  groupadd --gid 1000 $hcloud_user
  useradd -rm -d /home/$hcloud_user -s /bin/bash -g 1000 -u 1000 $hcloud_user
  mkdir -p /home/$hcloud_user/.ssh
  cp /root/.ssh/authorized_keys /home/$hcloud_user/.ssh
  chown -R $hcloud_user:$hcloud_user /home/$hcloud_user
  echo "$hcloud_user ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers
fi

# remove root ssh access
cat <<EOF | tee /etc/ssh/sshd_config 
AllowUsers $hcloud_user

PermitRootLogin no
PasswordAuthentication no
ChallengeResponseAuthentication no
UsePAM yes
AuthenticationMethods publickey
PubkeyAuthentication yes
PermitEmptyPasswords no

PrintMotd no
AcceptEnv LANG LC_*
Subsystem sftp /usr/lib/openssh/sftp-server

AllowTcpForwarding no
X11Forwarding no
AllowAgentForwarding no
EOF

# restart sshd to apply new config
mkdir -p /run/sshd
sshd -t
systemctl restart ssh.service

# disable swap
swapoff -a

# disable 111/udp 111/tcp port
systemctl stop rpcbind.service rpcbind.socket rpcbind.target
systemctl disable rpcbind.service rpcbind.socket rpcbind.target

mkdir -p /etc/apt/keyrings

rm -rf /usr/share/keyrings/docker-archive-keyring.gpg /usr/share/keyrings/kubernetes-archive-keyring.gpg

# add docker gpg key
# curl -fsSL https://download.docker.com/linux/ubuntu/gpg | base64
echo LS0tLS1CRUdJTiBQR1AgUFVCTElDIEtFWSBCTE9DSy0tLS0tCgptUUlOQkZpdDJpb0JFQURoV3BaOC93dlo2aFVUaVhPd1FIWE1BbGFGSGNQSDloQXRyNEYxeTIrT1lkYnRNdXRoCmxxcXdwMDI4QXF5WStQUmZWTXRTWU1ianVRdXU1Ynl5S1IwMUJicVlodVMzanRxUW1salovYkp2WHFubWlWWGgKMzhVdUxhK3owNzdQeHl4UWh1NUJicW50VFBRTWZpeXFFaVUrQkticTJXbUFOVUtRZisxQW1aWS9JcnVPWGJucQpMNEMxK2dKOHZmbVhRdDk5bnBDYXhFamFOUlZZZk9TOFFjaXhOekhVWW5iNmVtamxBTnlFVmxaemVxbzdYS2w3ClVyd1Y1aW5hd1RTeldOdnRqRWpqNG5KTDhOc0x3c2NwTFBRVWhUUSs3QmJRWEF3QW1lSENVVFFJdnZXWHF3ME4KY21oaDRIZ2VRc2NRSFlnT0pqakRWZm9ZNU11Y3ZnbGJJZ0NxZnpBSFc5anhtUkw0cWJNWmorYjFYb2VQRXRodAprdTRiSVFOMVg1UDA3Zk5XemxnYVJMNVo0UE9YRERaVGxJUS9FbDU4ajlrcDRibldSQ0pXMGx5YStmOG9jb2RvCnZaWitEb2krZnk0RDVaR3JMNFhFY0lRUC9MdjV1RnlmK2tRdGwvOTRWRllWSk9sZUF2OFc5MktkZ0RraFRjVEQKRzdjMHRJa1ZFS05VcTQ4YjNhUTY0Tk9aUVc3ZlZqZm9Ld0VaZE9xUEU3MlBhNDVqclp6dlVGeFNwZGlOazJ0WgpYWXVrSGpseHhFZ0JkQy9KM2NNTU5SRTFGNE5DQTNBcGZWMVk3L2hUZU9ubUR1RFl3cjkvb2JBOHQwMTZZbGpqCnE1cmRreXdQZjRKRjhtWFVXNWVDTjF2QUZIeGVnOVpXZW1oQnRRbUd4WG53OU0rejZoV3djNmFobXdBUkFRQUIKdEN0RWIyTnJaWElnVW1Wc1pXRnpaU0FvUTBVZ1pHVmlLU0E4Wkc5amEyVnlRR1J2WTJ0bGNpNWpiMjAraVFJMwpCQk1CQ2dBaEJRSllyZWZBQWhzdkJRc0pDQWNEQlJVS0NRZ0xCUllDQXdFQUFoNEJBaGVBQUFvSkVJMkJnRHdPCnY4Mklzc2tQL2lRWm82OGZsRFFtTnZuOFg1WFRkNlJSYVVIMzNrWFlYcXVUNk5rSEpjaVM3RTJnVEptcXZNcWQKdEk0bU5ZSENTRVl4STVxcmNZVjVZcVg5UDYrS28rdm96bzRuc2VVUUxQSC9BVFE0cUwwWm9rKzFqa2FnM0xnawpqb255VWY5Ynd0V3hGcDA1SEMzR01IUGhoY1VTZXhDeFFMUXZuRldYRDJzV0xLaXZIcDJmVDhRYlJHZVorZDNtCjZmcWNkNUZ1N3B4c3FtMEVVREs1TkwrblBJZ1loTithdVRyaGd6aEsxQ1NoZkdjY00vd2ZSbGVpOVV0ejZwOVAKWFJLSWxXblh0VDRxTkdaTlROMHRSK05MRy82QnFkOE9ZQmFGQVVjdWUvdzFWVzZKUTJWR1laSG5adTlTOExNYwpGWUJhNUlnOVB4d0dRT2dxNlJES0RiVitQcVRRVDVFRk1lUjFtcmpja2s0RFFKamJ4ZU1aYmlOTUc1a0dFQ0E4CmczODNQM2VsaG4wM1dHYkVFYTRNTmMzWjQrN2MyMzZRSTN4V0pmTlBkVWJYUmFBd2h5LzZyVFNGYnp3S0IwSm0KZWJ3elFmd2pRWTZmNTVNaUkvUnFEQ3l1UGozcjNqeVZSa0s4NnBRS0JBSndGSHlxajlLYUtYTVpqZlZub3dMaAo5c3ZJR2ZOYkdIcHVjQVRxUkV2VUh1UWJObnFrQ3g4VlZodFlraERiOWZFUDJ4QnU1VnZIYlIrM25mVmhNdXQ1CkczNEN0NVJTN0p0NkxJZkZkdGNuOENhU2FzL2wxSGJpR2VSZ2M3MFgvOWFZeC9WL0NFSnYwbEllOGdQNnVEb1cKRlBJWjdkNnZIK1ZybzZ4dVdFR2l1TWFpem5hcDJLaFptcGtnZnVweUZtcGxoMHM2a255bXVRSU5CRml0MmlvQgpFQURuZUw5UzltNHZoVTNibGFSalZVVXlKN2IvcVRqY1N5bHZDSDVYVUU2UjJrK2NrRVpqZkFNWlBMcE8rL3RGCk0ySklKTUQ0U2lmS3VTM3hjazlLdFpHQ3VmR21jd2lMUVJ6ZUhGN3ZKVUtyTEQ1UlRrTmkyM3lkdldaZ1BqdHgKUStEVFQxWmNuN0JyUUZZNkZnblJvVVZJeHd0ZHcxYk1ZLzg5cnNGZ1M1d3d1TUVTZDNRMlJZZ2I3RU9GT3BudQp3NmRhN1dha1dmNElobkY1bnNOWUdEVmFJSHpwaXFDbCt1VGJmMWVwQ2pyT2xJemtaM1ozWWs1Q00vVGlGelBrCnoybEx6ODljcEQ4VStOdENzZmFnV1dmamQyVTNqRGFwZ0grN25RbkNFV3BST3R6YUtIRzZsQTNwWGRpeDV6RzgKZVJjNi8wSWJVU1d2ZmpLeExMUGZOZUNTMnBDTDNJZUVJNW5vdGhFRVlkUUg2c3pwTG9nNzl4QjlkVm5KeUtKYgpWZnhYbnNlb1lxVnJSejJWVmJVSTVCbHdtNkI0MEUzZUdWZlVRV2l1eDU0RHNweVZNTWs0MU14N1FKM2l5bklhCjFONFpBcVZNQUVydXlYVFJUeGM5WFcwdFloRE1BLzFHWXZ6MEVtRnBtOEx6VEhBNnNGVnRQbS9abE5DWDZQMVgKekp3cnY3RFNRS0Q2R0dsQlFVWCtPZUVKOHRUa2tmOFFUSlNQVWRoOFA4WXhERlM1RU9HQXZoaHBNQllENDJrUQpwcVhqRUMrWGN5Y1R2R0k3aW1wZ3Y5UERZMVJDQzF6a0JqS1BhMTIwck5odi9oa1ZrL1lodUdvYWpvSHl5NGg3ClpRb3BkY010cE4yZGdtaEVlZ255OUpDU3d4ZlFtUTB6SzBnN202U0hpS013andBUkFRQUJpUVErQkJnQkNBQUoKQlFKWXJkb3FBaHNDQWlrSkVJMkJnRHdPdjgySXdWMGdCQmtCQ0FBR0JRSllyZG9xQUFvSkVINmdxY1B5Yy96WQoxV0FQLzJ3SitSMGdFNnFzY2UzcmphSXo1OFBKbWM4Z29LcmlyNWhuRWxXaFBnYnE3Y1lJc1c1cWlGeUxoa2RwClljTW1oRDltUmlQcFFuNllhMnczZTNCOHpmSVZLaXBiTUJua2UveXRaOU03cUhtRENjam9pU213RVhOM3dLWUkKbUQ5VkhPTnNsL0NHMXJVOUlzdzFqdEI1ZzFZeHVCQTdNL20zNlhONngydStOdE5NREI5UDU2eWM0Z2ZzWlZFUwpLQTl2K3lZMi9sNDVMOGQvV1VrVWkwWVhvbW42aHlCR0k3SnJCTHEwQ1gzN0dFWVA2TzlycktpcGZ6NzNYZk83CkpJR3pPS1psbGpiL0Q5UlgvZzduUmJDbiszRXRIN3huaytUSy81MGV1RUt3OFNNVWcxNDdzSlRjcFFtdjZVeloKY000SmdMMEhiSFZDb2pWNEMvcGxFTHdNZGRBTE9GZVlRelRpZjZzTVJQZiszRFNqOGZyYkluakNoQzN5T0x5MAo2YnI5MktGb20xN0VJajJDQWNvZXE3VVBoaTJvb3VZQndQeGg1eXRkZWhKa29vK3NON1JJV3VhNlAyV1Ntb241ClU4ODhjU3lsWEMwK0FERmRnTFg5SzJ6ckRWWVVHMXZvOENYMHZ6eEZCYUh3TjZQeDI2ZmhJVDEvaFlVSFFSMXoKVmZORGN5UW1YcWtPblp2dm9NZnovUTBzOUJoRkovelU2QWdRYklaRS9obTFzcHNmZ3Z0c0QxZnJaZnlnWEo5ZgppclArTVNBSTgweEhTZjkxcVNSWk9qNFBsM1pKTmJxNHlZeHYwYjFwa01xZUdkamRDWWhMVStMWjR3YlFtcENrClNWZTJwcmxMdXJlaWdYdG1aZmtxZXZSejdGcklaaXU5a3k4d25DQVB3Qzcvem1TMThyZ1AvMTdiT3RMNC9pSXoKUWh4QUFvQU1XVnJHeUppdlNramhTR3gxdUNvanNXZnNUQW0xMVA3anNydUlMNjFaek1VVkUyYU0zUG1qNUcrVwo5QWNaNThFbSsxV3NWbkFYZFVSLy9iTW1oeXI4d0wvRzFZTzFWM0pFSlRSZHhzU3hkWWE0ZGVHQkJZL0FkcHN3CjI0anhoT0pSK2xzSnBxSVVlYjk5OStSOGV1RGhSSEc5ZUZPN0RSdTZ3ZWF0VUo2c3V1cG9EVFJXdHIvNHlHcWUKZEt4VjNxUWhOTFNuYUF6cVcvMW5BM2lVQjRrN2tDYUtaeGhkaERiQ2xmOVAzN3FhUlc0NjdCTENWTy9jb0wzeQpWbTUwZHdkck50S3BNQmgzWnBiQjF1SnZnaTltWHR5Qk9NSjN2OFJaZUR6RmlHOEhkQ3RnOVJ2SXQvQUlGb0hSCkgzUytVNzlOVDZpMEtQekxJbURmczhUN1JscHl1TWM0VWZzOGdneWc5djNBZTZjTjNlUXl4Y0szdzBjYkJ3c2gKL25RTmZzQTZ1dSs5SDdOaGJlaEJNaFlucE5aeXJIekNtenlYa2F1d1JBcW9DYkdDTnlrVFJ3c3VyOWdTNDFUUQpNOHNzRDFqRmhlT0pmM2hPRG5rS1UrSEtqdk1ST2wxREs3emRtTGROekExY3Z0WkgvbkNDOUtQajF6OFFDNDdTCnh4K2RUWlN4NE9OQWh3YlMvTE4zUG9LdG44TFBqWTlOUDl1RFdJK1RXWXF1UzJVK0tIRHJCRGxzZ296RGJzL08KakN4Y3BEek5tWHBXUUhFdEhVNzY0OU9YSFA3VWVOU1QxbUNVQ0g1cWRhbmswVjFpZWpGNi9DZlRGVTRNZmNyRwpZVDkwcUZGOTNNM3YwMUJieFArRUlZMi85dGlJUGJyZAo9MFlZaAotLS0tLUVORCBQR1AgUFVCTElDIEtFWSBCTE9DSy0tLS0tCg== \
| base64 -d \
| gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg

# add kubernetes gpg key
# curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.31/deb/Release.key | base64
# expires 2026-12-29
echo LS0tLS1CRUdJTiBQR1AgUFVCTElDIEtFWSBCTE9DSy0tLS0tClZlcnNpb246IEdudVBHIHYxLjQuNSAoR05VL0xpbnV4KQoKbVFFTkJHTUhvWGNCQ0FEdWtHT0VReWxlVmlPZ3RrTVZhN2hLaWZQNlBPQ1RoKzk4eE5XNFRmSEsvbkJKTjJzbQp1NFhhaVVtdEI5VXVHdDlqbDhWeFFnNGhPTVJmNDBjb0l3SHNOd3RTcmMyUjl2NUtncHZjdjUzN1FWSWlnVkhICldNTnZYZW9aa2tvRElVbGp2YkNFRFdhRWhTOVI1T01ZS2Q0QWFKK2YxYzhPRUxoRWNWMmRBUUxMeWp0bkVhRi8KcW1SRU4rM1k5KzVWY1JadlFIZXlCeENHK2hkVUdFNzQwaXhnblkyZ1NxWi9KNFllUW50UTZwTVVFaFQ2cGJhRQoxMHEySFVpZXJqL2ltMFYrWlVkQ2g0NkxrL1JkZmE1WktscVlPaUEyaU4xY29EUElkeXFLYXZjZGZQcVNyYUtGCkxhbjJLTGNaY2dUeFArMCtIZnpLZWZ2R0VuWmExMWNpdmJlOUFCRUJBQUcwUG1semRqcHJkV0psY201bGRHVnoKSUU5Q1V5QlFjbTlxWldOMElEeHBjM1k2YTNWaVpYSnVaWFJsYzBCaWRXbHNaQzV2Y0dWdWMzVnpaUzV2Y21jKwppUUUrQkJNQkNBQW9CUUpuRkYzNEFoc0RCUWtJSzJ5QkJnc0pDQWNEQWdZVkNBSUpDZ3NFRmdJREFRSWVBUUlYCmdBQUtDUkFqUmxUYW1pbGtOdE9BQ0FDREs5ZFE4Q0gySmk5QzNROTI2blZNVWlYZHlKSzFvbkNCclFTRUJxZFIKTEphVDZoR3g1cHp4a1FHZ1VEcFM5cDdMQTB1OTIwSEtMd0diN3lJQVd0eUU1VEFqMkNZcHJHZ3BxOThzZnNHQworVTVUOUlyQWR5YS9CYVRBa2tQNmdOaGZNak5hSzNiT1dzdnVMUmxsdUtNTmNoNGlmeStJd0xxYzFKTEc0MGJqCjJIbktCR1lrQzNtMFZ0UWZVdVBRTUltU0x0YS9Od1JISk1QbzhqZkd5TWFucU1NeHAzNS9lY1AyclhNZmIvbDEKV2pGRFk3aCs2bnFYYXkyMGxqTVhrTjIzVzh3RlRkdkM2bHE0NXd3TTVJQm5LTlIvVGpOTllBSWl6Wm9IRld6MQpjL2VjTVdXV0NCMlM3V2JZNHhJM0pTQ09ENFhJZmYzaWU3cGM2OC9rZ1B5dGlRSWNCQk1CQWdBR0JRSmpCNkYzCkFBb0pFTThMa296ZTFrODczVFFQLzB0MkYvamx0TFJRTUc3VkNMdzcrcHM1SkNXNUZJcXUvUzJpOWdTZE5BMEUKNDJ1K0x5eGpHM1l4bVZvVlJNc3hldTRrRXJ4cjhiTGNBNHA3MVcvbktlcXdGOVZMdVhLaXJzQkM3ejJzeUZpTApOZGwwQVJuQzNFTnd1TVZsU0N3Sk8wTU01TmlKdUxPcU9HWXlEMVh6U2ZuQ3prWE4wSkdBL2JmUFJTNW1QZm9XCjBPSElSWkZocUU3RUQ2d3lXcEhJS1Q4clhrRVNGd3N6VXdXL0Q3bzFIYWdYNytkdUx0OFdrcm9oR2J4VEoyMTUKWWFuT0tTcXlLZCs2WUd6RE5Vb0d1TU5QWko1d1RyVGhPa1R6RUZaNEhqbVExNnc1eG1jVUlTbkNaZDRuaHNiUwpxTi9VeVY5VnUzbG5rYXV0UzE1RTRDY2pQMVJSelNrVDBqa2E2MnZQdEF6dytQaUdyeU0xRjdzdnVSYUVuSkQ1CkdYemo5UkNVYVI2dnRGVnZxcW80ZnZiQTk5azRYWGorZEZBWFcwVFJaL2cyUU1lUFc5Y2RXaWVsY3IrdkhGNFoKMkVuc0FtZHZGN3I1ZTJKQ09VM044T1VvZGViVTZ3czRWZ1JWRzlncHRRZ2ZNUjB2Y2lCYk5ERzJYdWsxV0RrMQpxdHNjYmZtNUZWTDM2bzdka2pBMHgrVFlDdHFaSXI0eDNtbWZBWUZVcXp4cGZ5WGJTSHFVSlIyQ29XeGx5ejcyClhuSjdVRW8vMFViZ3pHenNjeExQRHlKSE1NNURuL05pOUZWVFZLbEFMSG5GT1lZU1RsdW9ZQUNGMURNdDdOSjMKb3lBME1FTEwwSlF6RWluaXhxeHBaMXRhT21WUi84cFFWcnFzdHF3cXNwM1JBQmFlWjgwSmJpZ1VDMjl6SlVWZgo9RXBsagotLS0tLUVORCBQR1AgUFVCTElDIEtFWSBCTE9DSy0tLS0tCg== \
| base64 -d \
| gpg --dearmor -o /usr/share/keyrings/kubernetes-archive-keyring.gpg

cat <<EOF >/etc/apt/sources.list.d/kubernetes-v1.29.list
deb [signed-by=/usr/share/keyrings/kubernetes-archive-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.29/deb/ /
EOF
cat <<EOF >/etc/apt/sources.list.d/kubernetes-v1.30.list
deb [signed-by=/usr/share/keyrings/kubernetes-archive-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.30/deb/ /
EOF
cat <<EOF >/etc/apt/sources.list.d/kubernetes-v1.31.list
deb [signed-by=/usr/share/keyrings/kubernetes-archive-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.31/deb/ /
EOF
cat <<EOF >/etc/apt/sources.list.d/kubernetes-v1.32.list
deb [signed-by=/usr/share/keyrings/kubernetes-archive-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.32/deb/ /
EOF
cat <<EOF >/etc/apt/sources.list.d/docker.list
deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable
EOF

apt update
apt-mark unhold docker-ce docker-ce-cli containerd.io
apt install -y docker-ce=$DOCKER_VERSION docker-ce-cli=$DOCKER_VERSION containerd.io=$CONTAINERD_VERSION
apt-mark hold docker-ce docker-ce-cli containerd.io

mkdir -p /etc/docker/
cat <<EOF | tee /etc/docker/daemon.json
{
  "exec-opts": ["native.cgroupdriver=systemd"],
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "100m",
    "max-file": "10"
  },
  "live-restore": true,
  "max-concurrent-downloads": 10,
  "max-concurrent-uploads": 10,
  "default-ulimits": {
    "memlock": {
      "Hard": -1,
      "Name": "memlock",
      "Soft": -1
    }
  },
  "storage-driver": "overlay2",
  "insecure-registries": ["10.100.0.0/16"]
}
EOF

cat <<EOF | tee /etc/containerd/config.toml
version = 2
root = "/var/lib/containerd"
state = "/run/containerd"

[grpc]
address = "/run/containerd/containerd.sock"

[plugins."io.containerd.grpc.v1.cri".containerd]
default_runtime_name = "runc"

[plugins."io.containerd.grpc.v1.cri"]
sandbox_image = "$PAUSE_CONTAINER"

[plugins."io.containerd.grpc.v1.cri".registry]
config_path = "/etc/containerd/certs.d:/etc/docker/certs.d"

[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
runtime_type = "io.containerd.runc.v2"

[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
SystemdCgroup = true

[plugins."io.containerd.grpc.v1.cri".cni]
bin_dir = "/opt/cni/bin"
conf_dir = "/etc/cni/net.d"
EOF

cat <<EOF | tee /etc/crictl.yaml
runtime-endpoint: unix:///var/run/containerd/containerd.sock
image-endpoint: unix:///var/run/containerd/containerd.sock
timeout: 10
debug: false
EOF

cat <<EOF | tee /etc/modules-load.d/k8s.conf
overlay
br_netfilter
EOF

modprobe br_netfilter overlay

cat <<EOF | tee /etc/sysctl.conf
net.bridge.bridge-nf-call-ip6tables=1
net.bridge.bridge-nf-call-iptables=1
net.ipv4.ip_forward=1
fs.inotify.max_user_watches=524288
fs.inotify.max_user_instances=8192
vm.max_map_count=524288
vm.overcommit_memory=1
kernel.panic=10
kernel.panic_on_oops=1
EOF
sysctl -p

apt-mark unhold kubelet kubeadm kubectl
apt-get install -y kubelet=${KUBERNETES_VERSION} kubeadm=${KUBERNETES_VERSION} kubectl=${KUBERNETES_VERSION}
apt-mark hold kubelet kubeadm kubectl

# stop all services
systemctl stop kubelet containerd docker docker.socket

INTERNAL_IP=$(hostname -I | awk '{print $2}')

mkdir -p /etc/kubernetes/kubelet/

# https://docs.hetzner.com/dns-console/dns/general/recursive-name-servers
cat > /etc/kubernetes/kubelet/resolv.conf <<EOF
nameserver 185.12.64.1
nameserver 185.12.64.2
EOF

# use custom provider id
: ${PROVIDER_ID="hcloud://$(curl http://169.254.169.254/hetzner/v1/metadata/instance-id)"}

# kubeadm config print init-defaults --component-configs KubeletConfiguration
cat <<EOF | tee /etc/kubernetes/kubelet/config.yaml
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
authentication:
  anonymous:
    enabled: false
  webhook:
    cacheTTL: 2m0s
    enabled: true
  x509:
    clientCAFile: /etc/kubernetes/pki/ca.crt
authorization:
  mode: Webhook
  webhook:
    cacheAuthorizedTTL: 5m0s
    cacheUnauthorizedTTL: 30s
cgroupDriver: systemd
clusterDNS:
- 10.96.0.10
clusterDomain: cluster.local
resolvConf: /etc/kubernetes/kubelet/resolv.conf
rotateCertificates: true
staticPodPath: /etc/kubernetes/manifests
featureGates:
  RotateKubeletServerCertificate: true
evictionHard:
  memory.available: "100Mi"
  nodefs.available: "10%"
  nodefs.inodesFree: "5%"
kubeReserved:
  cpu: "10m"
  memory: "100Mi"
  ephemeral-storage: "1Gi"
protectKernelDefaults: true
serializeImagePulls: false
serverTLSBootstrap: true
providerID: "$PROVIDER_ID"
runtimeRequestTimeout: "15m"
EOF

cat <<EOF | tee /etc/default/kubelet
KUBELET_CONFIG_ARGS=--config=/etc/kubernetes/kubelet/config.yaml
KUBELET_EXTRA_ARGS=--cloud-provider=external --node-ip=$INTERNAL_IP --v=2
EOF

# some UBUNTU installations use /etc/sysconfig/kubelet
# latest ubuntu versions use /etc/default/kubelet
cp /etc/default/kubelet /etc/sysconfig/kubelet || true

apt -y autoremove
apt -y autoclean

# prestart script
{{ .Values.preStartScript }}

# start all node services
systemctl daemon-reload
systemctl enable kubelet containerd docker docker.socket
systemctl start kubelet containerd docker docker.socket

# pull sandbox image
ctr --namespace k8s.io image pull $PAUSE_CONTAINER

# poststart script
{{ .Values.postStartScript }}
