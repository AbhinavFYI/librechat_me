package utils

// PermissionDependencies defines which permissions are required for each action
// Based on the rules:
// - Create requires: Read + Update + Create
// - Delete requires: Read + Delete
// - Update requires: Read + Update
// - Read: standalone
var PermissionDependencies = map[string][]string{
	"create": {"read", "update", "create"},
	"update": {"read", "update"},
	"delete": {"read", "delete"},
	"read":   {"read"},
}

// GetRequiredPermissions returns all permissions needed for a given action
// This includes the action itself and its dependencies
func GetRequiredPermissions(action string) []string {
	if deps, exists := PermissionDependencies[action]; exists {
		return deps
	}
	// If action not found, return just the action itself
	return []string{action}
}

// CheckPermissionWithDependencies checks if a user has a permission,
// considering that having a higher-level permission (like "create")
// automatically grants lower-level permissions (like "read", "update")
func CheckPermissionWithDependencies(userPermissions map[string]map[string]bool, resource, action string) bool {
	// Get all required permissions for this action
	required := GetRequiredPermissions(action)

	// Check if user has all required permissions for this resource
	for _, reqAction := range required {
		if resourcePerms, exists := userPermissions[resource]; exists {
			if hasPerm, exists := resourcePerms[reqAction]; exists && hasPerm {
				continue // User has this required permission
			}
		}
		// User doesn't have one of the required permissions
		return false
	}

	return true
}

// BuildPermissionMap converts a list of permissions into a map for quick lookup
// Format: map[resource][action] = true
func BuildPermissionMap(permissions []struct {
	Resource string
	Action   string
}) map[string]map[string]bool {
	permMap := make(map[string]map[string]bool)

	for _, perm := range permissions {
		if permMap[perm.Resource] == nil {
			permMap[perm.Resource] = make(map[string]bool)
		}
		permMap[perm.Resource][perm.Action] = true
	}

	return permMap
}
