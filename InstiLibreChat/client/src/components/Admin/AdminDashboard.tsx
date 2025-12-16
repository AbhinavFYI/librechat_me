import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { saasApi } from '~/services/saasApi';
import { PermissionManager, type Permission } from '~/utils/permissions';
import OrganizationsView from './OrganizationsView';
import UsersView from './UsersView';
import RolesView from './RolesView';
import PermissionsView from './PermissionsView';

interface UserInfo {
  id: string;
  email: string;
  is_super_admin?: boolean;
  org_id?: string;
  org_role?: string;
}

interface AdminDashboardProps {
  userInfo?: UserInfo;
}

export default function AdminDashboard({ userInfo }: AdminDashboardProps) {
  const [activeTab, setActiveTab] = useState('Organizations');
  const [organizations, setOrganizations] = useState<any[]>([]);
  const [users, setUsers] = useState<any[]>([]);
  const [roles, setRoles] = useState<any[]>([]);
  const [permissions, setPermissions] = useState<Permission[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selectedOrgFilter, setSelectedOrgFilter] = useState('');
  const [permissionManager, setPermissionManager] = useState<PermissionManager | null>(null);
  const navigate = useNavigate();

  const isSuperAdmin = userInfo?.is_super_admin || false;
  const userOrgId = userInfo?.org_id || null;

  // Load permissions from localStorage
  useEffect(() => {
    const storedPerms = localStorage.getItem('permissions');
    if (storedPerms) {
      try {
        const perms = JSON.parse(storedPerms);
        setPermissions(perms);
        setPermissionManager(new PermissionManager(perms));
      } catch (error) {
        console.error('Error parsing permissions:', error);
      }
    }
  }, []);

  // Fetch organizations
  const fetchOrganizations = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await saasApi.getOrganizations(isSuperAdmin, userOrgId || undefined);
      const orgs = Array.isArray(data) ? data : (data as any).data || ((data as any).id ? [data] : []);
      setOrganizations(orgs);
    } catch (err: any) {
      setError(err.message || 'Failed to fetch organizations');
    } finally {
      setLoading(false);
    }
  };

  // Fetch users
  const fetchUsers = async (orgFilterId: string | null = null) => {
    setLoading(true);
    setError(null);
    try {
      const data = await saasApi.getUsers(isSuperAdmin, orgFilterId || undefined);
      const usersList = Array.isArray(data) ? data : (data as any).data || [];
      
      // Apply client-side filtering if needed
      if (isSuperAdmin && orgFilterId && orgFilterId !== '') {
        const filteredUsers = usersList.filter((user: any) => {
          const userOrgId = user.org_id ? String(user.org_id) : null;
          return userOrgId === orgFilterId;
        });
        setUsers(filteredUsers);
      } else if (!isSuperAdmin && userOrgId) {
        const filteredUsers = usersList.filter((user: any) => {
          const userOrgId = user.org_id ? String(user.org_id) : null;
          return userOrgId === String(userOrgId);
        });
        setUsers(filteredUsers);
      } else {
        setUsers(usersList);
      }
    } catch (err: any) {
      setError(err.message || 'Failed to fetch users');
    } finally {
      setLoading(false);
    }
  };

  // Fetch roles
  const fetchRoles = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await saasApi.getRoles();
      const rolesList = Array.isArray(data) ? data : (data as any).data || [];
      const filteredRoles = isSuperAdmin
        ? rolesList
        : rolesList.filter((role: any) => {
            if (role.type === 'system') return true;
            if (!role.org_id || !userOrgId) return false;
            return String(role.org_id) === String(userOrgId);
          });
      setRoles(filteredRoles);
    } catch (err: any) {
      setError(err.message || 'Failed to fetch roles');
    } finally {
      setLoading(false);
    }
  };

  // Fetch permissions
  const fetchPermissions = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await saasApi.getPermissions();
      const permsList = Array.isArray(data) ? data : (data as any).data || [];
      setPermissions(permsList);
    } catch (err: any) {
      setError(err.message || 'Failed to fetch permissions');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (activeTab === 'Organizations') fetchOrganizations();
    else if (activeTab === 'Users') fetchUsers(selectedOrgFilter || null);
    else if (activeTab === 'Roles') fetchRoles();
    else if (activeTab === 'Permissions') fetchPermissions();
  }, [activeTab, isSuperAdmin, userOrgId, selectedOrgFilter]);

  const tabs = ['Organizations', 'Users', 'Roles', 'Permissions'];

  return (
    <div className="h-full flex flex-col bg-white dark:bg-gray-850">
      {/* Secondary Navigation Tabs */}
      <div className="border-b border-gray-200 dark:border-gray-700 px-6">
        <div className="flex gap-6">
          {tabs.map((tab) => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              className={`py-4 px-1 border-b-2 font-medium transition ${
                activeTab === tab
                  ? 'border-blue-600 text-blue-600 dark:text-blue-400'
                  : 'border-transparent text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-200'
              }`}
            >
              {tab}
            </button>
          ))}
        </div>
      </div>

      {/* Content Area */}
      <div className="flex-1 overflow-y-auto p-6">
        {loading && (
          <div className="text-center py-8 text-gray-500 dark:text-gray-400">Loading...</div>
        )}

        {error && (
          <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-4 text-red-700 dark:text-red-400 mb-4">
            {error}
          </div>
        )}

        {!loading && !error && (
          <>
            {activeTab === 'Organizations' && (
              <OrganizationsView
                organizations={organizations}
                isSuperAdmin={isSuperAdmin}
                permissionManager={permissionManager}
                onRefresh={fetchOrganizations}
              />
            )}
            {activeTab === 'Users' && (
              <UsersView
                users={users}
                isSuperAdmin={isSuperAdmin}
                userOrgId={userOrgId}
                organizations={organizations}
                permissionManager={permissionManager}
                selectedOrgFilter={selectedOrgFilter}
                onOrgFilterChange={setSelectedOrgFilter}
                onRefresh={(orgFilterId) => fetchUsers(orgFilterId || null)}
              />
            )}
            {activeTab === 'Roles' && (
              <RolesView
                roles={roles}
                isSuperAdmin={isSuperAdmin}
                userOrgId={userOrgId}
                permissionManager={permissionManager}
                onRefresh={fetchRoles}
              />
            )}
            {activeTab === 'Permissions' && <PermissionsView permissions={permissions} />}
          </>
        )}
      </div>
    </div>
  );
}

