//+build libvirt

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

package local

import (
	"encoding/xml"
	"fmt"
	"math"
	"net"
	"regexp"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/CS-SI/SafeScale/iaas/resources"
	"github.com/CS-SI/SafeScale/iaas/resources/enums/IPVersion"
	libvirt "github.com/libvirt/libvirt-go"
	libvirtxml "github.com/libvirt/libvirt-go-xml"
)

func infoFromCidr(cidr string) (string, string, string, string, error) {
	IP, IPNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", "", "", "", fmt.Errorf("Failed to parse cidr : %s", err.Error())
	} else if IPNet.Mask[3] >= 63 {
		return "", "", "", "", fmt.Errorf("Please use a wider network range")
	}

	mask := fmt.Sprintf("%d.%d.%d.%d", IPNet.Mask[0], IPNet.Mask[1], IPNet.Mask[2], IPNet.Mask[3])
	dhcpStart := fmt.Sprintf("%d.%d.%d.%d", IP[12], IP[13], IPNet.IP[2], IPNet.IP[3]+2)
	dhcpEnd := fmt.Sprintf("%d.%d.%d.%d", IP[12], IP[13], IPNet.IP[2]+(255-IPNet.Mask[2]), IPNet.IP[3]+(255-IPNet.Mask[3]-1))

	return IPNet.IP.String(), mask, dhcpStart, dhcpEnd, nil
}

func getNetworkFromRef(ref string, libvirtService *libvirt.Connect) (*libvirt.Network, error) {
	libvirtNetwork, err := libvirtService.LookupNetworkByUUIDString(ref)
	if err != nil {
		libvirtNetwork, err = libvirtService.LookupNetworkByName(ref)
		if err != nil {
			re := regexp.MustCompile("[0-9]+")
			errCode, _ := strconv.Atoi(re.FindString(err.Error()))
			if errCode == 43 {
				return nil, resources.ResourceNotFoundError("network", ref)
			}
			return nil, fmt.Errorf(fmt.Sprintf("Failed to fetch network from ref : %s", err.Error()))
		}
	}

	return libvirtNetwork, nil
}

func getNetworkFromLibvirtNetwork(libvirtNetwork *libvirt.Network) (*resources.Network, error) {
	libvirtNetworkXML, err := libvirtNetwork.GetXMLDesc(0)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Failed get network's xml description  : %s", err.Error()))
	}
	networkDescription := &libvirtxml.Network{}
	err = xml.Unmarshal([]byte(libvirtNetworkXML), networkDescription)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Failed get Unmarshal networks's xml description  : %s", err.Error()))
	}

	var ipVersion IPVersion.Enum
	if networkDescription.IPv6 == "" {
		ipVersion = IPVersion.IPv4
	} else {
		ipVersion = IPVersion.IPv6
	}

	cidr := ""
	if ipVersion == IPVersion.IPv4 {
		netmaskBloc := strings.Split(networkDescription.IPs[0].Netmask, ".")
		ipBlocstring := strings.Split(networkDescription.IPs[0].Address, ".")
		var ipBloc [4]int
		netmaskInt := 0
		for i := 0; i < 4; i++ {
			value, err := strconv.Atoi(netmaskBloc[i])
			ipBloc[i], err = strconv.Atoi(ipBlocstring[i])
			if err != nil {
				return nil, fmt.Errorf("Failed to convert x.x.x.x nemask to [0-32] netmask")
			}
			nbBits := 0
			if value != 0 {
				nbBits = int(math.Log2(float64(value)) + 1)
				netmaskInt += nbBits
			}
			ipBloc[i] -= ipBloc[i] % int(math.Pow(2, float64(8-nbBits)))
		}
		cidr = fmt.Sprintf("%d.%d.%d.%d/%d", ipBloc[0], ipBloc[1], ipBloc[2], ipBloc[3], netmaskInt)
	} else {
		cidr = networkDescription.IPv6
	}

	network := resources.NewNetwork()
	network.ID = networkDescription.UUID
	network.Name = networkDescription.Name
	network.CIDR = cidr
	network.IPVersion = ipVersion
	//network.GatewayID
	//network.Properties

	return network, nil
}

// CreateNetwork creates a network named name
func (s *Stack) CreateNetwork(req resources.NetworkRequest) (*resources.Network, error) {
	log.Debug("local.Client.CreateNetwork() called")
	defer log.Debug("local.Client.CreateNetwork() done")

	name := req.Name
	ipVersion := req.IPVersion
	cidr := req.CIDR
	dns := req.DNSServers

	if ipVersion != IPVersion.IPv4 {
		// TODO implement IPV6 networks
		panic("only ipv4 networks are implemented")
	}
	if len(dns) != 0 {
		// TODO implement DNS for networks
		panic("DNS not implemented yet in networks creation")
	}

	libvirtNetwork, err := getNetworkFromRef(name, s.LibvirtService)
	if err != nil {
		if _, ok := err.(resources.ErrResourceNotFound); !ok {
			return nil, err
		}
	}

	if libvirtNetwork != nil {
		return nil, fmt.Errorf("Network %s already exists", name)
	}

	ip, netmask, dhcpStart, dhcpEnd, err := infoFromCidr(cidr)
	if err != nil {
		return nil, err
	}

	requestXML := `
	 <network>
		 <name>` + name + `</name>
		 <ip address="` + ip + `" netmask="` + netmask + `">
			 <dhcp> 
				 <range start="` + dhcpStart + `" end="` + dhcpEnd + `" />
			 </dhcp>
		 </ip>
	 </network>`

	libvirtNetwork, err = s.LibvirtService.NetworkDefineXML(requestXML)
	if err != nil {
		return nil, fmt.Errorf("Failed to create network : %s", err.Error())
	}
	defer func(*libvirt.Network) {
		if err != nil {
			if err := libvirtNetwork.Undefine(); err != nil {
				fmt.Printf("Failed undefining network %s : %s\n", name, err.Error())
				if err := libvirtNetwork.Destroy(); err != nil {
					fmt.Printf("Failed to destroy network %s : %s\n", name, err.Error())
				}
			}
		}
	}(libvirtNetwork)

	err = libvirtNetwork.SetAutostart(true)
	if err != nil {
		return nil, fmt.Errorf("Failed to enable network autostart : %s", err.Error())
	}
	err = libvirtNetwork.Create()
	if err != nil {
		return nil, fmt.Errorf("Failed to start network : %s", err.Error())
	}

	network, err := getNetworkFromLibvirtNetwork(libvirtNetwork)
	if err != nil {
		return nil, fmt.Errorf("Failed to convert a libvirt network into a network : %s", err.Error())
	}

	return network, nil
}

