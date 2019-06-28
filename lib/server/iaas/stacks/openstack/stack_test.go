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

package openstack_test

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/CS-SI/SafeScale/lib/server/iaas"
	"github.com/CS-SI/SafeScale/lib/server/iaas/tests"
)

var tester *tests.ServiceTester
var service iaas.Service

func getTester() (*tests.ServiceTester, error) {
	if tester == nil {
		the_service, err := getService()
		if err != nil {
			return nil, err
		}
		tester = &tests.ServiceTester{
			Service: the_service,
		}
	}
	return tester, nil
}

func getService() (iaas.Service, error) {
	if service == nil {
		tenant_name := "TestOpenstack"
		if tenant_override := os.Getenv("TEST_OPENSTACK"); tenant_override != "" {
			tenant_name = tenant_override
		}
		var err error
		service, err = iaas.UseService(tenant_name)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("You must provide a VALID tenant [%v], check your environment variables and your Safescale configuration files", tenant_name))
		}
	}
	return service, nil
}

/* TODO
   review the code to test with userdata.Prepare, or move the test to userdata ?

func Test_Template(t *testing.T) {
	client := getClient()
	//Data structure to apply to userdata.sh template
	type userData struct {
		User       string
		Key        string
		ConfIF     bool
		IsGateway  bool
		AddGateway bool
		ResolvConf string
		GatewayIP  string
	}
	dataBuffer := bytes.NewBufferString("")
	data := userData{
		User:       api.DefaultUser,
		Key:        "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
		ConfIF:     true,
		IsGateway:  true,
		AddGateway: true,
		ResolvConf: "dskjfdshjjkdhsksdhhkjs\nsfdsfsdq\ndfsqdfqsdfq",
		GatewayIP:  "172.1.2.1",
	}
	output, err := userdata.Prepare("phase1", client, dataBuffer, data)
	assert.Nil(t, err)
	fmt.Println(output.String())
}*/

func Test_ListImages(t *testing.T) {
	tt, err := getTester()
	if err != nil {
		t.Skip(err)
	}
	require.Nil(t, err)
	tt.ListImages(t)
}

func Test_ListHostTemplates(t *testing.T) {
	tt, err := getTester()
	if err != nil {
		t.Skip(err)
	}
	require.Nil(t, err)
	tt.ListHostTemplates(t)
}

func Test_CreateKeyPair(t *testing.T) {
	tt, err := getTester()
	if err != nil {
		t.Skip(err)
	}
	require.Nil(t, err)
	tt.CreateKeyPair(t)
}

func Test_GetKeyPair(t *testing.T) {
	tt, err := getTester()
	if err != nil {
		t.Skip(err)
	}
	require.Nil(t, err)
	tt.GetKeyPair(t)
}

func Test_ListKeyPairs(t *testing.T) {
	tt, err := getTester()
	if err != nil {
		t.Skip(err)
	}
	require.Nil(t, err)
	tt.ListKeyPairs(t)
}

func Test_Networks(t *testing.T) {
	tt, err := getTester()
	if err != nil {
		t.Skip(err)
	}
	require.Nil(t, err)
	tt.Networks(t)
}

func Test_Hosts(t *testing.T) {
	tt, err := getTester()
	if err != nil {
		t.Skip(err)
	}
	require.Nil(t, err)
	tt.Hosts(t)
}

func Test_StartStopHost(t *testing.T) {
	tt, err := getTester()
	if err != nil {
		t.Skip(err)
	}
	require.Nil(t, err)
	tt.StartStopHost(t)
}

func Test_Volume(t *testing.T) {
	tt, err := getTester()
	if err != nil {
		t.Skip(err)
	}
	require.Nil(t, err)
	tt.Volume(t)
}

func Test_VolumeAttachment(t *testing.T) {
	tt, err := getTester()
	if err != nil {
		t.Skip(err)
	}
	require.Nil(t, err)
	tt.VolumeAttachment(t)
}

func Test_Containers(t *testing.T) {
	tt, err := getTester()
	if err != nil {
		t.Skip(err)
	}
	require.Nil(t, err)
	tt.Containers(t)
}

// func Test_Objects(t *testing.T) {
// 	tt, err := getTester()
// 	require.Nil(t, err)
// 	tt.Objects(t)
// }
