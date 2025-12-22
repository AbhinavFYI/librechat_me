import { useState, useEffect } from 'react';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@librechat/client';
import { Button } from '@librechat/client';
import { saasApi } from '~/services/saasApi';
import { Trash2 } from 'lucide-react';

interface FolderPermissionsModalProps {
  folderId: string;
  onClose: () => void;
}

export default function FolderPermissionsModal({ folderId, onClose }: FolderPermissionsModalProps) {
  const [permissions, setPermissions] = useState<any[]>([]);
  const [roles, setRoles] = useState<any[]>([]);
  const [selectedRoleId, setSelectedRoleId] = useState('');
  const [selectedPermission, setSelectedPermission] = useState('read');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadData();
  }, [folderId]);

  const loadData = async () => {
    try {
      setLoading(true);
      const [permsData, rolesData] = await Promise.all([
        saasApi.getFolderPermissions(folderId),
        saasApi.getRoles(),
      ]);
      setPermissions(Array.isArray(permsData) ? permsData : []);
      setRoles(Array.isArray(rolesData) ? rolesData : []);
    } catch (err: any) {
      setError(err.message || 'Failed to load permissions');
    } finally {
      setLoading(false);
    }
  };

  const handleAssignPermission = async () => {
    if (!selectedRoleId) {
      setError('Please select a role');
      return;
    }

    try {
      setLoading(true);
      setError(null);
      await saasApi.assignFolderPermission(folderId, {
        role_id: selectedRoleId,
        permission: selectedPermission,
      });
      await loadData();
      setSelectedRoleId('');
      setSelectedPermission('read');
    } catch (err: any) {
      setError(err.message || 'Failed to assign permission');
    } finally {
      setLoading(false);
    }
  };

  const handleRemovePermission = async (roleId: string) => {
    if (!confirm('Are you sure you want to remove this permission?')) {
      return;
    }

    try {
      setLoading(true);
      await saasApi.removeFolderPermission(folderId, roleId);
      await loadData();
    } catch (err: any) {
      setError(err.message || 'Failed to remove permission');
    } finally {
      setLoading(false);
    }
  };

  const availableRoles = roles.filter(
    (role) => !permissions.some((perm) => perm.role_id === role.id)
  );

  return (
    <Dialog open={true} onOpenChange={onClose}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto p-6">
        <DialogHeader className="mb-4">
          <DialogTitle className="text-xl font-semibold">Folder Permissions</DialogTitle>
        </DialogHeader>
        {error && (
          <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-3 text-red-700 dark:text-red-400 mb-4 text-sm">
            {error}
          </div>
        )}

        <div className="space-y-4">
          <div className="border-b border-gray-200 dark:border-gray-700 pb-4">
            <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">
              Assign Permission
            </h3>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Role
                </label>
                <select
                  value={selectedRoleId}
                  onChange={(e) => setSelectedRoleId(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 dark:border-gray-400 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100"
                >
                  <option value="">Select a role</option>
                  {availableRoles.map((role) => (
                    <option key={role.id} value={role.id}>
                      {role.name}
                    </option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Permission
                </label>
                <select
                  value={selectedPermission}
                  onChange={(e) => setSelectedPermission(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 dark:border-gray-400 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100"
                >
                  <option value="read">Read</option>
                  <option value="write">Write</option>
                  <option value="delete">Delete</option>
                  <option value="move">Move</option>
                  <option value="share">Share</option>
                </select>
              </div>
            </div>
            <Button
              onClick={handleAssignPermission}
              disabled={loading || !selectedRoleId}
              className="mt-3"
            >
              Assign Permission
            </Button>
          </div>

          <div>
            <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">
              Current Permissions
            </h3>
            {permissions.length === 0 ? (
              <p className="text-sm text-gray-500 dark:text-gray-400">No permissions assigned</p>
            ) : (
              <div className="space-y-2">
                {permissions.map((perm) => (
                  <div
                    key={perm.id}
                    className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-800 rounded-lg"
                  >
                    <div>
                      <span className="text-sm font-medium text-gray-900 dark:text-gray-100">
                        {perm.role_name || 'Unknown Role'}
                      </span>
                      <span className="ml-2 text-sm text-gray-600 dark:text-gray-400">
                        ({perm.permission})
                      </span>
                    </div>
                    <button
                      onClick={() => handleRemovePermission(perm.role_id)}
                      className="p-1 hover:bg-red-100 dark:hover:bg-red-900/20 rounded"
                      title="Remove permission"
                    >
                      <Trash2 className="h-4 w-4 text-red-600 dark:text-red-400" />
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>

        <div className="flex justify-end gap-3 pt-4 mt-4 border-t border-gray-200 dark:border-gray-700">
          <Button onClick={onClose} variant="outline">
            Close
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}

