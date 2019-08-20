package secrethub

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"

	"github.com/secrethub/secrethub-cli/internals/cli/ui"

	"github.com/secrethub/secrethub-go/internals/api"
	"github.com/secrethub/secrethub-go/internals/errio"
)

// Errors
var (
	ErrInvalidAWSRegion      = errMain.Code("invalid_region").Error("invalid AWS region")
	ErrRoleAlreadyTaken      = errMain.Code("role_taken").Error("a service using that IAM role already exists")
	ErrInvalidPermissionPath = errMain.Code("invalid_permission_path").ErrorPref("invalid permission path: %s")
)

// ServiceAWSInitCommand initializes a service for AWS.
type ServiceAWSInitCommand struct {
	description string
	repo        api.RepoPath
	kmsKeyID    string
	role        string
	region      string
	permission  string
	io          ui.IO
	newClient   newClientFunc
}

// NewServiceAWSInitCommand creates a new ServiceAWSInitCommand.
func NewServiceAWSInitCommand(io ui.IO, newClient newClientFunc) *ServiceAWSInitCommand {
	return &ServiceAWSInitCommand{
		io:        io,
		newClient: newClient,
	}
}

// Run initializes an AWS service.
func (cmd *ServiceAWSInitCommand) Run() error {
	client, err := cmd.newClient()
	if err != nil {
		return err
	}

	cfg := aws.NewConfig()
	if cmd.region != "" {
		_, ok := endpoints.AwsPartition().Regions()[cmd.region]
		if !ok {
			return ErrInvalidAWSRegion
		}
		cfg = cfg.WithRegion(cmd.region)
	}

	service, err := client.Services().AWS().Create(cmd.repo.Value(), cmd.description, cmd.kmsKeyID, cmd.role, cfg)
	if err == api.ErrCredentialAlreadyExists {
		return ErrRoleAlreadyTaken
	} else if err != nil {
		return err
	}

	permissionPath := cmd.repo.GetDirPath()
	var permission api.Permission
	values := strings.SplitN(cmd.permission, ":", 2)
	if len(values) == 1 {
		err := permission.Set(values[0])
		if err != nil {
			return err
		}
	} else if len(values) == 2 {
		err := permission.Set(values[1])
		if err != nil {
			return err
		}

		permissionPath, err = api.NewDirPath(api.JoinPaths(permissionPath.String(), values[0]))
		if err != nil {
			return ErrInvalidPermissionPath(err)
		}
	}

	if permission != 0 {
		_, err = client.AccessRules().Set(permissionPath.Value(), permission.String(), service.ServiceID)
		if err != nil {
			_, delErr := client.Services().Delete(service.ServiceID)
			if delErr != nil {
				fmt.Fprintf(cmd.io.Stdout(), "Failed to cleanup after creating an access rule for %s failed. Be sure to manually remove the created service account %s: %s\n", service.ServiceID, service.ServiceID, err)
				return errio.Error(delErr)
			}

			return errio.Error(err)
		}
	}

	fmt.Fprintf(cmd.io.Stdout(), "The service %s is now reachable through AWS when the role %s is assumed.\n", service.ServiceID, cmd.role)

	return nil
}

// Register registers the command, arguments and flags on the provided Registerer.
func (cmd *ServiceAWSInitCommand) Register(r Registerer) {
	clause := r.Command("init", "Create a new AWS service account attached to a repository.")
	clause.Flag("repo", "The service account is attached to the repository in this path.").Required().SetValue(&cmd.repo)
	clause.Flag("kms-key-id", "ID of the KMS-key to be used for encrypting the service's account key.").Required().StringVar(&cmd.kmsKeyID)
	clause.Flag("role", "ARN of the IAM role that should have access to this service account.").Required().StringVar(&cmd.role)
	clause.Flag("region", "The AWS region that should be used").StringVar(&cmd.region)
	clause.Flag("desc", "A description for the service").StringVar(&cmd.description)
	clause.Flag("permission", "Create an access rule giving the service account permission on a directory. Accepted permissions are `read`, `write` and `admin`. Use <subdirectory>:<permission> format to give permission on a subdirectory of the repo.").StringVar(&cmd.permission)

	BindAction(clause, cmd.Run)
}
