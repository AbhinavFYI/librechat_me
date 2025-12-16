import { useState, useEffect } from 'react';
import { Dialog, DialogContent, DialogHeader, DialogTitle, Button } from '@librechat/client';
import { saasApi } from '~/services/saasApi';

interface AssignRoleModalProps {
  user: any;
  isSuperAdmin: boolean;
  userOrgId: string | null;
  onClose: () => void;
  onSuccess: () => void;
}

export default function AssignRoleModal({
  user,
  isSuperAdmin,
  userOrgId,
  onClose,
  onSuccess,
}: AssignRoleModalProps) {
  const [availableRoles, setAvailableRoles] = useState<any[]>([]);
  const [userRoles, setUserRoles] = useState<any[]>([]);
  const [selectedRoleId, setSelectedRoleId] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    // Fetch available roles
    const fetchRoles = async () => {
      try {
        const data = await saasApi.getRoles();
        const rolesList = Array.isArray(data) ? data : (data as any).data || [];
        // Filter roles based on user type
        const filteredRoles = isSuperAdmin
          ? rolesList
          : rolesList.filter((role: any) => {
              if (role.type === 'system') return true;
              if (!role.org_id || !userOrgId) return false;
              const roleOrgId = typeof role.org_id === 'string' ? role.org_id : String(role.org_id);
              const userOrgIdStr = typeof userOrgId === 'string' ? userOrgId : String(userOrgId);
              return roleOrgId === userOrgIdStr;
            });
        setAvailableRoles(filteredRoles);
      } catch (error) {
        console.error('Error fetching roles:', error);
      }
    };

    // Fetch user's current roles - check prop first, then API
    const fetchUserRoles = async () => {
      // First check if user object passed as prop has roles
      if (user.roles && Array.isArray(user.roles) && user.roles.length > 0) {
        setUserRoles(user.roles);
        setSelectedRoleId(user.roles[0].id);
        return;
      }

      // Otherwise fetch from API
      try {
        const userData = await saasApi.getUsers(false, user.id);
        const userObj = Array.isArray(userData) ? userData.find((u: any) => u.id === user.id) : userData;
        if (userObj?.roles && Array.isArray(userObj.roles) && userObj.roles.length > 0) {
          setUserRoles(userObj.roles);
          setSelectedRoleId(userObj.roles[0].id);
          return;
        }
      } catch (error) {
        console.error('Error fetching user roles:', error);
      }

      setUserRoles([]);
      setSelectedRoleId('');
    };

    fetchRoles();
    fetchUserRoles();
  }, [isSuperAdmin, userOrgId, user.id, user.roles]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedRoleId) {
      setError('Please select a role');
      return;
    }

    setLoading(true);
    setError(null);

    try {
      // Ensure role_id is a valid UUID string
      if (!selectedRoleId || selectedRoleId.trim() === '') {
        throw new Error('Please select a valid role');
      }

      await saasApi.assignRoleToUser(user.id, selectedRoleId);
      onSuccess();
    } catch (err: any) {
      setError(err.message || 'Failed to assign role. Please try again.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={true} onOpenChange={onClose}>
      <DialogContent className="max-w-md p-6">
        <DialogHeader className="mb-4">
          <DialogTitle className="text-xl font-semibold">Assign Role to {user.email}</DialogTitle>
        </DialogHeader>
        {error && (
          <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-3 text-red-700 dark:text-red-400 mb-4 text-sm">
            {error}
          </div>
        )}
        <form onSubmit={handleSubmit} className="space-y-5">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Select Role
            </label>
            <select
              value={selectedRoleId}
              onChange={(e) => {
                const newRoleId = e.target.value;
                setSelectedRoleId(newRoleId);
              }}
              className="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100"
              required
            >
              <option value="">-- Select a role --</option>
              {availableRoles.length === 0 ? (
                <option value="" disabled>
                  No roles available
                </option>
              ) : (
                availableRoles.map((role) => (
                  <option key={role.id} value={role.id}>
                    {role.name} {role.type === 'system' ? '(System)' : ''}
                    {userRoles.some((ur) => ur.id === role.id) ? ' (Current)' : ''}
                  </option>
                ))
              )}
            </select>
            {availableRoles.length === 0 && (
              <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                No roles available. Please create a role first.
              </p>
            )}
          </div>

          <div className="flex gap-3 pt-4 border-t border-gray-200 dark:border-gray-700 mt-6">
            <Button type="button" onClick={onClose} variant="outline" className="flex-1">
              Cancel
            </Button>
            <Button type="submit" disabled={loading} className="flex-1">
              {loading ? 'Assigning...' : 'Assign Role'}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}

