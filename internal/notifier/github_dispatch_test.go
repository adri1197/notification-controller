/*
Copyright 2020 The Flux authors

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

package notifier

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	authgithub "github.com/fluxcd/pkg/git/github"
	"github.com/fluxcd/pkg/ssh"
)

func TestNewGitHubDispatchBasic(t *testing.T) {
	gomega := NewWithT(t)
	github, err := NewGitHubDispatch("https://github.com/foo/bar", "foobar", nil, "", "", "", nil, nil)
	gomega.Expect(err).ToNot(HaveOccurred())
	gomega.Expect(github.Owner).To(Equal("foo"))
	gomega.Expect(github.Repo).To(Equal("bar"))
	gomega.Expect(github.Client.BaseURL.Host).To(Equal("api.github.com"))
}

func TestNewEnterpriseGitHubDispatchBasic(t *testing.T) {
	gomega := NewWithT(t)
	github, err := NewGitHubDispatch("https://foobar.com/foo/bar", "foobar", nil, "", "", "", nil, nil)
	gomega.Expect(err).ToNot(HaveOccurred())
	gomega.Expect(github.Owner).To(Equal("foo"))
	gomega.Expect(github.Repo).To(Equal("bar"))
	gomega.Expect(github.Client.BaseURL.Host).To(Equal("foobar.com"))
}

func TestNewGitHubDispatchInvalidUrl(t *testing.T) {
	g := NewWithT(t)
	_, err := NewGitHubDispatch("https://github.com/foo/bar/baz", "foobar", nil, "", "", "", nil, nil)
	g.Expect(err).To(HaveOccurred())
}

func TestNewGitHubDispatchEmptyToken(t *testing.T) {
	g := NewWithT(t)
	_, err := NewGitHubDispatch("https://github.com/foo/bar", "", nil, "", "", "", nil, nil)
	g.Expect(err).To(HaveOccurred())
}

func TestNewGithubDispatchProvider(t *testing.T) {
	appID := "123"
	installationID := "456"
	kp, _ := ssh.GenerateKeyPair(ssh.RSA_4096)
	expiresAt := time.Now().UTC().Add(time.Hour)

	for _, tt := range []struct {
		name       string
		secretData map[string][]byte
		wantErr    error
	}{
		{
			name:    "nil provider, no token",
			wantErr: errors.New("github token or github app details must be specified"),
		},
		{
			name:       "provider with no github options",
			secretData: map[string][]byte{},
			wantErr:    errors.New("github token or github app details must be specified"),
		},
		{
			name: "provider with missing app ID in options ",
			secretData: map[string][]byte{
				"githubAppInstallationID": []byte(installationID),
				"githubAppPrivateKey":     kp.PrivateKey,
			},
			wantErr: errors.New("github token or github app details must be specified"),
		},
		{
			name: "provider with missing app installation ID in options ",
			secretData: map[string][]byte{
				"githubAppID":         []byte(appID),
				"githubAppPrivateKey": kp.PrivateKey,
			},
			wantErr: errors.New("app installation ID must be provided to use github app authentication"),
		},
		{
			name: "provider with missing app private key in options ",
			secretData: map[string][]byte{
				"githubAppID":             []byte(appID),
				"githubAppInstallationID": []byte(installationID),
			},
			wantErr: errors.New("private key must be provided to use github app authentication"),
		},
		{
			name: "provider with complete app authentication information",
			secretData: map[string][]byte{
				"githubAppID":             []byte(appID),
				"githubAppInstallationID": []byte(installationID),
				"githubAppPrivateKey":     kp.PrivateKey,
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			handler := func(w http.ResponseWriter, r *http.Request) {
				g := NewWithT(t)
				w.WriteHeader(http.StatusOK)
				var response []byte
				var err error
				response, err = json.Marshal(&authgithub.AppToken{Token: "access-token", ExpiresAt: expiresAt})
				g.Expect(err).ToNot(HaveOccurred())
				w.Write(response)
			}
			srv := httptest.NewServer(http.HandlerFunc(handler))
			t.Cleanup(func() {
				srv.Close()
			})

			if len(tt.secretData) > 0 {
				tt.secretData["githubAppBaseURL"] = []byte(srv.URL)
			}
			g := NewWithT(t)
			_, err := NewGitHubDispatch("https://github.com/foo/bar", "", nil, "", "foo", "bar", tt.secretData, nil)
			if tt.wantErr != nil {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(Equal(tt.wantErr))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}

func TestGitHubDispatch_PostUpdate(t *testing.T) {
	g := NewWithT(t)
	githubDispatch, err := NewGitHubDispatch("https://github.com/foo/bar", "foobar", nil, "", "", "", nil, nil)
	g.Expect(err).ToNot(HaveOccurred())

	event := testEvent()
	event.Metadata[eventv1.MetaCommitStatusKey] = eventv1.MetaCommitStatusUpdateValue
	err = githubDispatch.Post(context.TODO(), event)
	g.Expect(err).ToNot(HaveOccurred())
}
