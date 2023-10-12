package connector

import (
	"context"
	"fmt"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	ent "github.com/conductorone/baton-sdk/pkg/types/entitlement"
	grant "github.com/conductorone/baton-sdk/pkg/types/grant"
	res "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-twingate/pkg/connector/client"
)

const (
	groupMemberEntitlement = "member"
)

type groupResourceType struct {
	resourceType *v2.ResourceType
	domain       string
	client       *client.ConnectorClient
}

func (o *groupResourceType) ResourceType(_ context.Context) *v2.ResourceType {
	return o.resourceType
}

func groupResource(ctx context.Context, group client.Group) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"group_id":   group.ID,
		"group_name": group.Name,
	}

	groupTraitOptions := []res.GroupTraitOption{
		res.WithGroupProfile(profile),
	}

	resource, err := res.NewGroupResource(
		group.Name,
		resourceTypeGroup,
		group.ID,
		groupTraitOptions,
	)
	if err != nil {
		return nil, err
	}

	return resource, nil
}

func (o *groupResourceType) List(ctx context.Context, resourceId *v2.ResourceId, pt *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	bag := &pagination.Bag{}
	err := bag.Unmarshal(pt.Token)
	if err != nil {
		return nil, "", nil, err
	}
	if bag.Current() == nil {
		bag.Push(pagination.PageState{
			ResourceTypeID: resourceTypeGroup.Id,
		})
	}
	resp, err := o.client.ListGroups(ctx, bag.PageToken(), ResourcesPageSize)
	if err != nil {
		return nil, "", nil, err
	}

	rv := make([]*v2.Resource, 0, len(resp.Groups))
	for _, g := range resp.Groups {
		groupCopy := g
		gr, err := groupResource(ctx, groupCopy)
		if err != nil {
			return nil, "", nil, err
		}

		rv = append(rv, gr)
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

func (o *groupResourceType) Entitlements(ctx context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	var rv []*v2.Entitlement

	assignmentOptions := []ent.EntitlementOption{
		ent.WithGrantableTo(resourceTypeUser),
		ent.WithDisplayName(fmt.Sprintf("%s Group Member", resource.DisplayName)),
		ent.WithDescription(fmt.Sprintf("Is member of the %s group in Twingate", resource.DisplayName)),
	}

	rv = append(rv, ent.NewAssignmentEntitlement(resource, groupMemberEntitlement, assignmentOptions...))

	return rv, "", nil, nil
}

func (o *groupResourceType) Grants(ctx context.Context, resource *v2.Resource, pt *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	resp, err := o.client.ListGroupGrants(ctx, resource.Id.Resource)
	if err != nil {
		return nil, "", nil, err
	}
	var rv []*v2.Grant
	for _, groupGrant := range resp.Grants {
		rv = append(rv, grant.NewGrant(
			resource,
			groupMemberEntitlement,
			&v2.ResourceId{
				ResourceType: resourceTypeUser.Id,
				Resource:     groupGrant.PrincipalID,
			},
		))
	}
	annotations := annotations.Annotations{}
	if resp.RateLimitDescription != nil {
		annotations.WithRateLimiting(resp.RateLimitDescription)
	}
	return rv, "", annotations, nil
}

func groupBuilder(client *client.ConnectorClient, domain string) *groupResourceType {
	return &groupResourceType{
		resourceType: resourceTypeGroup,
		domain:       domain,
		client:       client,
	}
}
