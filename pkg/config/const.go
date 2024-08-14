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
package config

import "time"

const (
	masterServersCount          = 3
	workersCount                = 20
	loadBalancerDefaultPort     = 6443
	waitTimeInRetry             = 3 * time.Second
	retryTimeLimit              = 20
	secretString                = "<secret>"
	defaultLocation             = hcloudLocationEUFalkenstein
	defaultDatacenter           = hcloudLocationEUFalkenstein + "-dc14"
	defaultAutoscalerInstances  = "cpx11,cpx21,cx22,cpx31,cx32,cpx41,cx42,cx52,cpx51"
	hcloudLocationEUFalkenstein = "fsn1"
	hcloudLocationEUNuremberg   = "nbg1"
	hcloudLocationEUHelsinki    = "hel1"
)
