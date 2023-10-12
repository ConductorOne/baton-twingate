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
	roleMemberEntitlement = "member"
)

type roleResourceType struct {
	resourceType *v2.ResourceType
	domain       string
	client       *client.ConnectorClient
}

func (o *roleResourceType) ResourceType(_ context.Context) *v2.ResourceType {
	return o.resourceType
}

func (o *roleResourceType) List(ctx context.Context, _ *v2.ResourceId, pt *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	roles, err := o.client.ListRoles(ctx)
	if err != nil {
		return nil, "", nil, err
	}

	rv := make([]*v2.Resource, 0, len(roles))
	for _, r := range roles {
		annos := &v2.V1Identifier{
			Id: r.Id,
		}
		profile := roleProfile(ctx, r)
		roleTrait := []res.RoleTraitOption{res.WithRoleProfile(profile)}
		roleResource, err := res.NewRoleResource(r.Name, resourceTypeRole, r.Id, roleTrait, res.WithAnnotation(annos))
		if err != nil {
			return nil, "", nil, err
		}
		rv = append(rv, roleResource)
	}
	return rv, "", nil, nil
}

func (o *roleResourceType) Entitlements(ctx context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	var annos annotations.Annotations
	annos.Update(&v2.V1Identifier{
		Id: V1MembershipEntitlementID(resource.Id.Resource),
	})
	member := ent.NewAssignmentEntitlement(resource, roleMemberEntitlement, ent.WithGrantableTo(resourceTypeUser))
	member.Description = fmt.Sprintf("Has the %s role in Twingate", resource.DisplayName)
	member.Annotations = annos
	member.DisplayName = fmt.Sprintf("%s Role Member", resource.DisplayName)
	return []*v2.Entitlement{member}, "", nil, nil
}

func (o *roleResourceType) Grants(ctx context.Context, resource *v2.Resource, pt *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	bag := &pagination.Bag{}
	err := bag.Unmarshal(pt.Token)
	if err != nil {
		return nil, "", nil, err
	}

	if bag.Current() == nil {
		bag.Push(pagination.PageState{
			ResourceTypeID: resource.Id.ResourceType,
			ResourceID:     resource.Id.Resource,
		})
	}

	resp, err := o.client.ListRoleGrants(ctx, resource.Id.Resource, bag.PageToken(), 100)
	if err != nil {
		return nil, "", nil, err
	}
	var rv []*v2.Grant
	for _, roleGrant := range resp.Grants {
		v1Identifier := &v2.V1Identifier{
			Id: V1GrantID(resourceTypeRole.Id, roleGrant.RoleID, roleGrant.PrincipalID),
		}
		rID, err := res.NewResourceID(resourceTypeUser, roleGrant.PrincipalID)
		if err != nil {
			return nil, "", nil, err
		}
		grant := grant.NewGrant(resource, roleMemberEntitlement, rID, grant.WithAnnotation(v1Identifier))
		rv = append(rv, grant)
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

func roleBuilder(client *client.ConnectorClient, domain string) *roleResourceType {
	return &roleResourceType{
		resourceType: resourceTypeRole,
		client:       client,
		domain:       domain,
	}
}

func roleProfile(ctx context.Context, role *client.Role) map[string]interface{} {
	profile := make(map[string]interface{})
	profile["role_id"] = role.Id
	profile["role_name"] = role.Name
	return profile
}
