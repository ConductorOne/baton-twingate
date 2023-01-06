package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	APIDomain = "%s.twingate.com"
	APIPath   = "api"
	Path      = "graphql"
	rateLimit = 20 // TODO(mstanbCO) Change this back to 60
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

var defaultRoles = []*Role{{Name: "Admin", Id: "admin"}, {Name: "Member", Id: "member"}}

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
	Domain                string
	Client                *http.Client
	ApiKey                string
	rateLimitBucket       int64
	rateLimitRequestCount int64
}

func New(ctx context.Context, apiKey string, domain string) (*ConnectorClient, error) {
	client, err := newClient(ctx)
	if err != nil {
		return nil, err
	}
	return &ConnectorClient{
		Domain: domain,
		Client: client,
		ApiKey: apiKey,
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
	reqUrl := url.URL{Scheme: "https", Host: fmt.Sprintf(APIDomain, c.Domain), Path: strings.Join([]string{APIPath, Path, ""}, "/")}
	q := &Query{
		Query:     rawQuery,
		Variables: variables,
	}
	b, err := json.Marshal(q)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqUrl.String(), bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header["X-API-KEY"] = []string{c.ApiKey}
	req.Header["Content-Type"] = []string{"application/json"}
	resp, err := c.Client.Do(req)
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
	var pagePointer *string = nil
	if pagination != "" {
		pagePointer = &pagination
	}
	resp := &UsersQueryResponse{}
	rateLimitDescription, err := c.query(ctx, allUsersQuery(pagePointer, pageSize), resp, nil)
	if err != nil {
		return nil, fmt.Errorf("twingate-client: error getting role grants for %s: %w", c.Domain, err)
	}
	grants := make([]RoleGrant, 0, len(resp.Data.Users.Edges))
	for _, user := range resp.Data.Users.Edges {
		if roleID == "admin" && user.User.IsAdmin {
			grants = append(grants, RoleGrant{
				PrincipalID: user.User.ID,
				RoleID:      roleID,
			})
		} else if roleID == "member" && !user.User.IsAdmin {
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
