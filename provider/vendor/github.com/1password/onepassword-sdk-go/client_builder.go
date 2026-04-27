package onepassword

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	"github.com/1password/onepassword-sdk-go/internal"
)

const (
	DefaultIntegrationName    = "Unknown"
	DefaultIntegrationVersion = "Unknown"
)

// NewClient returns a 1Password Go SDK client using the provided ClientOption list.
func NewClient(ctx context.Context, opts ...ClientOption) (*Client, error) {
	client := Client{
		config: internal.NewDefaultConfig(),
	}

	for _, opt := range opts {
		err := opt(&client)
		if err != nil {
			return nil, err
		}
	}

	if client.config.AccountName != nil && client.config.SAToken != "" {
		return nil, fmt.Errorf("cannot use both SA token and desktop app authentication")
	}

	var core *internal.CoreWrapper
	var err error
	if client.config.AccountName != nil {
		core, err = internal.GetSharedLibCore(*client.config.AccountName)
	} else {
		core, err = internal.GetExtismCore()
	}

	if err != nil {
		return nil, err
	}
	return initClient(ctx, *core, client)
}

// Initializes the client with the backend and gets it ready for later invocations.
func initClient(ctx context.Context, core internal.CoreWrapper, client Client) (*Client, error) {
	clientID, err := core.InitClient(ctx, client.config)
	if err != nil {
		return nil, fmt.Errorf("error initializing client: %w", unmarshalError(err.Error()))
	}

	inner := internal.InnerClient{
		ID:     *clientID,
		Core:   core,
		Config: client.config,
	}

	initAPIs(&client, &inner)

	runtime.SetFinalizer(&client, func(f *Client) {
		core.ReleaseClient(*clientID)
	})
	return &client, nil
}

type ClientOption func(client *Client) error

// WithServiceAccountToken specifies the [1Password Service Account](https://developer.1password.com/docs/service-accounts) token to use to authenticate the SDK client. Read more about how to get started with service accounts: https://developer.1password.com/docs/service-accounts/get-started/#create-a-service-account
func WithServiceAccountToken(token string) ClientOption {
	return func(c *Client) error {
		c.config.SAToken = token
		return nil
	}
}

// WithIntegrationInfo specifies the name and version of the integration built using the 1Password Go SDK. If you don't know which name and version to use, use `DefaultIntegrationName` and `DefaultIntegrationVersion`, respectively.
func WithIntegrationInfo(name string, version string) ClientOption {
	return func(c *Client) error {
		c.config.IntegrationName = name
		c.config.IntegrationVersion = version
		return nil
	}
}

func clientInvoke(ctx context.Context, innerClient *internal.InnerClient, invocation string, params map[string]interface{}) (*string, error) {
	invocationResponse, err := innerClient.Core.Invoke(ctx, internal.InvokeConfig{
		Invocation: internal.Invocation{
			ClientID: &innerClient.ID,
			Parameters: internal.Parameters{
				MethodName:       invocation,
				SerializedParams: params,
			},
		},
	})
	if err != nil {
		err = unmarshalError(err.Error())
		var e *DesktopSessionExpiredError
		if errors.As(err, &e) {
			var clientID *uint64
			clientID, err = innerClient.Core.InitClient(ctx, innerClient.Config)
			if err != nil {
				return nil, err
			}
			innerClient.ID = *clientID
			invocationResponse, err = innerClient.Core.Invoke(ctx, internal.InvokeConfig{
				Invocation: internal.Invocation{
					ClientID: &innerClient.ID,
					Parameters: internal.Parameters{
						MethodName:       invocation,
						SerializedParams: params,
					},
				},
			})
			if err == nil {
				return invocationResponse, nil
			}
		}

		return nil, err
	}
	return invocationResponse, nil
}
