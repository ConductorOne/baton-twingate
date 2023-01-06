package connector

import (
	"context"
	"fmt"

	"github.com/ConductorOne/baton-twingate/pkg/connector/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	resource "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

type userResourceType struct {
	resourceType *v2.ResourceType
	domain       string
	client       *client.ConnectorClient
}

func (o *userResourceType) ResourceType(_ context.Context) *v2.ResourceType {
	return o.resourceType
}

func (o *userResourceType) List(ctx context.Context, _ *v2.ResourceId, pt *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)
	bag := &pagination.Bag{}
	err := bag.Unmarshal(pt.Token)
	if err != nil {
		return nil, "", nil, err
	}

	if bag.Current() == nil {
		bag.Push(pagination.PageState{
			ResourceTypeID: resourceTypeUser.Id,
		})
	}

	resp, err := o.client.ListUsers(ctx, bag.PageToken(), 100)
	if err != nil {
		return nil, "", nil, err
	}
	rv := make([]*v2.Resource, 0, len(resp.Users))
	for _, user := range resp.Users {
		if user.ID == "" {
			l.Error("twingate: user had no id", zap.String("email", user.Email))
			continue
		}
		annos := &v2.V1Identifier{
			Id: user.ID,
		}
		profile := userProfile(ctx, *user)
		userTrait := []resource.UserTraitOption{resource.WithUserProfile(profile), resource.WithEmail(user.Email, true), resource.WithStatus(v2.UserTrait_Status_STATUS_ENABLED)}
		userResource, err := resource.NewUserResource(fmt.Sprintf("%s %s", user.FirstName, user.LastName), resourceTypeUser, user.ID, userTrait, resource.WithAnnotation(annos))
		if err != nil {
			return nil, "", nil, err
		}
		rv = append(rv, userResource)
	}

	nextPage, err := bag.NextToken(resp.Pagination)
	if err != nil {
		return nil, "", nil, err
	}

	annotations := annotations.Annotations{}
	if resp.RateLimitDescription != nil {
		annotations.WithRateLimiting(resp.RateLimitDescription)
	}

	return rv, nextPage, annotations, nil
}

func (o *userResourceType) Entitlements(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func (o *userResourceType) Grants(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func userBuilder(client *client.ConnectorClient, domain string) *userResourceType {
	return &userResourceType{
		resourceType: resourceTypeUser,
		domain:       domain,
		client:       client,
	}
}

func userProfile(ctx context.Context, user client.User) map[string]interface{} {
	profile := make(map[string]interface{})
	profile["first_name"] = user.FirstName
	profile["last_name"] = user.LastName
	profile["is_admin"] = user.IsAdmin
	profile["email"] = user.Email
	profile["id"] = user.ID
	return profile
}
