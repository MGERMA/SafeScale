/*
 * Copyright 2018, CS Systemes d'Information, http://www.c-s.fr
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package ErrorCode

//go:generate stringer -type=Enum

type Enum int

const (
	_ Enum = iota

	DcosInstallDownload
	DcosInstallExecution
	DcosConfigGeneratorDownload
	DcosGenerateConfig
	DcosCliDownload
	DockerNginxDownload
	DockerNginxStart
	DockerProxyBuild
	DockerProxyStart
	DockerGuacamoleBuild
	DockerPyInstall
	DockerInstall
	DockerComposeDownload
	DockerComposeConfig
	DockerComposeExecution
	DesktopInstall
	DesktopStart
	DesktopTimeout
	GuacamoleImageDownload
	KubectlDownload
	SystemUpdate
	ToolsInstall
	PipInstall

	//NextErrorCode is the next error code useable
	NextErrorCode
)
