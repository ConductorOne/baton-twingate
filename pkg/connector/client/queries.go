package client

import "fmt"

const (
	getAllUsersQuery = `query getUsers{
  users(after: "%v", first: %d) {
    edges {
      node {
        id
        firstName
        lastName
        email
        createdAt
        updatedAt
        isAdmin
        state
      }
  }
  pageInfo {
    endCursor
    hasNextPage
  }
  }
}`

	getGroupsQuery = `query getGroups{
  groups(after: "%v", first: %d) {
    edges {
      node {
        id
        name
        isActive
      }
    }
    pageInfo {
      endCursor
      hasNextPage
    }
  }
}`

	getGroupMembersQuery = `query getGroup($groupID: ID!){
		group(id:$groupID) {
			  id
			  createdAt
			  updatedAt
			  users {
				  edges{
					  node{
						  id
						  email
						  firstName
						  lastName
					  }
				  }
			  }
			}
		}`
	addGroupMemberQuery = `mutation{
		groupUpdate(id: "%s", addedUserIds: ["%s"]) {
		  ok
		  error
		}
	}`

	removeGroupMemberQuery = `mutation{
		groupUpdate(id: "%s", removedUserIds: ["%s"]) {
		  ok
		  error
		}
	}`
)

func allUsersQuery(pg *string, pageSize uint32) string {
	if pg == nil {
		return fmt.Sprintf(getAllUsersQuery, pg, pageSize)
	} else {
		return fmt.Sprintf(getAllUsersQuery, *pg, pageSize)
	}
}

func groupsQuery(pg *string, pageSize uint32) string {
	if pg == nil {
		return fmt.Sprintf(getGroupsQuery, pg, pageSize)
	} else {
		return fmt.Sprintf(getGroupsQuery, *pg, pageSize)
	}
}

func addGroupMemberQueryFormat(groupID string, userID string) string {
	return fmt.Sprintf(addGroupMemberQuery, groupID, userID)
}

func removeGroupMemberQueryFormat(groupID string, userID string) string {
	return fmt.Sprintf(removeGroupMemberQuery, groupID, userID)
}
