package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	DefaultBaseURL = "https://%s.twingate.com/api/graphql/"
	rateLimit      = 20 // TODO(mstanbCO) Change this back to 60
)

type Role struct {
	Name string
	Id   string
}

type Query struct {
	Query     string            `json:"query"`
	Variables map[string]string `json:"variables,omitempty"`
}

type UsersQueryResponse struct {
	Data struct {
		Users struct {
			Edges []struct {
				User *User `json:"node"`
			} `json:"edges"`
			Pagination PageInfo `json:"pageInfo"`
		} `json:"users"`
	} `json:"data"`
}

type GrantAndRevokeGroupResponse struct {
	Data struct {
		GroupUpdate struct {
			Ok    bool    `json:"ok"`
			Error *string `json:"error"`
		} `json:"groupUpdate"`
	} `json:"data"`
}

type GroupsQueryResponse struct {
	Data struct {
		Groups struct {
			Edges []struct {
				Group *Group `json:"node"`
			} `json:"edges"`
			Pagination PageInfo `json:"pageInfo"`
		} `json:"groups"`
	} `json:"data"`
}

type RolesQueryResponse struct {
	Data struct {
		Roles []*Role `json:"roles"`
	} `json:"data"`
}

type GroupMembersQueryResponse struct {
	Data struct {
		Group struct {
			Id    string `json:"id"`
			Name  string `json:"name"`
			Users struct {
				Edges []struct {
					User *User `json:"node"`
				} `json:"edges"`
			} `json:"users"`
		} `json:"group"`
	} `json:"data"`
}

type RoleGrantsQueryResponse struct {
	Data struct {
		Users []struct {
			Id       string `json:"id"`
			Email    string `json:"email"`
			Fullname string `json:"fullname"`
			Roles    []struct {
				Id int `json:"id"`
			} `json:"roles"`
		} `json:"users"`
	} `json:"data"`
}

type User struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	IsAdmin   bool   `json:"isAdmin"`
	// Role is the UserRole enum value: ADMIN, DEVOPS, SUPPORT, ACCESS_REVIEWER, or MEMBER.
	// Bucket role grants on this field, NOT isAdmin (isAdmin is true for all but MEMBER).
	Role string `json:"role"`
}

// GraphQLError is a top-level GraphQL error entry (validation errors, e.g. a bad enum).
// These land in the response's top-level `errors[]`, NOT in data.<op>.error.
type GraphQLError struct {
	Message string `json:"message"`
}

type UserRoleUpdateResponse struct {
	Data struct {
		UserRoleUpdate struct {
			Ok    bool    `json:"ok"`
			Error *string `json:"error"`
		} `json:"userRoleUpdate"`
	} `json:"data"`
	Errors []GraphQLError `json:"errors"`
}

type GetUserResponse struct {
	Data struct {
		User *User `json:"user"`
	} `json:"data"`
	Errors []GraphQLError `json:"errors"`
}

type PageInfo struct {
	EndCursor   string `json:"endCursor"`
	HasNextPage bool   `json:"hasNextPage"`
}

type Group struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	IsActive bool   `json:"isActive,omitempty"`
}

// defaultRoles models all 5 Twingate UserRole enum values. The enum is ADMIN, DEVOPS,
// SUPPORT, ACCESS_REVIEWER, MEMBER — modeling only Admin/Member mis-buckets the middle
// three (which all report isAdmin=true) as Admin.
var defaultRoles = []*Role{
	{Id: "admin", Name: "Admin"},
	{Id: "devops", Name: "DevOps"},
	{Id: "support", Name: "Support"},
	{Id: "access_reviewer", Name: "Access Reviewer"},
	{Id: "member", Name: "Member"},
}

// roleEnumByID maps an internal role ID to the on-wire UserRole enum. An explicit table
// decouples role IDs from enum names (no strings.ToUpper) and fails loudly on unknown roles.
var roleEnumByID = map[string]string{
	"admin":           "ADMIN",
	"devops":          "DEVOPS",
	"support":         "SUPPORT",
	"access_reviewer": "ACCESS_REVIEWER",
	"member":          "MEMBER",
}

// RoleEnumByID returns the UserRole enum for a role ID, and whether the role is known.
func RoleEnumByID(roleID string) (string, bool) {
	enum, ok := roleEnumByID[roleID]
	return enum, ok
}

type GroupGrant struct {
	GroupID     string
	PrincipalID string
}

