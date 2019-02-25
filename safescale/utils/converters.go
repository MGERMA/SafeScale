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
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package utils

import (
	pb "github.com/CS-SI/SafeScale/safescale"
	"github.com/CS-SI/SafeScale/iaas/resources"
	"github.com/CS-SI/SafeScale/iaas/resources/enums/HostProperty"
	propsv1 "github.com/CS-SI/SafeScale/iaas/resources/properties/v1"
	"github.com/CS-SI/SafeScale/system"
)

// ToPBSshConfig converts a system.SSHConfig into a SshConfig
func ToPBSshConfig(from *system.SSHConfig) *pb.SshConfig {
	var gw *pb.SshConfig
	if from.GatewayConfig != nil {
		gw = ToPBSshConfig(from.GatewayConfig)
	}
	return &pb.SshConfig{
		Gateway:    gw,
		Host:       from.Host,
		Port:       int32(from.Port),
		PrivateKey: from.PrivateKey,
		User:       from.User,
	}
}

// ToSystemSSHConfig converts a pb.SshConfig into a system.SSHConfig
func ToSystemSSHConfig(from *pb.SshConfig) *system.SSHConfig {
	var gw *system.SSHConfig
	if from.Gateway != nil {
		gw = ToSystemSSHConfig(from.Gateway)
	}
	return &system.SSHConfig{
		User:          from.User,
		Host:          from.Host,
		PrivateKey:    from.PrivateKey,
		Port:          int(from.Port),
		GatewayConfig: gw,
	}
}

// ToPBVolume converts an api.Volume to a *Volume
func ToPBVolume(in *resources.Volume) *pb.Volume {
	return &pb.Volume{
		ID:    in.ID,
		Name:  in.Name,
		Size:  int32(in.Size),
		Speed: pb.VolumeSpeed(in.Speed),
	}
}

// ToPBVolumeAttachment converts an api.Volume to a *Volume
func ToPBVolumeAttachment(in *resources.VolumeAttachment) *pb.VolumeAttachment {
	return &pb.VolumeAttachment{
		Volume:    &pb.Reference{ID: in.VolumeID},
		Host:      &pb.Reference{ID: in.ServerID},
		MountPath: in.MountPoint,
		Device:    in.Device,
	}
}

// ToPBVolumeInfo converts an api.Volume to a *VolumeInfo
func ToPBVolumeInfo(volume *resources.Volume, mounts map[string]*propsv1.HostLocalMount) *pb.VolumeInfo {
	pbvi := &pb.VolumeInfo{
		ID:    volume.ID,
		Name:  volume.Name,
		Size:  int32(volume.Size),
		Speed: pb.VolumeSpeed(volume.Speed),
	}
	if len(mounts) > 0 {
		for k, mount := range mounts {
			pbvi.Host = &pb.Reference{Name: k}
			pbvi.MountPath = mount.Path
			pbvi.Device = mount.Device
			pbvi.Format = mount.FileSystem

			break
		}
	}
	return pbvi
}

// ToPBBucketList convert a list of string into a *ContainerLsit
func ToPBBucketList(in []string) *pb.BucketList {
	var buckets []*pb.Bucket
	for _, name := range in {
		buckets = append(buckets, &pb.Bucket{Name: name})
	}
	return &pb.BucketList{
		Buckets: buckets,
	}
}

// ToPBBucketMountPoint convert a Bucket into a BucketMountingPoint
func ToPBBucketMountPoint(in *resources.Bucket) *pb.BucketMountingPoint {
	return &pb.BucketMountingPoint{
		Bucket: in.Name,
		Path:   in.MountPoint,
		Host:   &pb.Reference{Name: in.Host},
	}
}

// ToPBShare convert a share from model to protocolbuffer format
func ToPBShare(hostName string, share *propsv1.HostShare) *pb.ShareDefinition {
	return &pb.ShareDefinition{
		ID:   share.ID,
		Name: share.Name,
		Host: &pb.Reference{Name: hostName},
		Path: share.Path,
		Type: "nfs",
	}
}

// ToPBShareMount convert share mount on host to protocolbuffer format
func ToPBShareMount(shareName string, hostName string, mount *propsv1.HostRemoteMount) *pb.ShareMountDefinition {
	return &pb.ShareMountDefinition{
		Share: &pb.Reference{Name: shareName},
		Host:  &pb.Reference{Name: hostName},
		Path:  mount.Path,
		Type:  mount.FileSystem,
	}
}