// GetNetwork returns the network identified by ref (id or name)
func (s *Stack) GetNetwork(ref string) (*resources.Network, error) {
	log.Debug("local.Client.GetNetwork() called")
	defer log.Debug("local.Client.GetNetwork() done")

	libvirtNetwork, err := getNetworkFromRef(ref, s.LibvirtService)
	if err != nil {
		return nil, err
	}
	active, err := libvirtNetwork.IsActive()
	if err != nil {
		return nil, fmt.Errorf("Failed to check if the network is active : %s", err.Error())
	}
	if !active {
		err = libvirtNetwork.Create()
		if err != nil {
			return nil, fmt.Errorf("Failed to start network : %s", err.Error())
		}
	}

	network, err := getNetworkFromLibvirtNetwork(libvirtNetwork)
	if err != nil {
		return nil, fmt.Errorf("Failed to convert a libvirt network into a network : %s", err.Error())
	}

	return network, nil
}

// GetNetworkByName returns the network identified by ref (id or name)
func (s *Stack) GetNetworkByName(ref string) (*resources.Network, error) {
	log.Debug("local.Client.GetNetworkByName() called")
	defer log.Debug("local.Client.GetNetworkByName() done")
	return s.GetNetwork(ref)
}

// ListNetworks lists available networks
func (s *Stack) ListNetworks() ([]*resources.Network, error) {
	log.Debug("local.Client.ListNetworks() called")
	defer log.Debug("local.Client.ListNetworks() done")

	var networks []*resources.Network

	libvirtNetworks, err := s.LibvirtService.ListAllNetworks(3)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Error listing networks : %s", err.Error()))
	}
	for _, libvirtNetwork := range libvirtNetworks {
		network, err := getNetworkFromLibvirtNetwork(&libvirtNetwork)
		if err != nil {
			return nil, fmt.Errorf(fmt.Sprintf("Failed to get network from libvirtNetwork : %s", err.Error()))
		}

		networks = append(networks, network)
	}

	return networks, nil
}

// DeleteNetwork deletes the network identified by id
func (s *Stack) DeleteNetwork(ref string) error {
	log.Debug("local.Client.DeleteNetwork() called")
	defer log.Debug("local.Client.DeleteNetwork() done")

	libvirtNetwork, err := getNetworkFromRef(ref, s.LibvirtService)
	if err != nil {
		return err
	}

	isActive, err := libvirtNetwork.IsActive()
	if err != nil {
		return fmt.Errorf("Failed to check if the network is active : %s", err.Error())
	}
	if isActive {
		err = libvirtNetwork.Destroy()
		if err != nil {
			return fmt.Errorf("Failed to destroy network : %s", err.Error())
		}
	}

	err = libvirtNetwork.Undefine()
	if err != nil {
		return fmt.Errorf("Failed to undefine network : %s", err.Error())
	}

	return nil
}

// CreateGateway creates a public Gateway for a private network
func (s *Stack) CreateGateway(req resources.GatewayRequest) (*resources.Host, error) {
	log.Debug("local.Client.CreateGateway() called")
	defer log.Debug("local.Client.CreateGateway() done")

	network := req.Network
	templateID := req.TemplateID
	imageID := req.ImageID
	keyPair := req.KeyPair
	gwName := req.Name

	networkLibvirt, err := getNetworkFromRef(network.ID, s.LibvirtService)
	if err != nil {
		return nil, err
	}
	if gwName == "" {
		name, err := networkLibvirt.GetName()
		if err != nil {
			return nil, fmt.Errorf("Failed to get network name : %s", err.Error())
		}
		gwName = "gw-" + name
	}

	hostReq := resources.HostRequest{
		ImageID:      imageID,
		KeyPair:      keyPair,
		ResourceName: gwName,
		TemplateID:   templateID,
		Networks:     []*resources.Network{network},
		PublicIP:     true,
	}

	host, err := s.CreateHost(hostReq)
	if err != nil {
		return nil, fmt.Errorf("Failed to create gateway host : %s", err.Error())
	}

	return host, nil
}

// DeleteGateway delete the public gateway referenced by ref (id or name)
func (s *Stack) DeleteGateway(ref string) error {
	log.Debug("local.Client.DeleteGateway() called")
	defer log.Debug("local.Client.DeleteGateway() done")

	return s.DeleteHost(ref)
}