type RoleGrant struct {
	RoleID      string
	PrincipalID string
}

type InfoResponse struct {
	User                 *User
	RateLimitDescription *v2.RateLimitDescription
}

type UsersResponse struct {
	Users                []*User
	RateLimitDescription *v2.RateLimitDescription
	Pagination           string
}

type RoleGrantsResponse struct {
	Grants               []RoleGrant
	RateLimitDescription *v2.RateLimitDescription
	Pagination           string
}

type GroupGrantsResponse struct {
	Grants               []GroupGrant
	RateLimitDescription *v2.RateLimitDescription
	Pagination           string
}

type GrantEntitlementResponse struct {
	RateLimitDescription *v2.RateLimitDescription
}

type RevokeEntitlementResponse struct {
	RateLimitDescription *v2.RateLimitDescription
}

type GroupResourcesResponse struct {
	Groups               []Group
	RateLimitDescription *v2.RateLimitDescription
	Pagination           string
}

type Client interface {
	ListUsers(ctx context.Context, pagination string) (*UsersResponse, error)
	ListRoles(ctx context.Context, pagination string) ([]*Role, error)
	ListGroups(ctx context.Context, pagination string) (*GroupResourcesResponse, error)
	ListRoleGrants(ctx context.Context, roleID string, pagination string) (*RoleGrantsResponse, error)
	ListGroupGrants(ctx context.Context, groupID string, pagination string) (*GroupGrantsResponse, error)
}

type ConnectorClient struct {
	Domain  string
	baseURL string
	Client  *http.Client
	//nolint:gosec,nolintlint // G117: legitimate field name, not a credential
	ApiKey                string
	rateLimitBucket       int64
	rateLimitRequestCount int64
}

func New(ctx context.Context, apiKey string, domain string, baseURL string) (*ConnectorClient, error) {
	client, err := newClient(ctx)
	if err != nil {
		return nil, err
	}
	effectiveBaseURL := baseURL
	if effectiveBaseURL == "" {
		effectiveBaseURL = fmt.Sprintf(DefaultBaseURL, domain)
	}
	return &ConnectorClient{
		Domain:  domain,
		baseURL: effectiveBaseURL,
		Client:  client,
		ApiKey:  apiKey,
	}, nil
}

func newClient(ctx context.Context) (*http.Client, error) {
	httpClient, err := uhttp.NewClient(ctx, uhttp.WithLogger(true, nil))
	if err != nil {
		return nil, err
	}
	return httpClient, nil
}

func (c *ConnectorClient) query(ctx context.Context, rawQuery string, res interface{}, variables map[string]string) (*v2.RateLimitDescription, error) {
	q := &Query{
		Query:     rawQuery,
		Variables: variables,
	}
	b, err := json.Marshal(q)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header["X-API-KEY"] = []string{c.ApiKey}
	req.Header["Content-Type"] = []string{"application/json"}
	resp, err := c.Client.Do(req) //nolint:gosec,nolintlint // G704: URL constructed from trusted config
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	rawResp, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusTooManyRequests {
		return nil, fmt.Errorf("twingate-client: GraphQL HTTP request failed %d %s", resp.StatusCode, string(rawResp))
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return c.getRateLimitDescription(ctx, true), nil
	}
	if err := json.Unmarshal(rawResp, res); err != nil {
		return nil, err
	}

	return c.getRateLimitDescription(ctx, false), nil
}

func (c *ConnectorClient) ListUsers(ctx context.Context, pagination string, pageSize uint32) (*UsersResponse, error) {
	var pagePointer *string = nil
	if pagination != "" {
		pagePointer = &pagination
	}
	resp := &UsersQueryResponse{}
	rateLimitDescription, err := c.query(ctx, allUsersQuery(pagePointer, pageSize), resp, nil)
	if err != nil {
		return nil, fmt.Errorf("twingate-client: error getting all users %w", err)
	}
	pg := ""
	if resp.Data.Users.Pagination.HasNextPage {
		pg = resp.Data.Users.Pagination.EndCursor
	}
	users := make([]*User, 0, len(resp.Data.Users.Edges))
	for _, user := range resp.Data.Users.Edges {
		users = append(users, user.User)
	}

	rv := &UsersResponse{
		Users:                users,
		RateLimitDescription: rateLimitDescription,
		Pagination:           pg,
	}
	return rv, nil
}

