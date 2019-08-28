package secrethub

import (
	"errors"
	"testing"
	"time"

	"github.com/secrethub/secrethub-cli/internals/cli/ui"

	"github.com/secrethub/secrethub-go/internals/api"
	"github.com/secrethub/secrethub-go/internals/assert"
	"github.com/secrethub/secrethub-go/pkg/secrethub"
	"github.com/secrethub/secrethub-go/pkg/secrethub/fakeclient"
)

func TestServiceLsCommand_Run(t *testing.T) {
	cases := map[string]struct {
		cmd            ServiceLsCommand
		serviceService fakeclient.ServiceService
		newClientErr   error
		out            string
		err            error
	}{
		"success": {
			cmd: ServiceLsCommand{
				newServiceTable: newKeyServiceTable,
			},
			serviceService: fakeclient.ServiceService{
				Lister: fakeclient.RepoServiceLister{
					ReturnsServices: []*api.Service{
						{
							ServiceID:   "test",
							Description: "foobar",
							Credential: &api.Credential{
								Type: api.CredentialType("key"),
							},
							CreatedAt: time.Now().Add(-1 * time.Hour),
						},
						{
							ServiceID:   "second",
							Description: "foobarbaz",
							Credential: &api.Credential{
								Type: api.CredentialType("key"),
							},
							CreatedAt: time.Now().Add(-2 * time.Hour),
						},
					},
				},
			},
			out: "ID      DESCRIPTION  CREATED            TYPE\ntest    foobar       About an hour ago  key\nsecond  foobarbaz    2 hours ago        key\n",
		},
		"success quiet": {
			cmd: ServiceLsCommand{
				quiet: true,
			},
			serviceService: fakeclient.ServiceService{
				Lister: fakeclient.RepoServiceLister{
					ReturnsServices: []*api.Service{
						{
							ServiceID:   "test",
							Description: "foobar",
						},
						{
							ServiceID:   "second",
							Description: "foobarbaz",
						},
					},
				},
			},
			out: "test\nsecond\n",
		},
		"new client error": {
			newClientErr: errors.New("error"),
			err:          errors.New("error"),
		},
		"client list error": {
			serviceService: fakeclient.ServiceService{
				Lister: fakeclient.RepoServiceLister{
					Err: errors.New("error"),
				},
			},
			err: errors.New("error"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Setup
			io := ui.NewFakeIO()
			tc.cmd.io = io

			if tc.newClientErr != nil {
				tc.cmd.newClient = func() (secrethub.ClientAdapter, error) {
					return nil, tc.newClientErr
				}
			} else {
				tc.cmd.newClient = func() (secrethub.ClientAdapter, error) {
					return fakeclient.Client{
						ServiceService: &tc.serviceService,
					}, nil
				}
			}

			// Act
			err := tc.cmd.Run()

			// Assert
			assert.Equal(t, err, tc.err)
			assert.Equal(t, io.StdOut.String(), tc.out)
		})
	}
}
