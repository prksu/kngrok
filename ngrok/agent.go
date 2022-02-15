/*
Copyright 2022 The Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ngrok

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"path"

	nerrors "github.com/prksu/kngrok/ngrok/errors"
)

const baseURL = "http://127.0.0.1:4040/api/"

type Tunnel struct {
	Name      string       `json:"name"`
	Addr      string       `json:"addr,omitempty"`
	URI       string       `json:"uri,omitempty"`
	PublicURL string       `json:"public_url,omitempty"`
	Proto     string       `json:"proto"`
	Config    TunnelConfig `json:"config,omitempty"`
}

type TunnelConfig struct {
	Addr       string `json:"addr,omitempty"`
	Proto      string `json:"proto,omitempty"`
	RemoteAddr string `json:"remote_addr,omitempty"`
}

var DefaultAgent Agent = &AgentClient{Client: &http.Client{}}

type Agent interface {
	Find(ctx context.Context, tunnelName string) (*Tunnel, error)
	Start(ctx context.Context, tunnelName string, config TunnelConfig) (*Tunnel, error)
	Stop(ctx context.Context, tunnelName string) error
}

type AgentClient struct {
	*http.Client
}

func (c *AgentClient) Find(ctx context.Context, tunnelName string) (*Tunnel, error) {
	u := baseURL + path.Join("tunnels", tunnelName)
	resp, err := c.Get(u)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusOK {
		tunnel := &Tunnel{}
		if err := json.NewDecoder(resp.Body).Decode(tunnel); err != nil {
			return nil, err
		}

		return tunnel, nil
	}

	var nerr nerrors.Error
	if err := json.NewDecoder(resp.Body).Decode(&nerr); err != nil {
		return nil, err
	}

	return nil, nerr
}

func (c *AgentClient) Start(ctx context.Context, tunnelName string, config TunnelConfig) (*Tunnel, error) {
	u := baseURL + "tunnels"
	b := struct {
		Name         string `json:"name"`
		TunnelConfig `json:",inline"`
	}{
		Name:         tunnelName,
		TunnelConfig: config,
	}

	rb, _ := json.Marshal(b)
	resp, err := c.Post(u, "application/json", bytes.NewReader(rb))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusCreated {
		return c.Find(ctx, tunnelName)
	}

	var nerr nerrors.Error
	if err := json.NewDecoder(resp.Body).Decode(&nerr); err != nil {
		return nil, err
	}

	return nil, nerr
}

func (c *AgentClient) Stop(ctx context.Context, tunnelName string) error {
	u := baseURL + path.Join("tunnels", tunnelName)
	req, err := http.NewRequest(http.MethodDelete, u, nil)
	if err != nil {
		return err
	}

	resp, err := c.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	var nerr nerrors.Error
	if err := json.NewDecoder(resp.Body).Decode(&nerr); err != nil {
		return err
	}

	return nerr
}
