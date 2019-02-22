/*
 * Copyright 2018-2019, CS Systemes d'Information, http://www.c-s.fr
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or provideried.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cloudferro

import (
	"github.com/CS-SI/SafeScale/iaas"
	"github.com/CS-SI/SafeScale/iaas/objectstorage"
	"github.com/CS-SI/SafeScale/iaas/providers"
	"github.com/CS-SI/SafeScale/iaas/resources"
	"github.com/CS-SI/SafeScale/iaas/resources/enums/VolumeSpeed"
	"github.com/CS-SI/SafeScale/iaas/stacks"
	"github.com/CS-SI/SafeScale/iaas/stacks/openstack"
)

var (
	cloudferroIdentityEndpoint = "https://cf2.cloudferro.com:5000/v3"
	cloudferroDefaultImage     = "Ubuntu 18.04"
	cloudferroDNSServers       = []string{"185.48.234.234", "185.48.234.238"}
)

// provider is the providerementation of the CloudFerro provider
type provider struct {
	stacks.Stack
}

// New creates a new instance of cloudferro provider
func New() providers.Provider {
	return &provider{}
}

// Build build a new Client from configuration parameter
func (p *provider) Build(params map[string]interface{}) (providers.Provider, error) {
	// tenantName, _ := params["name"].(string)

	identity, _ := params["identity"].(map[string]interface{})
	compute, _ := params["compute"].(map[string]interface{})
	// network, _ := params["network"].(map[string]interface{})

	username, _ := identity["Username"].(string)
	password, _ := identity["Password"].(string)
	domainName, _ := identity["DomainName"].(string)

	region, _ := compute["Region"].(string)
	projectName, _ := compute["ProjectName"].(string)
	// projectID, _ := compute["ProjectID"].(string)
	defaultImage, _ := compute["DefaultImage"].(string)
	if defaultImage == "" {
		defaultImage = cloudferroDefaultImage
	}

	authOptions := stacks.AuthenticationOptions{
		IdentityEndpoint: cloudferroIdentityEndpoint,
		Username:         username,
		Password:         password,
		DomainName:       domainName,
		TenantName:       projectName,
		Region:           region,
		FloatingIPPool:   "external",
		AllowReauth:      true,
	}

	metadataBucketName, err := objectstorage.BuildMetadataBucketName("huaweicloud", region, domainName, projectName)
	if err != nil {
		return nil, err
	}

	cfgOptions := stacks.ConfigurationOptions{
		ProviderNetwork:           "external",
		UseFloatingIP:             true,
		UseLayer3Networking:       true,
		AutoHostNetworkInterfaces: true,
		VolumeSpeeds: map[string]VolumeSpeed.Enum{
			"HDD": VolumeSpeed.HDD,
			"SSD": VolumeSpeed.SSD,
		},
		MetadataBucket: metadataBucketName,
		DNSList:        cloudferroDNSServers,
		DefaultImage:   defaultImage,
	}

	stack, err := openstack.New(authOptions, nil, cfgOptions, nil)
	if err != nil {
		return nil, err
	}
	err = stack.InitDefaultSecurityGroup()
	if err != nil {
		return nil, err
	}
	return &provider{Stack: stack}, nil
}

// GetAuthOpts returns the auth options
func (p *provider) GetAuthOpts() (providers.Config, error) {
	cfg := providers.ConfigMap{}

	opts := p.Stack.GetAuthenticationOptions()
	cfg.Set("TenantName", opts.TenantName)
	cfg.Set("Login", opts.Username)
	cfg.Set("Password", opts.Password)
	cfg.Set("AuthUrl", opts.IdentityEndpoint)
	cfg.Set("Region", opts.Region)
	return cfg, nil
}

// GetCfgOpts return configuration parameters
func (p *provider) GetCfgOpts() (providers.Config, error) {
	cfg := providers.ConfigMap{}

	opts := p.Stack.GetConfigurationOptions()
	cfg.Set("DNSList", opts.DNSList)
	cfg.Set("AutoHostNetworkInterfaces", opts.AutoHostNetworkInterfaces)
	cfg.Set("UseLayer3Networking", opts.UseLayer3Networking)
	cfg.Set("DefaultImage", opts.DefaultImage)
	cfg.Set("MetadataBucketName", opts.MetadataBucket)
	return cfg, nil
}

// ListTemplates ...
// Value of all has no impact on the result
func (p *provider) ListTemplates(all bool) ([]resources.HostTemplate, error) {
	allTemplates, err := p.Stack.ListTemplates()
	if err != nil {
		return nil, err
	}
	return allTemplates, nil
}

// ListImages ...
// Value of all has no impact on the result
func (p *provider) ListImages(all bool) ([]resources.Image, error) {
	allImages, err := p.Stack.ListImages()
	if err != nil {
		return nil, err
	}
	return allImages, nil
}

func init() {
	iaas.Register("cloudferro", &provider{})
}