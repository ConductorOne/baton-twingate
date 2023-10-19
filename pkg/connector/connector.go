package connector

import (
	"context"
	"io"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-twingate/pkg/connector/client"
)

var (
	resourceTypeRole = &v2.ResourceType{
		Id:          "role",
		DisplayName: "Role",
		Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_ROLE},
	}
	resourceTypeGroup = &v2.ResourceType{
		Id:          "group",
		DisplayName: "Group",
		Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_GROUP},
	}
	resourceTypeUser = &v2.ResourceType{
		Id:          "user",
		DisplayName: "User",
		Traits: []v2.ResourceType_Trait{
			v2.ResourceType_TRAIT_USER,
		},
		Annotations: annotationsForUserResourceType(),
	}
)

type Config struct {
	Domain string
	ApiKey string
}
type Twingate struct {
	client *client.ConnectorClient
	domain string
	apiKey string
}

func New(ctx context.Context, config Config) (*Twingate, error) {
	client, err := client.New(ctx, config.ApiKey, config.Domain)
	if err != nil {
		return nil, err
	}
	rv := &Twingate{
		domain: config.Domain,
		apiKey: config.ApiKey,
		client: client,
	}
	return rv, nil
}

func (c *Twingate) Metadata(ctx context.Context) (*v2.ConnectorMetadata, error) {
	var annos annotations.Annotations
	annos.Update(&v2.ExternalLink{
		Url: c.domain,
	})

	return &v2.ConnectorMetadata{
		DisplayName: "Twingate",
		Description: "Connector syncing Twingate users, groups, and roles to Baton",
		Annotations: annos,
	}, nil
}

func (c *Twingate) Validate(ctx context.Context) (annotations.Annotations, error) {
	_, err := c.client.ListUsers(ctx, "", 1)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (c *Twingate) Asset(ctx context.Context, asset *v2.AssetRef) (string, io.ReadCloser, error) {
	return "", nil, nil
}

func (c *Twingate) ResourceSyncers(ctx context.Context) []connectorbuilder.ResourceSyncer {
	return []connectorbuilder.ResourceSyncer{
		groupBuilder(c.client, c.domain),
		roleBuilder(c.client, c.domain),
		userBuilder(c.client, c.domain),
	}
}
