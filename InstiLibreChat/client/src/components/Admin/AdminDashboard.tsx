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
  const [selectedRoleOrgFilter, setSelectedRoleOrgFilter] = useState('');
  const [permissionManager, setPermissionManager] = useState<PermissionManager | null>(null);
  const navigate = useNavigate();

  const isSuperAdmin = userInfo?.is_super_admin || false;
  const userOrgId = userInfo?.org_id || null;

  // Load permissions from localStorage
  useEffect(() => {
    let perms: Permission[] = [];
    
    // If superadmin, grant ALL permissions automatically
    if (isSuperAdmin) {
      // Create all possible permissions for superadmin
      const resources = ['organizations', 'users', 'roles', 'permissions'];
      const actions = ['read', 'create', 'update', 'delete'];
      
      perms = resources.flatMap(resource =>
        actions.map(action => ({
          id: `${resource}-${action}`,
          resource,
          action, 
        }))
      );
      
      console.log('🔑 Superadmin detected - granting all permissions:', perms.length);
    } else {
      // For non-superadmins, load from localStorage
      const storedPerms = localStorage.getItem('permissions');
      if (storedPerms) {
        try {
          perms = JSON.parse(storedPerms);
        } catch (error) {
          console.error('Error parsing permissions:', error);
        }
      }
    }
    
    setPermissions(perms);
    setPermissionManager(new PermissionManager(perms));
  }, [isSuperAdmin]);

  // Fetch organizations
  const fetchOrganizations = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await saasApi.getOrganizations(isSuperAdmin, userOrgId || undefined);
      // Handle both response formats
      // After normalizeResponse: { organizations: [...], page, limit, total, total_pages }
      const orgs = Array.isArray(data) 
        ? data 
        : (data as any).organizations || (data as any).data || ((data as any).id ? [data] : []);
      console.log('📊 Fetched organizations:', { count: orgs.length, data, orgs });
      setOrganizations(orgs);
    } catch (err: any) {
      console.error('❌ Error fetching organizations:', err);
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
      // Handle both response formats
      // After normalizeResponse: { users: [...], page, limit, total, total_pages }
      const usersList = Array.isArray(data) 
        ? data 
        : (data as any).users || (data as any).data || [];
      console.log('👥 Fetched users:', { count: usersList.length, isSuperAdmin, orgFilterId, data, usersList });
      
      // Note: Backend now handles filtering based on JWT claims
      // Super admin sees only org admins, org admin sees all users in their org
      // Client-side filtering is no longer needed, but kept for compatibility
        setUsers(usersList);
    } catch (err: any) {
      console.error('❌ Error fetching users:', err);
      setError(err.message || 'Failed to fetch users');
    } finally {
      setLoading(false);
    }
  };

  // Fetch roles
  const fetchRoles = async (orgFilterId: string | null = null) => {
    setLoading(true);
    setError(null);
    try {
      const data = await saasApi.getRoles(orgFilterId || undefined);
      const rolesList = Array.isArray(data) 
        ? data 
        : (data as any).roles || (data as any).data || [];
      const filteredRoles = isSuperAdmin
        ? rolesList
        : rolesList.filter((role: any) => {
            if (role.type === 'system') return true;
            if (!role.org_id || !userOrgId) return false;
            return String(role.org_id) === String(userOrgId);
          });
      console.log('🎭 Fetched roles:', { count: rolesList.length, filtered: filteredRoles.length });
      setRoles(filteredRoles);
    } catch (err: any) {
      console.error('❌ Error fetching roles:', err);
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
      const permsList = Array.isArray(data) 
        ? data 
        : (data as any).permissions || (data as any).data || [];
      console.log('🔐 Fetched permissions:', { count: permsList.length });
      setPermissions(permsList);
    } catch (err: any) {
      console.error('❌ Error fetching permissions:', err);
      setError(err.message || 'Failed to fetch permissions');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (activeTab === 'Organizations') fetchOrganizations();
    else if (activeTab === 'Users') fetchUsers(selectedOrgFilter || null);
    else if (activeTab === 'Roles') fetchRoles(selectedRoleOrgFilter || null);
    else if (activeTab === 'Permissions') fetchPermissions();
  }, [activeTab, isSuperAdmin, userOrgId, selectedOrgFilter, selectedRoleOrgFilter]);

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
              className={`px-0 py-3 border-b-2 font-normal text-[14px] leading-[20px] transition ${
                activeTab === tab
                  ? 'border-[#2434E7] text-[#2A2A2A] dark:text-gray-100'
                  : 'border-transparent text-[#6D6D6D] hover:text-[#2A2A2A] dark:text-gray-400 dark:hover:text-gray-200'
              }`}
              style={{ fontFamily: 'Inter, sans-serif' }}
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
                organizations={organizations}
                permissionManager={permissionManager}
                selectedOrgFilter={selectedRoleOrgFilter}
                onOrgFilterChange={setSelectedRoleOrgFilter}
                onRefresh={(orgFilterId) => fetchRoles(orgFilterId || null)}
              />
            )}
            {activeTab === 'Permissions' && <PermissionsView permissions={permissions} />}
          </>
        )}
      </div>
    </div>
  );
}