// ToPBShareMountList converts share mounts to protocol buffer
func ToPBShareMountList(hostName string, share *propsv1.HostShare, mounts map[string]*propsv1.HostRemoteMount) *pb.ShareMountList {
	var pbMounts []*pb.ShareMountDefinition
	for k, v := range mounts {
		pbMounts = append(pbMounts, &pb.ShareMountDefinition{
			Host:  &pb.Reference{Name: k},
			Share: &pb.Reference{Name: share.Name},
			Path:  v.Path,
			Type:  "nfs",
		})
	}
	return &pb.ShareMountList{
		Share:     ToPBShare(hostName, share),
		MountList: pbMounts,
	}
}

// ToPBHost convert an host from api to protocolbuffer format
func ToPBHost(in *resources.Host) *pb.Host {
	var (
		hostNetworkV1 *propsv1.HostNetwork
		hostSizingV1  *propsv1.HostSizing
		hostVolumesV1 *propsv1.HostVolumes
		volumes       []string
	)

	err := in.Properties.LockForRead(HostProperty.NetworkV1).ThenUse(func(v interface{}) error {
		hostNetworkV1 = v.(*propsv1.HostNetwork)
		return in.Properties.LockForRead(HostProperty.SizingV1).ThenUse(func(v interface{}) error {
			hostSizingV1 = v.(*propsv1.HostSizing)
			return in.Properties.LockForRead(HostProperty.VolumesV1).ThenUse(func(v interface{}) error {
				hostVolumesV1 = v.(*propsv1.HostVolumes)
				for k := range hostVolumesV1.VolumesByName {
					volumes = append(volumes, k)
				}
				return nil
			})
		})
	})
	if err != nil {
		return nil
	}
	return &pb.Host{
		CPU:                 int32(hostSizingV1.AllocatedSize.Cores),
		Disk:                int32(hostSizingV1.AllocatedSize.DiskSize),
		GatewayID:           hostNetworkV1.DefaultGatewayID,
		ID:                  in.ID,
		PublicIP:            in.GetPublicIP(),
		PrivateIP:           in.GetPrivateIP(),
		Name:                in.Name,
		PrivateKey:          in.PrivateKey,
		RAM:                 hostSizingV1.AllocatedSize.RAMSize,
		State:               pb.HostState(in.LastState),
		AttachedVolumeNames: volumes,
	}
}

// ToPBHostDefinition ...
func ToPBHostDefinition(in *resources.HostDefinition) *pb.HostDefinition {
	return &pb.HostDefinition{
		CPUNumber: int32(in.Cores),
		RAM:       in.RAMSize,
		Disk:      int32(in.DiskSize),
		GPUNumber: int32(in.GPUNumber),
		Freq:      in.CPUFreq,
		ImageID:   in.ImageID,
	}
}

// ToPBGatewayDefinition ...
func ToPBGatewayDefinition(in *resources.HostDefinition) *pb.GatewayDefinition {
	return &pb.GatewayDefinition{
		CPU:     int32(in.Cores),
		RAM:     in.RAMSize,
		Disk:    int32(in.DiskSize),
		ImageID: in.ImageID,
	}
}

// ToHostStatus ...
func ToHostStatus(in *resources.Host) *pb.HostStatus {
	return &pb.HostStatus{
		Name:   in.Name,
		Status: pb.HostState(in.LastState).String(),
	}
}

// ToPBHostTemplate convert an template from api to protocolbuffer format
func ToPBHostTemplate(in *resources.HostTemplate) *pb.HostTemplate {
	return &pb.HostTemplate{
		ID:      in.ID,
		Name:    in.Name,
		Cores:   int32(in.Cores),
		Ram:     int32(in.RAMSize),
		Disk:    int32(in.DiskSize),
		GPUs:    int32(in.GPUNumber),
		GPUType: in.GPUType,
	}
}

// ToPBImage convert an image from api to protocolbuffer format
func ToPBImage(in *resources.Image) *pb.Image {
	return &pb.Image{
		ID:   in.ID,
		Name: in.Name,
	}
}

//ToPBNetwork convert a network from api to protocolbuffer format
func ToPBNetwork(in *resources.Network) *pb.Network {
	return &pb.Network{
		ID:        in.ID,
		Name:      in.Name,
		CIDR:      in.CIDR,
		GatewayID: in.GatewayID,
	}
}