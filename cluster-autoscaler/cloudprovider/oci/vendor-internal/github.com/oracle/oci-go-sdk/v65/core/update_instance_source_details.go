// Copyright (c) 2016, 2018, 2025, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.
// Code generated. DO NOT EDIT.

// Core Services API
//
// Use the Core Services API to manage resources such as virtual cloud networks (VCNs),
// compute instances, and block storage volumes. For more information, see the console
// documentation for the Networking (https://docs.oracle.com/iaas/Content/Network/Concepts/overview.htm),
// Compute (https://docs.oracle.com/iaas/Content/Compute/Concepts/computeoverview.htm), and
// Block Volume (https://docs.oracle.com/iaas/Content/Block/Concepts/overview.htm) services.
// The required permissions are documented in the
// Details for the Core Services (https://docs.oracle.com/iaas/Content/Identity/Reference/corepolicyreference.htm) article.
//

package core

import (
	"encoding/json"
	"fmt"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/oci/vendor-internal/github.com/oracle/oci-go-sdk/v65/common"
	"strings"
)

// UpdateInstanceSourceDetails The details for updating the instance source.
type UpdateInstanceSourceDetails interface {

	// Whether to preserve the boot volume that was previously attached to the instance after a successful replacement of that boot volume.
	GetIsPreserveBootVolumeEnabled() *bool
}

type updateinstancesourcedetails struct {
	JsonData                    []byte
	IsPreserveBootVolumeEnabled *bool  `mandatory:"false" json:"isPreserveBootVolumeEnabled"`
	SourceType                  string `json:"sourceType"`
}

// UnmarshalJSON unmarshals json
func (m *updateinstancesourcedetails) UnmarshalJSON(data []byte) error {
	m.JsonData = data
	type Unmarshalerupdateinstancesourcedetails updateinstancesourcedetails
	s := struct {
		Model Unmarshalerupdateinstancesourcedetails
	}{}
	err := json.Unmarshal(data, &s.Model)
	if err != nil {
		return err
	}
	m.IsPreserveBootVolumeEnabled = s.Model.IsPreserveBootVolumeEnabled
	m.SourceType = s.Model.SourceType

	return err
}

// UnmarshalPolymorphicJSON unmarshals polymorphic json
func (m *updateinstancesourcedetails) UnmarshalPolymorphicJSON(data []byte) (interface{}, error) {

	if data == nil || string(data) == "null" {
		return nil, nil
	}

	var err error
	switch m.SourceType {
	case "bootVolume":
		mm := UpdateInstanceSourceViaBootVolumeDetails{}
		err = json.Unmarshal(data, &mm)
		return mm, err
	case "image":
		mm := UpdateInstanceSourceViaImageDetails{}
		err = json.Unmarshal(data, &mm)
		return mm, err
	default:
		common.Logf("Received unsupported enum value for UpdateInstanceSourceDetails: %s.", m.SourceType)
		return *m, nil
	}
}

// GetIsPreserveBootVolumeEnabled returns IsPreserveBootVolumeEnabled
func (m updateinstancesourcedetails) GetIsPreserveBootVolumeEnabled() *bool {
	return m.IsPreserveBootVolumeEnabled
}

func (m updateinstancesourcedetails) String() string {
	return common.PointerString(m)
}

// ValidateEnumValue returns an error when providing an unsupported enum value
// This function is being called during constructing API request process
// Not recommended for calling this function directly
func (m updateinstancesourcedetails) ValidateEnumValue() (bool, error) {
	errMessage := []string{}

	if len(errMessage) > 0 {
		return true, fmt.Errorf(strings.Join(errMessage, "\n"))
	}
	return false, nil
}
