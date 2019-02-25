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

package handlers

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	uuid "github.com/satori/go.uuid"

	"github.com/CS-SI/SafeScale/safescale/server/metadata"
	"github.com/CS-SI/SafeScale/iaas"
	"github.com/CS-SI/SafeScale/iaas/resources"
	"github.com/CS-SI/SafeScale/iaas/resources/enums/HostProperty"
	propsv1 "github.com/CS-SI/SafeScale/iaas/resources/properties/v1"
	"github.com/CS-SI/SafeScale/system/nfs"
	"github.com/CS-SI/SafeScale/utils"
)

//go:generate mockgen -destination=../mocks/mock_nasapi.go -package=mocks github.com/CS-SI/SafeScale/safescale/server/handlers ShareAPI

// TODO At service level, ve need to log before returning, because it's the last chance to track the real issue in server side

// ShareAPI defines API to manipulate Shares
type ShareAPI interface {
	Create(context.Context, string, string, string, []string, bool, bool, bool, bool, bool, bool, bool) (*propsv1.HostShare, error)
	ForceInspect(context.Context, string) (*resources.Host, *propsv1.HostShare, map[string]*propsv1.HostRemoteMount, error)
	Inspect(context.Context, string) (*resources.Host, *propsv1.HostShare, map[string]*propsv1.HostRemoteMount, error)
	Delete(context.Context, string) error
	List(context.Context) (map[string]map[string]*propsv1.HostShare, error)
	Mount(context.Context, string, string, string, bool) (*propsv1.HostRemoteMount, error)
	Unmount(context.Context, string, string) error
}

// ShareHandler nas service
type ShareHandler struct {
	service *iaas.Service
}

// NewShareHandler creates a ShareHandler
func NewShareHandler(svc *iaas.Service) ShareAPI {
	return &ShareHandler{
		service: svc,
	}
}

func sanitize(in string) (string, error) {
	sanitized := path.Clean(in)
	if !path.IsAbs(sanitized) {
		return "", logicErr(fmt.Errorf("Exposed path must be absolute"))
	}
	return sanitized, nil
}

