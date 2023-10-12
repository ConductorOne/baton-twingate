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
	resp, err := o.client.ListGroups(ctx, bag.PageToken(), 100)
	if err != nil {
		return nil, "", nil, err
	}

	rv := make([]*v2.Resource, 0, len(resp.Groups))
	for _, g := range resp.Groups {
		annos := &v2.V1Identifier{
			Id: g.ID,
		}
		profile := groupProfile(ctx, g)
		groupTrait := []res.GroupTraitOption{res.WithGroupProfile(profile)}
		groupResource, err := res.NewGroupResource(g.Name, resourceTypeGroup, g.ID, groupTrait, res.WithAnnotation(annos))
		if err != nil {
			return nil, "", nil, err
		}
		rv = append(rv, groupResource)
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
	var annos annotations.Annotations
	annos.Update(&v2.V1Identifier{
		Id: V1MembershipEntitlementID(resource.Id.Resource),
	})
	member := ent.NewAssignmentEntitlement(resource, groupMemberEntitlement, ent.WithGrantableTo(resourceTypeUser))
	member.Description = fmt.Sprintf("Is member of the %s group in Twingate", resource.DisplayName)
	member.Annotations = annos
	member.DisplayName = fmt.Sprintf("%s Group Member", resource.DisplayName)
	return []*v2.Entitlement{member}, "", nil, nil
}

func (o *groupResourceType) Grants(ctx context.Context, resource *v2.Resource, pt *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	resp, err := o.client.ListGroupGrants(ctx, resource.Id.Resource)
	if err != nil {
		return nil, "", nil, err
	}
	var rv []*v2.Grant
	for _, groupGrant := range resp.Grants {
		v1Identifier := &v2.V1Identifier{
			Id: V1GrantID(resourceTypeRole.Id, groupGrant.GroupID, groupGrant.PrincipalID),
		}
		gmID, err := res.NewResourceID(resourceTypeUser, groupGrant.PrincipalID)
		if err != nil {
			return nil, "", nil, err
		}
		grant := grant.NewGrant(resource, groupMemberEntitlement, gmID, grant.WithAnnotation(v1Identifier))
		rv = append(rv, grant)
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

func groupProfile(ctx context.Context, group client.Group) map[string]interface{} {
	profile := make(map[string]interface{})
	profile["group_id"] = group.ID
	profile["group_name"] = group.Name
	return profile
}
