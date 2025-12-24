import { useState, useEffect } from 'react';
import { Dialog, DialogContent, DialogHeader, DialogTitle, Button, Input } from '@librechat/client';
import { saasApi } from '~/services/saasApi';

interface AddUserModalProps {
  isSuperAdmin: boolean;
  userOrgId: string | null;
  onClose: () => void;
  onSuccess: () => void;
}

export default function AddUserModal({
  isSuperAdmin,
  userOrgId,
  onClose,
  onSuccess,
}: AddUserModalProps) {
  const [formData, setFormData] = useState({
    email: '',
    password: '',
    first_name: '',
    last_name: '',
    phone: '',
    org_id: '',
    org_role: 'user',
    role_id: '',
    role_name: '',
  });
  const [availableRoles, setAvailableRoles] = useState<any[]>([]);
  const [availableOrganizations, setAvailableOrganizations] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    // Fetch available roles
    const fetchRoles = async () => {
      try {
        const data = await saasApi.getRoles();
        // Handle both response formats
        // After normalizeResponse: { roles: [...], page, limit, total, total_pages }
        const rolesList = Array.isArray(data) 
          ? data 
          : (data as any).roles || (data as any).data || [];
        console.log('🎭 AddUserModal - Fetched roles:', { count: rolesList.length, rolesList });
        
        // Remove duplicates based on role ID
        const uniqueRoles = Array.from(
          new Map(rolesList.map((role: any) => [role.id, role])).values()
        );
        console.log('🎭 AddUserModal - Unique roles:', { count: uniqueRoles.length });
        
        // Filter roles based on user type
        const filteredRoles = isSuperAdmin
          ? uniqueRoles
          : uniqueRoles.filter((role: any) => {
              if (role.type === 'system') return true;
              if (!role.org_id || !userOrgId) return false;
              const roleOrgId = typeof role.org_id === 'string' ? role.org_id : String(role.org_id);
              const userOrgIdStr = typeof userOrgId === 'string' ? userOrgId : String(userOrgId);
              return roleOrgId === userOrgIdStr;
            });
        console.log('🎭 AddUserModal - Filtered roles:', { count: filteredRoles.length, filteredRoles });
        setAvailableRoles(filteredRoles);
      } catch (error) {
        console.error('❌ Error fetching roles:', error);
      }
    };

    // Fetch organizations for super admin
    const fetchOrganizations = async () => {
      if (!isSuperAdmin) return;
      try {
        const data = await saasApi.getOrganizations(true);
        // Handle both response formats
        // After normalizeResponse: { organizations: [...], page, limit, total, total_pages }
        const orgsList = Array.isArray(data) 
          ? data 
          : (data as any).organizations || (data as any).data || [];
        console.log('🏢 AddUserModal - Fetched organizations:', { count: orgsList.length, orgsList });
        
        // Remove duplicates based on org ID
        const uniqueOrgs = Array.from(
          new Map(orgsList.map((org: any) => [org.id, org])).values()
        );
        console.log('🏢 AddUserModal - Unique organizations:', { count: uniqueOrgs.length });
        setAvailableOrganizations(uniqueOrgs);
      } catch (error) {
        console.error('❌ Error fetching organizations:', error);
      }
    };

    fetchRoles();
    fetchOrganizations();
  }, [isSuperAdmin, userOrgId]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);

    try {
      const payload: any = {
        email: formData.email,
        password: formData.password,
        first_name: formData.first_name || null,
        last_name: formData.last_name || null,
        phone: formData.phone || null,
        org_role: formData.org_role || 'user',
      };

      // Super admin can specify org_id, org admin uses their own org
      if (isSuperAdmin) {
        if (formData.org_id) {
          payload.org_id = formData.org_id;
        }
      } else if (userOrgId) {
        // Org admin: use their own org_id
        payload.org_id = userOrgId;
      }

      // Always include role assignment if selected
      if (formData.role_id && formData.role_id.trim() !== '') {
        payload.role_id = formData.role_id;
        console.log('🎭 Sending role_id:', formData.role_id);
      } else if (formData.role_name && formData.role_name.trim() !== '') {
        payload.role_name = formData.role_name;
        console.log('🎭 Sending role_name:', formData.role_name);
      }
      
      console.log('📤 Creating user with payload:', payload);

      await saasApi.createUser(payload);
      onSuccess();
    } catch (err: any) {
      setError(err.message || 'Failed to create user');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={true} onOpenChange={onClose}>
      <DialogContent className="max-w-md max-h-[90vh] overflow-y-auto p-6">
        <DialogHeader className="mb-4">
          <DialogTitle className="text-xl font-semibold">Add User</DialogTitle>
        </DialogHeader>
        {error && (
          <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-3 text-red-700 dark:text-red-400 mb-4 text-sm">
            {error}
          </div>
        )}
        <form onSubmit={handleSubmit} className="space-y-5">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Email *
            </label>
            <Input
              type="email"
              required
              value={formData.email}
              onChange={(e) => setFormData({ ...formData, email: e.target.value })}
              className="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-400 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Password *
            </label>
            <Input
              type="password"
              required
              minLength={8}
              value={formData.password}
              onChange={(e) => setFormData({ ...formData, password: e.target.value })}
              className="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-400 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100"
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                First Name
              </label>
              <Input
                type="text"
                value={formData.first_name}
                onChange={(e) => setFormData({ ...formData, first_name: e.target.value })}
                className="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-400 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                Last Name
              </label>
              <Input
                type="text"
                value={formData.last_name}
                onChange={(e) => setFormData({ ...formData, last_name: e.target.value })}
                className="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-400 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100"
              />
            </div>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Phone
            </label>
            <Input
              type="tel"
              value={formData.phone}
              onChange={(e) => setFormData({ ...formData, phone: e.target.value })}
              className="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-400 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100"
            />
          </div>

          {isSuperAdmin && (
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                Organization *
              </label>
              <select
                value={formData.org_id}
                onChange={(e) => setFormData({ ...formData, org_id: e.target.value })}
                className="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100"
                required
              >
                <option value="">-- Select Organization --</option>
                {availableOrganizations.map((org) => (
                  <option key={org.id} value={org.id}>
                    {org.name} {org.legal_name ? `(${org.legal_name})` : ''}
                  </option>
                ))}
              </select>
              {availableOrganizations.length === 0 && (
                <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                  No organizations available. Please create an organization first.
                </p>
              )}
            </div>
          )}

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Organization Role *
            </label>
            <select
              value={formData.org_role}
              onChange={(e) => setFormData({ ...formData, org_role: e.target.value })}
              className="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100"
              required
            >
              <option value="user">User (default)</option>
              <option value="admin">Admin (can login via OTP)</option>
              <option value="viewer">Viewer (read-only)</option>
            </select>
            <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
              This determines login eligibility. Admin can log in via OTP.
            </p>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Assign Role (Permissions)
            </label>
            <select
              value={formData.role_id || ''}
              onChange={(e) => {
                const selectedRoleId = e.target.value;
                if (selectedRoleId) {
                  const selectedRole = availableRoles.find((r) => r.id === selectedRoleId);
                  if (selectedRole) {
                    setFormData({
                      ...formData,
                      role_id: selectedRole.id,
                      role_name: selectedRole.name,
                    });
                  }
                } else {
                  setFormData({ ...formData, role_id: '', role_name: '' });
                }
              }}
              className="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100"
            >
              <option value="">No role (assign later)</option>
              {availableRoles.length === 0 ? (
                <option value="" disabled>
                  No roles available
                </option>
              ) : (
                availableRoles.map((role) => (
                  <option key={role.id} value={role.id}>
                    {role.name} {role.type === 'system' ? '(System)' : ''}
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
            <Button type="button" onClick={onClose} variant="outline" className="flex-1 bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100">
              Cancel
            </Button>
            <Button type="submit" disabled={loading} className="flex-1 bg-blue-600 hover:bg-blue-700 text-white disabled:bg-blue-400">
              {loading ? 'Creating...' : 'Create User'}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
