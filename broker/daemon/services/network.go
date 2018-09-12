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

package services

import (
	"fmt"

	"github.com/CS-SI/SafeScale/providers"
	"github.com/CS-SI/SafeScale/providers/api"
	"github.com/CS-SI/SafeScale/providers/api/enums/IPVersion"
)

//NetworkAPI defines API to manage networks
type NetworkAPI interface {
	Create(net string, cidr string, ipVersion IPVersion.Enum, cpu int, ram float32, disk int, os string, gwname string) (*api.Network, error)
	List(all bool) ([]api.Network, error)
	Get(ref string) (*api.Network, error)
	Delete(ref string) error
}

// NetworkService an instance of NetworkAPI
type NetworkService struct {
	provider  *providers.Service
	ipVersion IPVersion.Enum
}

// NewNetworkService Creates new Network service
func NewNetworkService(api api.ClientAPI) NetworkAPI {
	return &NetworkService{
		provider: providers.FromClient(api),
	}
}

// Create creates a network
func (svc *NetworkService) Create(net string, cidr string, ipVersion IPVersion.Enum, cpu int, ram float32, disk int, os string, gwname string) (apinetwork *api.Network, err error) {
	// Create the network
	network, err := svc.provider.CreateNetwork(api.NetworkRequest{
		Name:      net,
		IPVersion: ipVersion,
		CIDR:      cidr,
	})
	if err != nil {
		return nil, err
	}

	defer func() {
		if r := recover(); r != nil {
			svc.provider.DeleteNetwork(network.ID)
			switch t := r.(type) {
			case string:
				err = fmt.Errorf("%q", t)
			case error:
				err = t
			}
		}
	}()

	// Create a gateway
	tpls, err := svc.provider.SelectTemplatesBySize(api.SizingRequirements{
		MinCores:    cpu,
		MinRAMSize:  ram,
		MinDiskSize: disk,
	})
	if err != nil {
		panic(err)
	}
	if len(tpls) < 1 {
		panic(fmt.Sprintf("No template found for %v cpu, %v ram, %v disk", cpu, ram, disk))
	}
	img, err := svc.provider.SearchImage(os)
	if err != nil {
		panic(err)
	}

	keypairName := "kp_" + network.Name
	// Makes sure keypair doesn't exist
	svc.provider.DeleteKeyPair(keypairName)
	keypair, err := svc.provider.CreateKeyPair(keypairName)
	if err != nil {
		panic(err)
	}

	gwRequest := api.GWRequest{
		ImageID:    img.ID,
		NetworkID:  network.ID,
		KeyPair:    keypair,
		TemplateID: tpls[0].ID,
		GWName:     gwname,
	}

	err = svc.provider.CreateGateway(gwRequest)
	if err != nil {
		panic(err)
	}

	rv, err := svc.Get(net)
	return rv, err
}

//List returns the network list
func (svc *NetworkService) List(all bool) ([]api.Network, error) {
	return svc.provider.ListNetworks(all)
}

//Get returns the network identified by ref, ref can be the name or the id
func (svc *NetworkService) Get(ref string) (*api.Network, error) {
	return svc.provider.GetNetwork(ref)
}

//Delete deletes network referenced by ref
func (svc *NetworkService) Delete(ref string) error {
	return svc.provider.DeleteNetwork(ref)
}
