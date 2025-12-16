import { useState } from 'react';
import { Button } from '@librechat/client';
import { saasApi } from '~/services/saasApi';
import { PermissionManager } from '~/utils/permissions';
import CreateRoleModal from './Modals/CreateRoleModal';
import EditRoleModal from './Modals/EditRoleModal';
import RolePermissionsModal from './Modals/RolePermissionsModal';

interface RolesViewProps {
  roles: any[];
  isSuperAdmin: boolean;
  userOrgId: string | null;
  permissionManager: PermissionManager | null;
  onRefresh: () => void;
}

export default function RolesView({
  roles,
  isSuperAdmin,
  userOrgId,
  permissionManager,
  onRefresh,
}: RolesViewProps) {
  const [showCreateRoleModal, setShowCreateRoleModal] = useState(false);
  const [showEditRoleModal, setShowEditRoleModal] = useState(false);
  const [showRolePermissionsModal, setShowRolePermissionsModal] = useState(false);
  const [selectedRole, setSelectedRole] = useState<any>(null);

  const handleDeleteRole = async (role: any) => {
    if (!confirm(`Are you sure you want to delete role "${role.name}"? This action cannot be undone.`)) {
      return;
    }

    try {
      await saasApi.deleteRole(role.id);
      onRefresh();
    } catch (error: any) {
      alert(error.message || 'Failed to delete role');
    }
  };

  return (
    <div>
      <div className="flex justify-between items-center mb-4">
        <h2 className="text-xl font-semibold text-gray-900 dark:text-gray-100">Roles</h2>
        {permissionManager && permissionManager.canCreate('roles') && (
          <Button
            onClick={() => setShowCreateRoleModal(true)}
            className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
          >
            + Create Role
          </Button>
        )}
      </div>

      <div className="overflow-x-auto">
        <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
          <thead className="bg-gray-50 dark:bg-gray-800">
            <tr>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Name
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Type
              </th>
              {isSuperAdmin && (
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Organization
                </th>
              )}
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Description
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Actions
              </th>
            </tr>
          </thead>
          <tbody className="bg-white dark:bg-gray-850 divide-y divide-gray-200 dark:divide-gray-700">
            {roles.length === 0 ? (
              <tr>
                <td
                  colSpan={isSuperAdmin ? 5 : 4}
                  className="px-6 py-4 text-center text-gray-500 dark:text-gray-400"
                >
                  No roles found
                </td>
              </tr>
            ) : (
              roles.map((role) => (
                <tr key={role.id} className="hover:bg-gray-50 dark:hover:bg-gray-800">
                  <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900 dark:text-gray-100">
                    {role.name}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <span
                      className={`px-2 py-1 text-xs rounded-full ${
                        role.type === 'system'
                          ? 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-300'
                          : 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300'
                      }`}
                    >
                      {role.type}
                    </span>
                  </td>
                  {isSuperAdmin && (
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                      {role.org_id ? 'Org Role' : 'System'}
                    </td>
                  )}
                  <td className="px-6 py-4 text-sm text-gray-500 dark:text-gray-400">
                    {role.description || '-'}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm">
                    <div className="flex gap-2">
                      {permissionManager && permissionManager.canUpdate('roles') && (
                        <>
                          <button
                            onClick={() => {
                              setSelectedRole(role);
                              setShowEditRoleModal(true);
                            }}
                            className="text-blue-600 hover:text-blue-900 dark:text-blue-400 dark:hover:text-blue-300"
                          >
                            Edit
                          </button>
                          <button
                            onClick={() => {
                              setSelectedRole(role);
                              setShowRolePermissionsModal(true);
                            }}
                            className="text-green-600 hover:text-green-900 dark:text-green-400 dark:hover:text-green-300"
                          >
                            Permissions
                          </button>
                        </>
                      )}
                      {permissionManager && permissionManager.canDelete('roles') && (
                        <button
                          onClick={() => handleDeleteRole(role)}
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

      {showCreateRoleModal && (
        <CreateRoleModal
          isSuperAdmin={isSuperAdmin}
          userOrgId={userOrgId}
          onClose={() => setShowCreateRoleModal(false)}
          onSuccess={() => {
            setShowCreateRoleModal(false);
            onRefresh();
          }}
        />
      )}

      {showEditRoleModal && selectedRole && (
        <EditRoleModal
          role={selectedRole}
          onClose={() => {
            setShowEditRoleModal(false);
            setSelectedRole(null);
          }}
          onSuccess={() => {
            setShowEditRoleModal(false);
            setSelectedRole(null);
            onRefresh();
          }}
        />
      )}

      {showRolePermissionsModal && selectedRole && (
        <RolePermissionsModal
          role={selectedRole}
          onClose={() => {
            setShowRolePermissionsModal(false);
            setSelectedRole(null);
          }}
          onSuccess={() => {
            setShowRolePermissionsModal(false);
            setSelectedRole(null);
            onRefresh();
          }}
        />
      )}
    </div>
  );
}
