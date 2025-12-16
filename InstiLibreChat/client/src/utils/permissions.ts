// Permission checking utility for frontend
// Implements the Rules system with operation dependencies

export interface Permission {
  id: string;
  resource: string;
  action: string;
}

export class PermissionManager {
  private permissionMap: Record<string, Record<string, boolean>> = {};

  constructor(permissions: Permission[] = []) {
    // Build permission map: { resource: { action: true } }
    permissions.forEach((perm) => {
      if (!this.permissionMap[perm.resource]) {
        this.permissionMap[perm.resource] = {};
      }
      this.permissionMap[perm.resource][perm.action] = true;
    });
  }

  // Check if user has a specific permission
  // Handles dependencies:
  // - "create" requires "read", "update", and "create"
  // - "delete" requires "read" and "delete"
  // - "update" requires "read" and "update"
  // - "read" is standalone
  hasPermission(resource: string, action: string): boolean {
    const resourcePerms = this.permissionMap[resource] || {};

    // Check if user has the exact permission
    if (resourcePerms[action]) {
      return true;
    }

    // Check dependencies based on action
    switch (action) {
      case 'read':
        // Read is standalone, no dependencies
        return false;

      case 'update':
        // Update requires read + update
        return !!(resourcePerms['read'] && resourcePerms['update']);

      case 'create':
        // Create requires read + update + create
        return !!(
          resourcePerms['read'] && resourcePerms['update'] && resourcePerms['create']
        );

      case 'delete':
        // Delete requires read + delete
        return !!(resourcePerms['read'] && resourcePerms['delete']);

      default:
        // For other actions, just check if user has it
        return resourcePerms[action] || false;
    }
  }

  // Check if user can read (view) a resource
  canRead(resource: string): boolean {
    return this.hasPermission(resource, 'read');
  }

  // Check if user can update a resource
  canUpdate(resource: string): boolean {
    return this.hasPermission(resource, 'update');
  }

  // Check if user can create a resource
  canCreate(resource: string): boolean {
    return this.hasPermission(resource, 'create');
  }

  // Check if user can delete a resource
  canDelete(resource: string): boolean {
    return this.hasPermission(resource, 'delete');
  }

  // Get all permissions for a resource
  getResourcePermissions(resource: string): Record<string, boolean> {
    return this.permissionMap[resource] || {};
  }

  // Check if user has any permission for a resource
  hasAnyPermission(resource: string): boolean {
    const resourcePerms = this.permissionMap[resource];
    return resourcePerms && Object.keys(resourcePerms).length > 0;
  }
}

