import { useState } from 'react';
import { Dialog, DialogContent, DialogHeader, DialogTitle, Button, Input } from '@librechat/client';
import { saasApi } from '~/services/saasApi';
import { PermissionManager } from '~/utils/permissions';
import CreateOrganizationModal from './Modals/CreateOrganizationModal';
import EditOrganizationModal from './Modals/EditOrganizationModal';

interface OrganizationsViewProps {
  organizations: any[];
  isSuperAdmin: boolean;
  permissionManager: PermissionManager | null;
  onRefresh: () => void;
}

export default function OrganizationsView({
  organizations,
  isSuperAdmin,
  permissionManager,
  onRefresh,
}: OrganizationsViewProps) {
  const [showAddOrgModal, setShowAddOrgModal] = useState(false);
  const [showEditOrgModal, setShowEditOrgModal] = useState(false);
  const [selectedOrg, setSelectedOrg] = useState<any>(null);

  const handleDeleteOrg = async (org: any) => {
    if (!confirm(`Are you sure you want to delete organization "${org.name}"? This action cannot be undone.`)) {
      return;
    }

    try {
      await saasApi.deleteOrganization(org.id);
      onRefresh();
    } catch (error: any) {
      alert(error.message || 'Failed to delete organization');
    }
  };

  return (
    <div>
      <div className="flex justify-between items-center mb-4">
        <h2 className="text-xl font-semibold text-gray-900 dark:text-gray-100">Organizations</h2>
        {isSuperAdmin && permissionManager && permissionManager.canCreate('organizations') && (
          <Button
            onClick={() => setShowAddOrgModal(true)}
            className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
          >
            + Add Organization
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
                Slug
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Status
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Users
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Created
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Actions
              </th>
            </tr>
          </thead>
          <tbody className="bg-white dark:bg-gray-850 divide-y divide-gray-200 dark:divide-gray-700">
            {organizations.length === 0 ? (
              <tr>
                <td colSpan={6} className="px-6 py-4 text-center text-gray-500 dark:text-gray-400">
                  No organizations found
                </td>
              </tr>
            ) : (
              organizations.map((org) => (
                <tr key={org.id} className="hover:bg-gray-50 dark:hover:bg-gray-800">
                  <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900 dark:text-gray-100">
                    <div className="flex items-center gap-3">
                      {org.logo_url && org.logo_url.startsWith('data:') && (
                        <img
                          src={org.logo_url}
                          alt={`${org.name} logo`}
                          className="h-8 w-8 object-contain rounded border border-gray-200 dark:border-gray-700"
                          onError={(e) => {
                            console.error('Failed to load logo for org:', org.id);
                            // Hide image if it fails to load
                            (e.target as HTMLImageElement).style.display = 'none';
                          }}
                        />
                      )}
                      <span>{org.name}</span>
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                    {org.slug}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <span
                      className={`px-2 py-1 text-xs rounded-full ${
                        org.status === 'active'
                          ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300'
                          : 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300'
                      }`}
                    >
                      {org.status}
                    </span>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                    {org.current_users || 0} / {org.max_users || 0}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                    {new Date(org.created_at).toLocaleDateString()}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm">
                    {permissionManager && permissionManager.canUpdate('organizations') && (
                      <button
                        onClick={() => {
                          setSelectedOrg(org);
                          setShowEditOrgModal(true);
                        }}
                        className="text-blue-600 hover:text-blue-900 dark:text-blue-400 dark:hover:text-blue-300 mr-3"
                      >
                        Edit
                      </button>
                    )}
                    {permissionManager && permissionManager.canDelete('organizations') && (
                      <button
                        onClick={() => handleDeleteOrg(org)}
                        className="text-red-600 hover:text-red-900 dark:text-red-400 dark:hover:text-red-300"
                      >
                        Delete
                      </button>
                    )}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {showAddOrgModal && (
        <CreateOrganizationModal
          onClose={() => setShowAddOrgModal(false)}
          onSuccess={() => {
            setShowAddOrgModal(false);
            onRefresh();
          }}
        />
      )}

      {showEditOrgModal && selectedOrg && (
        <EditOrganizationModal
          organization={selectedOrg}
          onClose={() => {
            setShowEditOrgModal(false);
            setSelectedOrg(null);
          }}
          onSuccess={() => {
            setShowEditOrgModal(false);
            setSelectedOrg(null);
            onRefresh();
          }}
        />
      )}
    </div>
  );
}

