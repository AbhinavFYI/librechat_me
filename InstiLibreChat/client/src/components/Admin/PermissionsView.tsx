import { Permission } from '~/utils/permissions';

interface PermissionsViewProps {
  permissions: Permission[];
}

export default function PermissionsView({ permissions }: PermissionsViewProps) {
  // Group permissions by resource
  const groupedPermissions = permissions.reduce((acc, perm) => {
    if (!acc[perm.resource]) {
      acc[perm.resource] = [];
    }
    acc[perm.resource].push(perm);
    return acc;
  }, {} as Record<string, Permission[]>);

  return (
    <div>
      <h2 className="text-xl font-semibold text-gray-900 dark:text-gray-100 mb-4">Permissions</h2>

      <div className="space-y-6">
        {Object.entries(groupedPermissions).map(([resource, perms]) => (
          <div key={resource} className="border border-gray-200 dark:border-gray-700 rounded-lg p-4">
            <h3 className="font-semibold text-gray-900 dark:text-gray-100 mb-3 capitalize">
              {resource}
            </h3>
            <div className="flex flex-wrap gap-2">
              {perms.map((perm) => (
                <span
                  key={perm.id}
                  className="px-3 py-1 bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-300 rounded-full text-sm"
                >
                  {perm.action}
                </span>
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

