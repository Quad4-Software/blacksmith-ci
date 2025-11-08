// Copyright 2025 Crow Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package crow

import "fmt"

const (
	pathAutoscalers         = "%s/api/autoscalers"
	pathAutoscaler          = "%s/api/autoscalers/%d"
	pathAutoscalerLookup    = "%s/api/autoscalers/lookup/%s"
	pathAutoscalerHeartbeat = "%s/api/autoscalers/%d/heartbeat"
	pathAutoscalerSelf      = "%s/api/autoscaler"
)

// AutoscalerList returns a list of all registered autoscalers.
func (c *client) AutoscalerList() ([]*Autoscaler, error) {
	out := make([]*Autoscaler, 0)
	uri := fmt.Sprintf(pathAutoscalers, c.addr)
	err := c.get(uri, &out)
	return out, err
}

// Autoscaler returns an autoscaler by id.
func (c *client) Autoscaler(autoscalerID int64) (*Autoscaler, error) {
	out := new(Autoscaler)
	uri := fmt.Sprintf(pathAutoscaler, c.addr, autoscalerID)
	err := c.get(uri, out)
	return out, err
}

// AutoscalerLookup returns an autoscaler by name.
func (c *client) AutoscalerLookup(name string) (*Autoscaler, error) {
	out := new(Autoscaler)
	uri := fmt.Sprintf(pathAutoscalerLookup, c.addr, name)
	err := c.get(uri, out)
	return out, err
}

// AutoscalerCreate creates a new autoscaler.
func (c *client) AutoscalerCreate(in *Autoscaler) (*Autoscaler, error) {
	out := new(Autoscaler)
	uri := fmt.Sprintf(pathAutoscalers, c.addr)
	err := c.post(uri, in, out)
	return out, err
}

// AutoscalerUpdate updates an existing autoscaler.
func (c *client) AutoscalerUpdate(in *Autoscaler) (*Autoscaler, error) {
	out := new(Autoscaler)
	uri := fmt.Sprintf(pathAutoscaler, c.addr, in.ID)
	err := c.patch(uri, in, out)
	return out, err
}

// AutoscalerDelete deletes an autoscaler.
func (c *client) AutoscalerDelete(autoscalerID int64) error {
	uri := fmt.Sprintf(pathAutoscaler, c.addr, autoscalerID)
	return c.delete(uri)
}

// AutoscalerHeartbeat sends a heartbeat for an autoscaler.
func (c *client) AutoscalerHeartbeat(autoscalerID int64, heartbeat *AutoscalerHeartbeat) error {
	uri := fmt.Sprintf(pathAutoscalerHeartbeat, c.addr, autoscalerID)
	return c.post(uri, heartbeat, nil)
}

// AutoscalerSelf returns the autoscaler associated with the current authentication token.
func (c *client) AutoscalerSelf() (*Autoscaler, error) {
	out := new(Autoscaler)
	uri := fmt.Sprintf(pathAutoscalerSelf, c.addr)
	err := c.get(uri, out)
	if err != nil {
		return nil, fmt.Errorf("GET %s failed: %w", uri, err)
	}
	return out, nil
}
