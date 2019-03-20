package secrethub

import (
	"testing"
	"time"

	"github.com/keylockerbv/secrethub-cli/pkg/ui"
	"github.com/keylockerbv/secrethub-cli/pkg/secrethub/fakes"
	"github.com/keylockerbv/secrethub/testutil"
	"github.com/secrethub/secrethub-go/internals/api"
	"github.com/secrethub/secrethub-go/internals/errio"
	"github.com/secrethub/secrethub-go/pkg/secrethub"
	"github.com/secrethub/secrethub-go/pkg/secrethub/fakeclient"
)

func TestInspectRepo_Run(t *testing.T) {
	testutil.Unit(t)

	testErr := errio.Namespace("test").Code("test").Error("test error")

	testTime := time.Date(2018, 1, 1, 1, 1, 1, 1, time.UTC)

	cases := map[string]struct {
		cmd          RepoInspectCommand
		repoService  fakeclient.RepoService
		newClientErr error
		out          string
		err          error
	}{
		"succes one version": {
			cmd: RepoInspectCommand{
				path: "foo/bar/secret",
				timeFormatter: &fakes.TimeFormatter{
					Response: "2018-01-01T01:01:01+01:00",
				},
			},
			repoService: fakeclient.RepoService{
				Getter: fakeclient.RepoGetter{
					ArgPath: "foo/bar",
					ReturnsRepo: &api.Repo{
						Name:        "bar",
						Owner:       "Repo Owner",
						CreatedAt:   &testTime,
						SecretCount: 1,
					},
				},
				UserService: &fakeclient.RepoUserService{
					Lister: fakeclient.RepoUserLister{
						ReturnsUsers: []*api.User{
							{
								Username: "dev 1",
								FullName: "uno",
							},
							{
								Username: "dev 2",
								FullName: "dos",
							},
						},
					},
				},
				ServiceService: &fakeclient.RepoServiceService{
					Lister: fakeclient.RepoServiceLister{
						ReturnsServices: []*api.Service{
							{
								ServiceID:   "ser1-1",
								Description: "This is service 1",
							},
							{
								ServiceID:   "ser1-2",
								Description: "This is service 2",
							},
						},
					},
				},
			},
			out: "" +
				"{\n" +
				"    \"Name\": \"bar\",\n" +
				"    \"Owner\": \"Repo Owner\",\n" +
				"    \"CreatedAt\": \"2018-01-01T01:01:01+01:00\",\n" +
				"    \"SecretCount\": 1,\n" +
				"    \"MemberCount\": 2,\n" +
				"    \"Users\": [\n" +
				"        {\n" +
				"            \"User\": \"uno\",\n" +
				"            \"UserName\": \"dev 1\"\n" +
				"        },\n" +
				"        {\n" +
				"            \"User\": \"dos\",\n" +
				"            \"UserName\": \"dev 2\"\n" +
				"        }\n" +
				"    ],\n" +
				"    \"ServiceCount\": 2,\n" +
				"    \"Services\": [\n" +
				"        {\n" +
				"            \"Service\": \"ser1-1\",\n" +
				"            \"ServiceDescription\": \"This is service 1\"\n" +
				"        },\n" +
				"        {\n" +
				"            \"Service\": \"ser1-2\",\n" +
				"            \"ServiceDescription\": \"This is service 2\"\n" +
				"        }\n" +
				"    ]\n" +
				"}\n",
		},
		"no client": {
			newClientErr: testErr,
			err:          testErr,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			tc.cmd.newClient = func() (secrethub.Client, error) {
				return fakeclient.Client{
					RepoService: &tc.repoService,
				}, tc.newClientErr
			}

			io := ui.NewFakeIO()
			tc.cmd.io = io

			// Act
			err := tc.cmd.Run()

			// Assert
			testutil.Compare(t, err, tc.err)
			testutil.Compare(t, io.StdOut.String(), tc.out)
		})
	}
}