// Create a share on host
func (handler *ShareHandler) Create(
	ctx context.Context,
	shareName, hostName, path string, securityModes []string,
	readOnly, rootSquash, secure, async, noHide, crossMount, subtreeCheck bool,
) (*propsv1.HostShare, error) {

	// Check if a share already exists with the same name
	server, _, _, err := handler.Inspect(ctx, shareName)
	if err != nil {
		if _, ok := err.(resources.ErrResourceNotFound); !ok {
			return nil, infraErr(err)
		}
	}
	if server != nil {
		return nil, logicErr(resources.ResourceDuplicateError("share", shareName))
	}

	// Sanitize path
	sharePath, err := sanitize(path)
	if err != nil {
		return nil, infraErr(err)
	}

	hostHandler := NewHostHandler(handler.service)
	server, err = hostHandler.Inspect(ctx, hostName)
	if err != nil {
		return nil, throwErr(err)
	}

	// Check if the path to share isn't a remote mount or contains a remote mount
	err = server.Properties.LockForRead(HostProperty.MountsV1).ThenUse(func(v interface{}) error {
		serverMountsV1 := v.(*propsv1.HostMounts)
		if _, found := serverMountsV1.RemoteMountsByPath[path]; found {
			return logicErr(fmt.Errorf("path to export '%s' is a mounted share", sharePath))
		}
		for k := range serverMountsV1.RemoteMountsByPath {
			if strings.Index(sharePath, k) == 0 {
				return logicErr(fmt.Errorf("export path '%s' contains a share mounted in '%s'", sharePath, k))
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Installs NFS Server software if needed
	sshHandler := NewSSHHandler(handler.service)
	sshConfig, err := sshHandler.GetConfig(ctx, server)
	if err != nil {
		return nil, infraErr(err)
	}
	nfsServer, err := nfs.NewServer(sshConfig)
	if err != nil {
		return nil, infraErr(err)
	}

	err = server.Properties.LockForRead(HostProperty.SharesV1).ThenUse(func(v interface{}) error {
		serverSharesV1 := v.(*propsv1.HostShares)
		if len(serverSharesV1.ByID) == 0 {
			// Host doesn't have shares yet, so install NFS
			err = nfsServer.Install()
			if err != nil {
				return infraErr(err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = nfsServer.AddShare(sharePath, securityModes, readOnly, rootSquash, secure, async, noHide, crossMount, subtreeCheck)
	if err != nil {
		return nil, infraErr(err)
	}
	defer func() {
		if err != nil {
			err2 := nfsServer.RemoveShare(sharePath)
			if err2 != nil {
				log.Warn("Failed to RemoveShare")
			}
		}
	}()

	var share *propsv1.HostShare

	// Updates Host Property propsv1.HostShares
	err = server.Properties.LockForWrite(HostProperty.SharesV1).ThenUse(func(v interface{}) error {
		serverSharesV1 := v.(*propsv1.HostShares)

		share = propsv1.NewHostShare()
		share.Name = shareName
		shareID, err := uuid.NewV4()
		if err != nil {
			return logicErrf(err, "Error creating UUID for share")
		}
		share.ID = shareID.String()
		share.Path = sharePath
		share.Type = "nfs"

		serverSharesV1.ByID[share.ID] = share
		serverSharesV1.ByName[share.Name] = share.ID

		return nil
	})
	if err != nil {
		return nil, err
	}

	mh, err := metadata.SaveHost(handler.service, server)
	if err != nil {
		return nil, logicErrf(err, "Error saving server metadata")
	}
	defer func() {
		if err != nil {
			err2 := server.Properties.LockForWrite(HostProperty.SharesV1).ThenUse(func(v interface{}) error {
				serverSharesV1 := v.(*propsv1.HostShares)
				delete(serverSharesV1.ByID, share.ID)
				delete(serverSharesV1.ByName, share.Name)
				return nil
			})
			if err2 != nil {
				log.Warnf("Failed to set shares metadata of host %s", hostName)
			}
			err2 = mh.Write()
			if err2 != nil {
				log.Warnf("Failed to save metadata of host %s", hostName)
			}
		}
	}()
	ms, err := metadata.SaveShare(handler.service, server.ID, server.Name, share.ID, share.Name)
	if err != nil {
		return nil, infraErr(err)
	}
	defer func() {
		if err != nil {
			derr := ms.Delete()
			if derr != nil {
				log.Warnf("Failed to delete metadata of share '%s'", share.Name)
			}
		}
	}()

	select {
	case <-ctx.Done():
		log.Warnf("Share creation cancelled by user")
		err = fmt.Errorf("share creation cancelled by user")
		return nil, err
	default:
	}

	return share, nil
}

// Delete a share from host
func (handler *ShareHandler) Delete(ctx context.Context, name string) error {
	// Retrieve info about the share
	server, share, _, err := handler.ForceInspect(ctx, name)
	if err != nil {
		return throwErr(err)
	}
	if server == nil {
		return throwErrf("delete share: unable to inspect host '%s'", name)
	}
	if share == nil {
		return throwErrf("Delete share: unable to found share of host '%s'", name)
	}

	err = server.Properties.LockForWrite(HostProperty.SharesV1).ThenUse(func(v interface{}) error {
		serverSharesV1 := v.(*propsv1.HostShares)
		if len(share.ClientsByName) > 0 {
			var list []string
			for k := range share.ClientsByName {
				list = append(list, "'"+k+"'")
			}
			return logicErr(fmt.Errorf("still used by: %s", strings.Join(list, ",")))
		}

		sshHandler := NewSSHHandler(handler.service)
		sshConfig, err := sshHandler.GetConfig(ctx, server.ID)
		if err != nil {
			return infraErr(err)
		}

		nfsServer, err := nfs.NewServer(sshConfig)
		if err != nil {
			return infraErr(err)
		}
		err = nfsServer.RemoveShare(share.Path)
		if err != nil {
			return infraErr(err)
		}

		delete(serverSharesV1.ByID, share.ID)
		delete(serverSharesV1.ByName, share.Name)
		return nil
	})
	if err != nil {
		return err
	}

	// Save server metadata
	_, err = metadata.SaveHost(handler.service, server)
	if err != nil {
		return infraErr(err)
	}

	// Remove share metadata
	err = metadata.RemoveShare(handler.service, server.ID, server.Name, share.ID, share.Name)
	if err != nil {
		return infraErr(err)
	}

	select {
	case <-ctx.Done():
		log.Warnf("Share deletion cancelled by user")
		_, err = handler.Create(context.Background(), share.Name, server.Name, share.Path, []string{}, false, false, false, false, false, false, false)
		if err != nil {
			return fmt.Errorf("failed to stop share deletion")
		}
		return fmt.Errorf("share deletion cancelled by user")
	default:
	}

	return nil
}

// List return the list of all shares from all servers
func (handler *ShareHandler) List(ctx context.Context) (map[string]map[string]*propsv1.HostShare, error) {
	shares := map[string]map[string]*propsv1.HostShare{}

	var servers []string
	ms := metadata.NewShare(handler.service)
	err := ms.Browse(func(hostName string, shareID string) error {
		servers = append(servers, hostName)
		return nil
	})
	if err != nil {
		return nil, logicErrf(err, "error browsing shares")
	}

	// Now walks through the hosts acting as Nas
	if len(servers) == 0 {
		return shares, nil
	}

	hostSvc := NewHostHandler(handler.service)
	for _, serverID := range servers {
		host, err := hostSvc.Inspect(ctx, serverID)
		if err != nil {
			return nil, infraErr(err)
		}

		err = host.Properties.LockForRead(HostProperty.SharesV1).ThenUse(func(v interface{}) error {
			hostSharesV1 := v.(*propsv1.HostShares)
			shares[serverID] = hostSharesV1.ByID
			return nil
		})
		if err != nil {
			return nil, infraErr(err)
		}
	}
	return shares, nil
}

// Mount a share on a local directory of an host
func (handler *ShareHandler) Mount(ctx context.Context, shareName, hostName, path string, withCache bool) (*propsv1.HostRemoteMount, error) {
	// Retrieve info about the share
	server, share, _, err := handler.Inspect(ctx, shareName)
	if err != nil {
		return nil, throwErr(err)
	}
	if share == nil {
		return nil, resources.ResourceNotFoundError("share", shareName)
	}
	if server == nil {
		return nil, resources.ResourceNotFoundError("host", hostName)
	}

	// Sanitize path
	mountPath, err := sanitize(path)
	if err != nil {
		return nil, logicErr(fmt.Errorf("invalid mount path '%s': '%s'", path, err))
	}

	var target *resources.Host
	if server.Name == hostName || server.ID == hostName {
		target = server
	} else {
		hostSvc := NewHostHandler(handler.service)
		target, err = hostSvc.Inspect(ctx, hostName)
		if err != nil {
			return nil, throwErr(err)
		}
	}

	// Check if share is already mounted
	// Check if there is already volume mounted in the path (or in subpath)
	err = target.Properties.LockForRead(HostProperty.MountsV1).ThenUse(func(v interface{}) error {
		targetMountsV1 := v.(*propsv1.HostMounts)
		if s, ok := targetMountsV1.RemoteMountsByShareID[share.ID]; ok {
			return logicErr(fmt.Errorf("already mounted in '%s:%s'", target.Name, targetMountsV1.RemoteMountsByPath[s].Path))
		}
		for _, i := range targetMountsV1.LocalMountsByPath {
			if i.Path == path {
				// Can't mount a share in place of a volume (by convention, nothing technically preventing it)
				return logicErr(fmt.Errorf("there is already a volume in path '%s:%s'", target.Name, path))
			}
		}
		for _, i := range targetMountsV1.RemoteMountsByPath {
			if strings.Index(path, i.Path) == 0 {
				// Can't mount a share inside another share (at least by convention, if not technically)
				return logicErr(fmt.Errorf("there is already a share mounted in '%s:%s'", target.Name, i.Path))
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	export := ""
	err = target.Properties.LockForRead(HostProperty.NetworkV1).ThenUse(func(v interface{}) error {
		if v.(*propsv1.HostNetwork).DefaultGatewayPrivateIP == server.GetPrivateIP() {
			export = server.GetPrivateIP() + ":" + share.Path
		} else {
			export = server.GetAccessIP() + ":" + share.Path
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sshHandler := NewSSHHandler(handler.service)
	sshConfig, err := sshHandler.GetConfig(ctx, target)
	if err != nil {
		return nil, infraErr(err)
	}

	// Mount the share on host
	err = server.Properties.LockForWrite(HostProperty.SharesV1).ThenUse(func(v interface{}) error {
		serverSharesV1 := v.(*propsv1.HostShares)
		_, found := serverSharesV1.ByID[serverSharesV1.ByName[shareName]]
		if !found {
			return logicErr(fmt.Errorf("failed to find metadata about share '%s'", shareName))
		}
		shareID := serverSharesV1.ByName[shareName]

		nfsClient, err := nfs.NewNFSClient(sshConfig)
		if err != nil {
			err = infraErr(err)
			return err
		}
		err = nfsClient.Install()
		if err != nil {
			return infraErr(err)
		}

		err = nfsClient.Mount(export, mountPath, withCache)
		if err != nil {
			return infraErr(err)
		}

		serverSharesV1.ByID[shareID].ClientsByName[target.Name] = target.ID
		serverSharesV1.ByID[shareID].ClientsByID[target.ID] = target.Name
		return nil
	})
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			sshHandler := NewSSHHandler(handler.service)
			sshConfig, err := sshHandler.GetConfig(ctx, target)
			if err != nil {
				log.Warn(infraErr(err).Error())
			}

			nfsClient, err := nfs.NewNFSClient(sshConfig)
			if err != nil {
				err = infraErr(err)
				log.Warn(err.Error())
			}
			err = nfsClient.Install()
			if err != nil {
				log.Warn(infraErr(err).Error())
			}

			err = nfsClient.Unmount(export)
			if err != nil {
				log.Warn(infraErr(err).Error())
			}
		}
	}()

	var mount *propsv1.HostRemoteMount
	err = target.Properties.LockForWrite(HostProperty.MountsV1).ThenUse(func(v interface{}) error {
		targetMountsV1 := v.(*propsv1.HostMounts)
		// Make sure the HostMounts is correctly init if there are no mount yet
		if !target.Properties.Lookup(HostProperty.MountsV1) {
			targetMountsV1.Reset()
		}
		mount = propsv1.NewHostRemoteMount()
		mount.ShareID = share.ID
		mount.Export = export
		mount.Path = mountPath
		mount.FileSystem = "nfs"
		targetMountsV1.RemoteMountsByPath[mount.Path] = mount
		targetMountsV1.RemoteMountsByShareID[mount.ShareID] = mount.Path
		targetMountsV1.RemoteMountsByExport[mount.Export] = mount.Path
		return nil
	})
	if err != nil {
		return nil, err
	}

	mh, err := metadata.SaveHost(handler.service, server)
	if err != nil {
		return nil, infraErr(err)
	}
	defer func() {
		if err != nil {
			err2 := server.Properties.LockForWrite(HostProperty.SharesV1).ThenUse(func(v interface{}) error {
				serverSharesV1 := v.(*propsv1.HostShares)
				delete(serverSharesV1.ByID[serverSharesV1.ByName[shareName]].ClientsByName, target.Name)
				delete(serverSharesV1.ByID[serverSharesV1.ByName[shareName]].ClientsByID, target.ID)
				return nil
			})
			if err2 != nil {
				log.Warnf("Failed to remove mounted share %s from host %s metadatas", shareName, server.Name)
			}
			err2 = mh.Write()
			if err2 != nil {
				log.Warnf("Failed to save host %s metadatas : %s", server.Name, err2.Error())
			}
		}
	}()

	if target != server {
		_, err = metadata.SaveHost(handler.service, target)
		if err != nil {
			return nil, infraErr(err)
		}
	}

	defer func() {
		if err != nil {
			err2 := target.Properties.LockForWrite(HostProperty.MountsV1).ThenUse(func(v interface{}) error {
				targetMountsV1 := v.(*propsv1.HostMounts)
				delete(targetMountsV1.RemoteMountsByShareID, mount.ShareID)
				delete(targetMountsV1.RemoteMountsByPath, mount.Path)
				delete(targetMountsV1.RemoteMountsByExport, mount.Export)
				return nil
			})
			if err2 != nil {
				log.Warnf("Failed to remove mounted share '%s' from host '%s' metadata", shareName, hostName)
			}
			_, err2 = metadata.SaveHost(handler.service, target)
			if err2 != nil {
				log.Warnf("Failed to save host '%s' metadata : %s", hostName, err2.Error())
			}
		}
	}()

	select {
	case <-ctx.Done():
		log.Warnf("Share mount cancelled by user")
		err = fmt.Errorf("share mount cancelled by user")
		return nil, err
	default:
	}

	return mount, nil
}

// Unmount a share from local directory of an host
func (handler *ShareHandler) Unmount(ctx context.Context, shareName, hostName string) error {
	server, share, _, err := handler.ForceInspect(ctx, shareName)
	if err != nil {
		return throwErr(err)
	}

	var shareID string
	err = server.Properties.LockForRead(HostProperty.SharesV1).ThenUse(func(v interface{}) error {
		serverSharesV1 := v.(*propsv1.HostShares)
		var found bool
		shareID, found = serverSharesV1.ByName[shareName]
		if !found {
			return logicErr(fmt.Errorf("failed to find data about share '%s'", shareName))
		}
		// share := serverSharesV1.ByID[shareID]
		// remotePath := server.GetAccessIP() + ":" + share.Path
		return nil
	})
	if err != nil {
		return err
	}

	var target *resources.Host
	if server.Name == hostName || server.ID == hostName {
		target = server
	} else {
		hostSvc := NewHostHandler(handler.service)
		target, err = hostSvc.ForceInspect(ctx, hostName)
		if err != nil {
			return throwErr(err)
		}
	}

	var mountPath string
	err = target.Properties.LockForWrite(HostProperty.MountsV1).ThenUse(func(v interface{}) error {
		targetMountsV1 := v.(*propsv1.HostMounts)
		mount, found := targetMountsV1.RemoteMountsByPath[targetMountsV1.RemoteMountsByShareID[shareID]]
		if !found {
			return logicErr(fmt.Errorf("not mounted on host '%s'", target.Name))
		}

		// Unmount share from client
		sshHandler := NewSSHHandler(handler.service)
		sshConfig, err := sshHandler.GetConfig(ctx, target.ID)
		if err != nil {
			return infraErr(err)
		}
		nfsClient, err := nfs.NewNFSClient(sshConfig)
		if err != nil {
			return infraErr(err)
		}
		err = nfsClient.Unmount(server.GetAccessIP() + ":" + share.Path)
		if err != nil {
			return infraErr(err)
		}

		// Remove mount from mount list
		mountPath = mount.Path
		delete(targetMountsV1.RemoteMountsByShareID, mount.ShareID)
		delete(targetMountsV1.RemoteMountsByPath, mountPath)
		return nil
	})
	if err != nil {
		return err
	}

	// Remove host from client lists of the share
	err = server.Properties.LockForWrite(HostProperty.SharesV1).ThenUse(func(v interface{}) error {
		serverSharesV1 := v.(*propsv1.HostShares)
		delete(serverSharesV1.ByID[shareID].ClientsByName, target.Name)
		delete(serverSharesV1.ByID[shareID].ClientsByID, target.ID)
		return nil
	})
	if err != nil {
		return err
	}

	// Saves metadata
	_, err = metadata.SaveHost(handler.service, server)
	if err != nil {
		return infraErr(err)
	}
	if server != target {
		_, err = metadata.SaveHost(handler.service, target)
		if err != nil {
			return infraErr(err)
		}
	}

	select {
	case <-ctx.Done():
		log.Warnf("Share unmount cancelled by user")
		_, err = handler.Mount(context.Background(), shareName, hostName, mountPath, false)
		if err != nil {
			return fmt.Errorf("failed to stop share unmount")
		}
		return fmt.Errorf("share unmounting cancelled by user")
	default:
	}

	return nil
}

// ForceInspect returns the host and share corresponding to 'shareName'
func (handler *ShareHandler) ForceInspect(
	ctx context.Context,
	shareName string,
) (*resources.Host, *propsv1.HostShare, map[string]*propsv1.HostRemoteMount, error) {

	host, share, mounts, err := handler.Inspect(ctx, shareName)
	if err != nil {
		return nil, nil, nil, throwErr(err)
	}
	if host == nil {
		return nil, nil, nil, logicErr(fmt.Errorf("failed to find host exporting the share '%s'", shareName))
	}
	return host, share, mounts, nil
}

// Inspect returns the host and share corresponding to 'shareName'
// If share isn't found, return (nil, nil, nil, resources.ErrResourceNotFound)
func (handler *ShareHandler) Inspect(
	ctx context.Context,
	shareName string,
) (*resources.Host, *propsv1.HostShare, map[string]*propsv1.HostRemoteMount, error) {

	hostName, err := metadata.LoadShare(handler.service, shareName)
	if err != nil {
		if _, ok := err.(utils.ErrNotFound); ok {
			return nil, nil, nil, resources.ResourceNotFoundError("share", shareName)
		}
		return nil, nil, nil, infraErr(errors.Wrap(err, "error loading share metadata"))
	}
	if hostName == "" {
		return nil, nil, nil, infraErr(errors.Wrap(err, fmt.Sprintf("failed to find host sharing '%s'", shareName)))
	}

	hostSvc := NewHostHandler(handler.service)
	server, err := hostSvc.ForceInspect(ctx, hostName)
	if err != nil {
		return nil, nil, nil, throwErr(err)
	}

	var (
		shareID string
		share   *propsv1.HostShare
	)
	err = server.Properties.LockForRead(HostProperty.SharesV1).ThenUse(func(v interface{}) error {
		serverSharesV1 := v.(*propsv1.HostShares)
		var found bool
		shareID, found = serverSharesV1.ByName[shareName]
		if !found {
			shareID = shareName
			_, found = serverSharesV1.ByID[shareID]
		}
		if !found {
			return infraErr(resources.ResourceNotFoundError("share", fmt.Sprintf("no share named '%s'", shareName)))
		}
		share = serverSharesV1.ByID[shareID]
		return nil
	})
	if err != nil {
		return nil, nil, nil, err
	}

	mounts := map[string]*propsv1.HostRemoteMount{}
	for k := range share.ClientsByName {
		client, err := hostSvc.Inspect(ctx, k)
		if err != nil {
			log.Errorf("%+v", err)
			continue
		}

		err = client.Properties.LockForRead(HostProperty.MountsV1).ThenUse(func(v interface{}) error {
			clientMountsV1 := v.(*propsv1.HostMounts)
			mountPath, ok := clientMountsV1.RemoteMountsByShareID[shareID]
			if ok {
				mount := clientMountsV1.RemoteMountsByPath[mountPath]
				mounts[client.Name] = &propsv1.HostRemoteMount{
					Path:       mount.Path,
					FileSystem: mount.FileSystem,
				}
			}
			return nil
		})
		if err != nil {
			log.Errorln(err.Error())
			continue
		}
	}
	return server, share, mounts, nil
}

func (handler *ShareHandler) findShare(shareName string) (string, error) {
	hostName, err := metadata.LoadShare(handler.service, shareName)
	if err != nil {
		return "", infraErrf(err, "Failed to load Share metadata '%s'", shareName)
	}
	return hostName, nil
}