func (c *ConnectorClient) ListGroups(ctx context.Context, pagination string, pageSize uint32) (*GroupResourcesResponse, error) {
	var pagePointer *string = nil
	if pagination != "" {
		pagePointer = &pagination
	}
	resp := &GroupsQueryResponse{}
	rateLimitDescription, err := c.query(ctx, groupsQuery(pagePointer, pageSize), resp, nil)
	if err != nil {
		return nil, fmt.Errorf("twingate-client: error getting groups %w", err)
	}
	groups := make([]Group, 0, len(resp.Data.Groups.Edges))
	for _, group := range resp.Data.Groups.Edges {
		groups = append(groups, *group.Group)
	}
	pg := ""
	if resp.Data.Groups.Pagination.HasNextPage {
		pg = resp.Data.Groups.Pagination.EndCursor
	}
	rv := &GroupResourcesResponse{
		Groups:               groups,
		RateLimitDescription: rateLimitDescription,
		Pagination:           pg,
	}
	return rv, nil
}

func (c *ConnectorClient) ListRoles(ctx context.Context) ([]*Role, error) {
	roles := make([]*Role, 0, len(defaultRoles))
	for _, role := range defaultRoles {
		role := role
		roles = append(roles, role)
	}
	return roles, nil
}

func (c *ConnectorClient) ListGroupGrants(ctx context.Context, groupID string) (*GroupGrantsResponse, error) {
	resp := &GroupMembersQueryResponse{}
	variable := map[string]string{"groupID": groupID}
	rateLimitDescription, err := c.query(ctx, getGroupMembersQuery, resp, variable)
	if err != nil {
		return nil, fmt.Errorf("twingate-client: error getting group members for %s: %w", c.Domain, err)
	}
	grants := make([]GroupGrant, 0, len(resp.Data.Group.Users.Edges))
	for _, user := range resp.Data.Group.Users.Edges {
		grants = append(grants, GroupGrant{
			PrincipalID: user.User.ID,
			GroupID:     groupID,
		})
	}
	rv := &GroupGrantsResponse{
		Grants:               grants,
		RateLimitDescription: rateLimitDescription,
	}
	return rv, nil
}

func (c *ConnectorClient) GrantGroupMembership(ctx context.Context, groupID string, userID string) (*GrantEntitlementResponse, error) {
	resp := &GrantAndRevokeGroupResponse{}
	rateLimitDescription, err := c.query(ctx, addGroupMemberQueryFormat(groupID, userID), resp, nil)
	if err != nil {
		return nil, fmt.Errorf("twingate-client: error granting group member for %s: %w", c.Domain, err)
	}

	if !resp.Data.GroupUpdate.Ok {
		if resp.Data.GroupUpdate.Error != nil {
			return nil, fmt.Errorf("twingate: api error: '%s'", *resp.Data.GroupUpdate.Error)
		}
		return nil, fmt.Errorf("twingate: api error: unable to get group membership for group %s, and user %s", groupID, userID)
	}

	rv := &GrantEntitlementResponse{
		RateLimitDescription: rateLimitDescription,
	}
	return rv, nil
}

// GetUser fetches a single user's current state for an idempotency pre-flight read.
// Returns a nil User (no error) when the user does not exist (data.user is null).
func (c *ConnectorClient) GetUser(ctx context.Context, userID string) (*User, *v2.RateLimitDescription, error) {
	resp := &GetUserResponse{}
	rld, err := c.query(ctx, getUserQueryFormat(userID), resp, nil)
	if err != nil {
		return nil, rld, fmt.Errorf("twingate-client: error getting user %s for %s: %w", userID, c.Domain, err)
	}
	if len(resp.Errors) > 0 {
		return nil, rld, fmt.Errorf("twingate: graphql error: %s", resp.Errors[0].Message)
	}
	return resp.Data.User, rld, nil
}

// UpdateUserRole sets a user's role via the userRoleUpdate mutation. role is the on-wire
// UserRole enum (e.g. "ADMIN", "MEMBER"). It surfaces top-level GraphQL errors (e.g. a bad
// enum), which land in errors[] and do NOT populate data.userRoleUpdate.
func (c *ConnectorClient) UpdateUserRole(ctx context.Context, userID string, role string) (*v2.RateLimitDescription, error) {
	resp := &UserRoleUpdateResponse{}
	rld, err := c.query(ctx, userRoleUpdateQueryFormat(userID, role), resp, nil)
	if err != nil {
		return rld, fmt.Errorf("twingate-client: error updating role to %s for user %s on %s: %w", role, userID, c.Domain, err)
	}
	if len(resp.Errors) > 0 {
		return rld, fmt.Errorf("twingate: graphql error: %s", resp.Errors[0].Message)
	}
	if !resp.Data.UserRoleUpdate.Ok {
		if resp.Data.UserRoleUpdate.Error != nil {
			return rld, fmt.Errorf("twingate: api error: '%s'", *resp.Data.UserRoleUpdate.Error)
		}
		return rld, fmt.Errorf("twingate: unable to update role to %s for user %s", role, userID)
	}
	return rld, nil
}

