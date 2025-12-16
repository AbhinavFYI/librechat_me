import { useState, useEffect } from 'react';
import { Dialog, DialogContent, DialogHeader, DialogTitle, Button } from '@librechat/client';
import { saasApi } from '~/services/saasApi';

interface RolePermissionsModalProps {
  role: any;
  onClose: () => void;
  onSuccess: () => void;
}

export default function RolePermissionsModal({ role, onClose, onSuccess }: RolePermissionsModalProps) {
  const [availablePermissions, setAvailablePermissions] = useState<any[]>([]);
  const [rolePermissions, setRolePermissions] = useState<any[]>([]);
  const [selectedPermissions, setSelectedPermissions] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchData = async () => {
      try {
        // Fetch all permissions
        const permsData = await saasApi.getPermissions();
        setAvailablePermissions(Array.isArray(permsData) ? permsData : []);

        // Fetch role permissions
        const rolePermsData = await saasApi.getRolePermissions(role.id);
        const perms = Array.isArray(rolePermsData) ? rolePermsData : [];
        setRolePermissions(perms);
        setSelectedPermissions(perms.map((p: any) => p.id));
      } catch (error) {
        console.error('Error fetching data:', error);
      }
    };
    fetchData();
  }, [role.id]);

  const togglePermission = (permId: string) => {
    setSelectedPermissions((prev) =>
      prev.includes(permId) ? prev.filter((id) => id !== permId) : [...prev, permId],
    );
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);

    try {
      await saasApi.assignPermissionsToRole(role.id, selectedPermissions);
      onSuccess();
    } catch (err: any) {
      setError(err.message || 'Failed to update role permissions');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={true} onOpenChange={onClose}>
      <DialogContent className="max-w-3xl max-h-[90vh] overflow-y-auto p-6">
        <DialogHeader className="mb-4">
          <DialogTitle className="text-xl font-semibold">Manage Permissions for {role.name}</DialogTitle>
        </DialogHeader>
        {error && (
          <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-3 text-red-700 dark:text-red-400 mb-4 text-sm">
            {error}
          </div>
        )}
        <form onSubmit={handleSubmit} className="space-y-5">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Select Permissions
            </label>
            <div className="border border-gray-300 dark:border-gray-600 rounded-lg p-4 max-h-96 overflow-y-auto">
              <div className="grid grid-cols-2 gap-2">
                {availablePermissions.map((perm) => (
                  <label key={perm.id} className="flex items-center">
                    <input
                      type="checkbox"
                      checked={selectedPermissions.includes(perm.id)}
                      onChange={() => togglePermission(perm.id)}
                      className="mr-2"
                    />
                    <span className="text-sm text-gray-700 dark:text-gray-300">
                      {perm.resource} - {perm.action}
                    </span>
                  </label>
                ))}
              </div>
              {availablePermissions.length === 0 && (
                <p className="text-gray-500 dark:text-gray-400 text-sm text-center py-4">
                  No permissions available
                </p>
              )}
            </div>
          </div>

          <div className="flex gap-3 pt-4 border-t border-gray-200 dark:border-gray-700 mt-6">
            <Button type="button" onClick={onClose} variant="outline" className="flex-1">
              Cancel
            </Button>
            <Button type="submit" disabled={loading} className="flex-1">
              {loading ? 'Updating...' : 'Update Permissions'}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}

