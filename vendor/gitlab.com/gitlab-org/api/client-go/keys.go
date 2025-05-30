//
// Copyright 2021, Patrick Webster
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package gitlab

import (
	"fmt"
	"net/http"
	"time"
)

type (
	KeysServiceInterface interface {
		GetKeyWithUser(key int, options ...RequestOptionFunc) (*Key, *Response, error)
		GetKeyByFingerprint(opt *GetKeyByFingerprintOptions, options ...RequestOptionFunc) (*Key, *Response, error)
	}

	// KeysService handles communication with the
	// keys related methods of the GitLab API.
	//
	// GitLab API docs:
	// https://docs.gitlab.com/api/keys/
	KeysService struct {
		client *Client
	}
)

var _ KeysServiceInterface = (*KeysService)(nil)

// Key represents a GitLab user's SSH key.
//
// GitLab API docs:
// https://docs.gitlab.com/api/keys/
type Key struct {
	ID        int        `json:"id"`
	Title     string     `json:"title"`
	Key       string     `json:"key"`
	CreatedAt *time.Time `json:"created_at"`
	User      User       `json:"user"`
}

// GetKeyWithUser gets a single key by id along with the associated
// user information.
//
// GitLab API docs:
// https://docs.gitlab.com/api/keys/#get-ssh-key-with-user-by-id-of-an-ssh-key
func (s *KeysService) GetKeyWithUser(key int, options ...RequestOptionFunc) (*Key, *Response, error) {
	u := fmt.Sprintf("keys/%d", key)

	req, err := s.client.NewRequest(http.MethodGet, u, nil, options)
	if err != nil {
		return nil, nil, err
	}

	k := new(Key)
	resp, err := s.client.Do(req, k)
	if err != nil {
		return nil, resp, err
	}

	return k, resp, nil
}

// GetKeyByFingerprintOptions represents the available GetKeyByFingerprint()
// options.
//
// GitLab API docs:
// https://docs.gitlab.com/api/keys/#get-user-by-fingerprint-of-ssh-key
// https://docs.gitlab.com/api/keys/#get-user-by-deploy-key-fingerprint
type GetKeyByFingerprintOptions struct {
	Fingerprint string `url:"fingerprint" json:"fingerprint"`
}

// GetKeyByFingerprint gets a specific SSH key or deploy key by fingerprint
// along with the associated user information.
//
// GitLab API docs:
// https://docs.gitlab.com/api/keys/#get-user-by-fingerprint-of-ssh-key
// https://docs.gitlab.com/api/keys/#get-user-by-deploy-key-fingerprint
func (s *KeysService) GetKeyByFingerprint(opt *GetKeyByFingerprintOptions, options ...RequestOptionFunc) (*Key, *Response, error) {
	req, err := s.client.NewRequest(http.MethodGet, "keys", opt, options)
	if err != nil {
		return nil, nil, err
	}

	k := new(Key)
	resp, err := s.client.Do(req, k)
	if err != nil {
		return nil, resp, err
	}

	return k, resp, nil
}
