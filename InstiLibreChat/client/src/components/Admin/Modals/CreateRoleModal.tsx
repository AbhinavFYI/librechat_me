import { useState, useEffect } from 'react';
import { Dialog, DialogContent, DialogHeader, DialogTitle, Button, Input, TextareaAutosize } from '@librechat/client';
import { saasApi } from '~/services/saasApi';

interface CreateRoleModalProps {
  isSuperAdmin: boolean;
  userOrgId: string | null;
  onClose: () => void;
  onSuccess: () => void;
}

export default function CreateRoleModal({
  isSuperAdmin,
  userOrgId,
  onClose,
  onSuccess,
}: CreateRoleModalProps) {
  const [formData, setFormData] = useState({
    name: '',
    description: '',
    is_default: false,
    org_id: null as string | null,
  });
  const [availablePermissions, setAvailablePermissions] = useState<any[]>([]);
  const [selectedPermissions, setSelectedPermissions] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchPermissions = async () => {
      try {
        const data = await saasApi.getPermissions();
        setAvailablePermissions(Array.isArray(data) ? data : []);
      } catch (error) {
        console.error('Error fetching permissions:', error);
      }
    };
    fetchPermissions();
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);

    try {
      const payload: any = {
        name: formData.name,
        description: formData.description,
        is_default: formData.is_default,
      };

      // Super admin can specify org_id, org admin uses their own org
      if (isSuperAdmin && formData.org_id) {
        payload.org_id = formData.org_id;
      }

      const roleData = await saasApi.createRole(payload);

      // If permissions were selected, assign them
      if (selectedPermissions.length > 0 && (roleData as any).id) {
        await saasApi.assignPermissionsToRole((roleData as any).id, selectedPermissions);
      }

      onSuccess();
    } catch (err: any) {
      setError(err.message || 'Failed to create role');
    } finally {
      setLoading(false);
    }
  };

  const togglePermission = (permId: string) => {
    setSelectedPermissions((prev) =>
      prev.includes(permId) ? prev.filter((id) => id !== permId) : [...prev, permId],
    );
  };

  return (
    <Dialog open={true} onOpenChange={onClose}>
      <DialogContent className="max-w-3xl max-h-[90vh] overflow-y-auto p-6">
        <DialogHeader className="mb-4">
          <DialogTitle className="text-xl font-semibold">Create Role</DialogTitle>
        </DialogHeader>
        {error && (
          <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-3 text-red-700 dark:text-red-400 mb-4 text-sm">
            {error}
          </div>
        )}
        <form onSubmit={handleSubmit} className="space-y-5">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Role Name *
            </label>
            <Input
              type="text"
              required
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Description
            </label>
            <TextareaAutosize
              value={formData.description}
              onChange={(e) => setFormData({ ...formData, description: e.target.value })}
              minRows={3}
              maxRows={6}
              aria-label="Role description"
              className="w-full px-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 resize-none"
            />
          </div>

          <div>
            <label className="flex items-center">
              <input
                type="checkbox"
                checked={formData.is_default}
                onChange={(e) => setFormData({ ...formData, is_default: e.target.checked })}
                className="mr-2"
              />
              <span className="text-sm text-gray-700 dark:text-gray-300">Set as default role</span>
            </label>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Permissions
            </label>
            <div className="border border-gray-300 dark:border-gray-600 rounded-lg p-4 max-h-64 overflow-y-auto">
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
              {loading ? 'Creating...' : 'Create Role'}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}

