package connector

import (
	"context"
	"fmt"
	"strings"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	ent "github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	res "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-twingate/pkg/connector/client"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

func roleResource(role *client.Role) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"role_id":   role.Id,
		"role_name": role.Name,
	}

	roleTraitOptions := []res.RoleTraitOption{}

	resource, err := res.NewRoleResource(
		role.Name,
		resourceTypeRole,
		role.Id,
		roleTraitOptions,
		res.WithResourceProfile(profile),
	)
	if err != nil {
		return nil, err
	}

	return resource, nil
}

func (o *roleResourceType) List(ctx context.Context, _ *v2.ResourceId, _ *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	roles, err := o.client.ListRoles(ctx)
	if err != nil {
		return nil, "", nil, err
	}

	rv := make([]*v2.Resource, 0, len(roles))
	for _, r := range roles {
		rr, err := roleResource(r)
		if err != nil {
			return nil, "", nil, err
		}

		rv = append(rv, rr)
	}
	return rv, "", nil, nil
}

func (o *roleResourceType) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	var rv []*v2.Entitlement

	assignmentOptions := []ent.EntitlementOption{
		ent.WithGrantableTo(resourceTypeUser),
		ent.WithDisplayName(fmt.Sprintf("%s Role Member", resource.DisplayName)),
		ent.WithDescription(fmt.Sprintf("Has the %s role in Twingate", resource.DisplayName)),
	}

	rv = append(rv, ent.NewAssignmentEntitlement(resource, roleMemberEntitlement, assignmentOptions...))

	return rv, "", nil, nil
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

	resp, err := o.client.ListRoleGrants(ctx, resource.Id.Resource, bag.PageToken(), ResourcesPageSize)
	if err != nil {
		return nil, "", nil, err
	}

	var rv []*v2.Grant
	for _, roleGrant := range resp.Grants {
		rv = append(rv, grant.NewGrant(
			resource,
			roleMemberEntitlement,
			&v2.ResourceId{
				ResourceType: resourceTypeUser.Id,
				Resource:     roleGrant.PrincipalID,
			},
		))
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

// Grant sets a user's Twingate role via userRoleUpdate. Twingate roles are set-style — each
// user holds exactly one role, so granting any role replaces the current one. A pre-flight
// read is required because the API does not signal "already has this role".
func (o *roleResourceType) Grant(ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement) (annotations.Annotations, error) {
	outputAnnotations := annotations.Annotations{}

	if principal.Id.ResourceType != resourceTypeUser.Id {
		return nil, status.Errorf(codes.InvalidArgument, "twingate: principal must be a user, got %s", principal.Id.ResourceType)
	}

	roleID := entitlement.Resource.Id.Resource
	userID := principal.Id.Resource

	targetEnum, ok := client.RoleEnumByID(roleID)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "twingate: unknown role: %s", roleID)
	}

	user, rld, err := o.client.GetUser(ctx, userID)
	if rld != nil {
		outputAnnotations.WithRateLimiting(rld)
	}
	if err != nil {
		return outputAnnotations, err
	}
	if user == nil {
		return outputAnnotations, status.Errorf(codes.NotFound, "twingate: user %s not found", userID)
	}
	if strings.EqualFold(user.Role, targetEnum) {
		outputAnnotations.Append(&v2.GrantAlreadyExists{})
		return outputAnnotations, nil
	}

	rld2, err := o.client.UpdateUserRole(ctx, userID, targetEnum)
	if rld2 != nil {
		outputAnnotations.WithRateLimiting(rld2)
	}
	if err != nil {
		return outputAnnotations, err
	}
	return outputAnnotations, nil
}

// Revoke demotes a user to MEMBER (Twingate's only "no privileged role" state). Revoking the
// Member role itself is invalid — Twingate users always have a role.
func (o *roleResourceType) Revoke(ctx context.Context, gr *v2.Grant) (annotations.Annotations, error) {
	outputAnnotations := annotations.Annotations{}

	roleID := gr.Entitlement.Resource.Id.Resource
	userID := gr.Principal.Id.Resource

	if roleID == "member" {
		return outputAnnotations, status.Error(codes.InvalidArgument,
			"twingate: cannot revoke the Member role — Twingate users always have a role; grant a different role to change it")
	}

	grantedEnum, ok := client.RoleEnumByID(roleID)
	if !ok {
		return outputAnnotations, status.Errorf(codes.InvalidArgument, "twingate: unknown role: %s", roleID)
	}

	user, rld, err := o.client.GetUser(ctx, userID)
	if rld != nil {
		outputAnnotations.WithRateLimiting(rld)
	}
	if err != nil {
		return outputAnnotations, err
	}
	if user == nil {
		return outputAnnotations, status.Errorf(codes.NotFound, "twingate: user %s not found", userID)
	}
	// If the user no longer holds the role we're revoking, the grant is already gone.
	if !strings.EqualFold(user.Role, grantedEnum) {
		outputAnnotations.Append(&v2.GrantAlreadyRevoked{})
		return outputAnnotations, nil
	}

	rld2, err := o.client.UpdateUserRole(ctx, userID, "MEMBER")
	if rld2 != nil {
		outputAnnotations.WithRateLimiting(rld2)
	}
	if err != nil {
		return outputAnnotations, err
	}
	return outputAnnotations, nil
}

func roleBuilder(client *client.ConnectorClient, domain string) *roleResourceType {
	return &roleResourceType{
		resourceType: resourceTypeRole,
		client:       client,
		domain:       domain,
	}
}
