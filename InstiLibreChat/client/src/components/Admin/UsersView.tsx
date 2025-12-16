import { useState } from 'react';
import { Button } from '@librechat/client';
import { saasApi } from '~/services/saasApi';
import { PermissionManager } from '~/utils/permissions';
import AddUserModal from './Modals/AddUserModal';
import EditUserModal from './Modals/EditUserModal';
import AssignRoleModal from './Modals/AssignRoleModal';
import PermissionsModal from './Modals/PermissionsModal';

interface UsersViewProps {
  users: any[];
  isSuperAdmin: boolean;
  userOrgId: string | null;
  organizations: any[];
  permissionManager: PermissionManager | null;
  selectedOrgFilter: string;
  onOrgFilterChange: (orgId: string) => void;
  onRefresh: (orgFilterId?: string | null) => void;
}

export default function UsersView({
  users,
  isSuperAdmin,
  userOrgId,
  organizations,
  permissionManager,
  selectedOrgFilter,
  onOrgFilterChange,
  onRefresh,
}: UsersViewProps) {
  const [showAddUserModal, setShowAddUserModal] = useState(false);
  const [showEditUserModal, setShowEditUserModal] = useState(false);
  const [selectedUser, setSelectedUser] = useState<any>(null);
  const [showPermissionsModal, setShowPermissionsModal] = useState(false);
  const [userPermissions, setUserPermissions] = useState<any[]>([]);
  const [showAssignRoleModal, setShowAssignRoleModal] = useState(false);

  const handleOrgFilterChange = (orgId: string) => {
    onOrgFilterChange(orgId);
    const filterValue = orgId && orgId !== '' ? orgId : null;
    onRefresh(filterValue);
  };

  const fetchUserPermissions = async (userId: string) => {
    try {
      const data = await saasApi.getUserPermissions(userId);
      setUserPermissions(Array.isArray(data) ? data : []);
    } catch (error) {
      console.error('Error fetching user permissions:', error);
    }
  };

  const handleViewPermissions = async (user: any) => {
    setSelectedUser(user);
    await fetchUserPermissions(user.id);
    setShowPermissionsModal(true);
  };

  const handleDeleteUser = async (user: any) => {
    if (!confirm(`Are you sure you want to delete user ${user.email}?`)) {
      return;
    }

    try {
      await saasApi.deleteUser(user.id);
      onRefresh(selectedOrgFilter || null);
    } catch (error: any) {
      alert(error.message || 'Failed to delete user');
    }
  };

  const handleStatusChange = async (user: any, newStatus: string) => {
    if (user.status === newStatus) {
      return; // No change needed
    }

    try {
      await saasApi.updateUser(user.id, { status: newStatus });
      onRefresh(selectedOrgFilter || null);
    } catch (error: any) {
      alert(error.message || 'Failed to update user status');
    }
  };

  return (
    <div>
      <div className="flex justify-between items-center mb-4">
        <h2 className="text-xl font-semibold text-gray-900 dark:text-gray-100">Users</h2>
        <div className="flex items-center gap-4">
          {isSuperAdmin && organizations && organizations.length > 0 && (
            <div className="flex items-center gap-2">
              <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
                Filter by Organization:
              </label>
              <select
                value={selectedOrgFilter}
                onChange={(e) => handleOrgFilterChange(e.target.value)}
                className="px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100"
              >
                <option value="">All Organizations</option>
                {organizations.map((org) => (
                  <option key={org.id} value={org.id}>
                    {org.name}
                  </option>
                ))}
              </select>
            </div>
          )}
          {permissionManager && permissionManager.canCreate('users') && (
            <Button
              onClick={() => setShowAddUserModal(true)}
              className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
            >
              + Add User
            </Button>
          )}
        </div>
      </div>

      <div className="overflow-x-auto">
        <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
          <thead className="bg-gray-50 dark:bg-gray-800">
            <tr>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Name
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Email
              </th>
              {isSuperAdmin && (
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Organization
                </th>
              )}
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Status
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Roles
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Permissions
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Actions
              </th>
            </tr>
          </thead>
          <tbody className="bg-white dark:bg-gray-850 divide-y divide-gray-200 dark:divide-gray-700">
            {users.length === 0 ? (
              <tr>
                <td
                  colSpan={isSuperAdmin ? 7 : 6}
                  className="px-6 py-4 text-center text-gray-500 dark:text-gray-400"
                >
                  No users found
                </td>
              </tr>
            ) : (
              users.map((user) => (
                <tr key={user.id} className="hover:bg-gray-50 dark:hover:bg-gray-800">
                  <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900 dark:text-gray-100">
                    {user.full_name || user.email}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                    {user.email}
                  </td>
                  {isSuperAdmin && (
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                      {user.is_super_admin
                        ? 'Super Admin'
                        : user.org_role === 'admin'
                          ? 'Org Admin'
                          : user.org_role === 'viewer'
                            ? 'Org Viewer'
                            : 'Org User'}
                    </td>
                  )}
                  <td className="px-6 py-4 whitespace-nowrap">
                    {permissionManager && permissionManager.canUpdate('users') ? (
                      <select
                        value={user.status || 'pending'}
                        onChange={(e) => handleStatusChange(user, e.target.value)}
                        className={`px-2 py-1 text-xs rounded-md border-0 focus:ring-2 focus:ring-blue-500 cursor-pointer font-medium transition-colors ${
                          user.status === 'active'
                            ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300 hover:bg-green-200 dark:hover:bg-green-800'
                            : user.status === 'suspended'
                              ? 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-300 hover:bg-red-200 dark:hover:bg-red-800'
                              : user.status === 'pending'
                                ? 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-300 hover:bg-yellow-200 dark:hover:bg-yellow-800'
                                : 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'
                        }`}
                        onClick={(e) => e.stopPropagation()}
                        title="Click to change status"
                      >
                        <option value="active">Active</option>
                        <option value="pending">Pending</option>
                        <option value="suspended">Suspended</option>
                      </select>
                    ) : (
                      <span
                        className={`px-2 py-1 text-xs rounded-full font-medium ${
                          user.status === 'active'
                            ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300'
                            : user.status === 'suspended'
                              ? 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-300'
                              : user.status === 'pending'
                                ? 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-300'
                                : 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300'
                        }`}
                      >
                        {user.status || 'pending'}
                      </span>
                    )}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                    {user.is_super_admin
                      ? 'Super Admin'
                      : user.roles?.map((r: any) => r.name).join(', ') || 'No roles'}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm">
                    <button
                      onClick={() => handleViewPermissions(user)}
                      className="text-blue-600 hover:text-blue-900 dark:text-blue-400 dark:hover:text-blue-300"
                    >
                      View
                    </button>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm">
                    <div className="flex gap-2">
                      {permissionManager && permissionManager.canUpdate('users') && (
                        <>
                          <button
                            onClick={() => {
                              setSelectedUser(user);
                              setShowEditUserModal(true);
                            }}
                            className="text-blue-600 hover:text-blue-900 dark:text-blue-400 dark:hover:text-blue-300"
                          >
                            Edit
                          </button>
                          <button
                            onClick={() => {
                              setSelectedUser(user);
                              setShowAssignRoleModal(true);
                            }}
                            className="text-green-600 hover:text-green-900 dark:text-green-400 dark:hover:text-green-300"
                          >
                            Assign Role
                          </button>
                        </>
                      )}
                      {permissionManager && permissionManager.canDelete('users') && (
                        <button
                          onClick={() => handleDeleteUser(user)}
                          className="text-red-600 hover:text-red-900 dark:text-red-400 dark:hover:text-red-300"
                        >
                          Delete
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {showAddUserModal && (
        <AddUserModal
          isSuperAdmin={isSuperAdmin}
          userOrgId={userOrgId}
          onClose={() => setShowAddUserModal(false)}
          onSuccess={() => {
            setShowAddUserModal(false);
            onRefresh();
          }}
        />
      )}

      {showEditUserModal && selectedUser && (
        <EditUserModal
          user={selectedUser}
          onClose={() => {
            setShowEditUserModal(false);
            setSelectedUser(null);
          }}
          onSuccess={() => {
            setShowEditUserModal(false);
            setSelectedUser(null);
            onRefresh();
          }}
        />
      )}

      {showAssignRoleModal && selectedUser && (
        <AssignRoleModal
          user={selectedUser}
          isSuperAdmin={isSuperAdmin}
          userOrgId={userOrgId}
          onClose={() => {
            setShowAssignRoleModal(false);
            setSelectedUser(null);
          }}
          onSuccess={() => {
            setShowAssignRoleModal(false);
            setSelectedUser(null);
            onRefresh();
          }}
        />
      )}

      {showPermissionsModal && selectedUser && (
        <PermissionsModal
          user={selectedUser}
          permissions={userPermissions}
          onClose={() => {
            setShowPermissionsModal(false);
            setSelectedUser(null);
          }}
        />
      )}
    </div>
  );
}
