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

package opentelekom

import (
	"github.com/CS-SI/SafeScale/providers/model"
)

// CreateContainer creates an object container
func (client *Client) CreateContainer(name string) error {
	return client.feclt.CreateContainer(name)
}

// GetContainer get container info
func (client *Client) GetContainer(name string) (*model.ContainerInfo, error) {
	//	return s3.GetContainer(awss3.New(client.S3Session), name)
	return client.feclt.GetContainer(name)
}

// DeleteContainer deletes an object container
func (client *Client) DeleteContainer(name string) error {
	return client.feclt.DeleteContainer(name)
}

// ListContainers list object containers
func (client *Client) ListContainers() ([]string, error) {
	return client.feclt.ListContainers()
}

// PutObject put an object into an object container
func (client *Client) PutObject(container string, obj model.Object) error {
	return client.feclt.PutObject(container, obj)
}

// UpdateObjectMetadata update an object into an object container
func (client *Client) UpdateObjectMetadata(container string, obj model.Object) error {
	return client.feclt.UpdateObjectMetadata(container, obj)
}

// GetObject get object content from an object container
func (client *Client) GetObject(container string, name string, ranges []model.Range) (*model.Object, error) {
	return client.feclt.GetObject(container, name, ranges)
}

// GetObjectMetadata get  object metadata from an object container
func (client *Client) GetObjectMetadata(container string, name string) (*model.Object, error) {
	return client.feclt.GetObjectMetadata(container, name)
}

// ListObjects list objects of a container
func (client *Client) ListObjects(container string, filter model.ObjectFilter) ([]string, error) {
	return client.feclt.ListObjects(container, filter)
}

// CopyObject copies an object
func (client *Client) CopyObject(containerSrc, objectSrc, objectDst string) error {
	return client.feclt.CopyObject(containerSrc, objectSrc, objectDst)
}

// DeleteObject deleta an object from a container
func (client *Client) DeleteObject(container, object string) error {
	return client.feclt.DeleteObject(container, object)
}