func (c *ConnectorClient) RevokeGroupMembership(ctx context.Context, groupID string, userID string) (*RevokeEntitlementResponse, error) {
	resp := &GrantAndRevokeGroupResponse{}
	rateLimitDescription, err := c.query(ctx, removeGroupMemberQueryFormat(groupID, userID), resp, nil)
	if err != nil {
		return nil, fmt.Errorf("twingate-client: error revoking group member for %s: %w", c.Domain, err)
	}
	if !resp.Data.GroupUpdate.Ok {
		if resp.Data.GroupUpdate.Error != nil {
			return nil, fmt.Errorf("twingate: api error: '%s'", *resp.Data.GroupUpdate.Error)
		}
		return nil, fmt.Errorf("twingate: api error: unable to revoke group membership for group %s, and user %s", groupID, userID)
	}

	rv := &RevokeEntitlementResponse{
		RateLimitDescription: rateLimitDescription,
	}
	return rv, nil
}

func (c *ConnectorClient) ListRoleGrants(ctx context.Context, roleID string, pagination string, pageSize uint32) (*RoleGrantsResponse, error) {
	// Fail loud on an unknown role ID — guards against drift where a role is added to
	// defaultRoles but not roleEnumByID, which would otherwise silently drop its grants.
	targetEnum, ok := roleEnumByID[roleID]
	if !ok {
		return nil, fmt.Errorf("twingate-client: unknown role ID %q", roleID)
	}
	var pagePointer *string = nil
	if pagination != "" {
		pagePointer = &pagination
	}
	resp := &UsersQueryResponse{}
	rateLimitDescription, err := c.query(ctx, allUsersQuery(pagePointer, pageSize), resp, nil)
	if err != nil {
		return nil, fmt.Errorf("twingate-client: error getting role grants for %s: %w", c.Domain, err)
	}
	// Bucket on user.role (the UserRole enum), NOT isAdmin — isAdmin is true for ADMIN,
	// DEVOPS, SUPPORT, AND ACCESS_REVIEWER, so it cannot distinguish them.
	grants := make([]RoleGrant, 0, len(resp.Data.Users.Edges))
	for _, user := range resp.Data.Users.Edges {
		if strings.EqualFold(user.User.Role, targetEnum) {
			grants = append(grants, RoleGrant{
				PrincipalID: user.User.ID,
				RoleID:      roleID,
			})
		}
	}
	pg := ""
	if resp.Data.Users.Pagination.HasNextPage {
		pg = resp.Data.Users.Pagination.EndCursor
	}
	rv := &RoleGrantsResponse{
		Grants:               grants,
		RateLimitDescription: rateLimitDescription,
		Pagination:           pg,
	}
	return rv, nil
}

// TODO(mstanbCO): Fix the rate limiting logic when it becomes an issue
func (c *ConnectorClient) getRateLimitDescription(ctx context.Context, isOverLimit bool) *v2.RateLimitDescription {
	var status v2.RateLimitDescription_Status
	var remaining int64
	now := time.Now().Unix()
	// Round down to the nearest whole minute
	currentBucket := now - (now % 60)
	if isOverLimit {
		status = v2.RateLimitDescription_STATUS_OVERLIMIT
		remaining = 0
	} else {
		status = v2.RateLimitDescription_STATUS_OK
		if currentBucket > c.rateLimitBucket {
			c.rateLimitBucket = currentBucket
			c.rateLimitRequestCount = 0
		}
		c.rateLimitRequestCount++
		remaining = rateLimit - c.rateLimitRequestCount
	}
	resetAt := time.Unix(c.rateLimitBucket, 0).Add(time.Minute * 2) // TODO(mstanbCO): Change this back to one minute
	rateLimitDescription := &v2.RateLimitDescription{Limit: rateLimit, ResetAt: timestamppb.New(resetAt), Remaining: remaining, Status: status}
	return rateLimitDescription
}
