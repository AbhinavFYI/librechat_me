import { Dialog, DialogContent, DialogHeader, DialogTitle, Button } from '@librechat/client';
import { Permission } from '~/utils/permissions';

interface PermissionsModalProps {
  user: any;
  permissions: Permission[];
  onClose: () => void;
}

export default function PermissionsModal({ user, permissions, onClose }: PermissionsModalProps) {
  // Group permissions by resource
  const groupedPermissions = permissions.reduce((acc, perm) => {
    if (!acc[perm.resource]) {
      acc[perm.resource] = [];
    }
    acc[perm.resource].push(perm);
    return acc;
  }, {} as Record<string, Permission[]>);

  return (
    <Dialog open={true} onOpenChange={onClose}>
      <DialogContent className="max-w-3xl max-h-[90vh] overflow-y-auto p-6">
        <DialogHeader className="mb-4">
          <DialogTitle className="text-xl font-semibold">
            Permissions for {user.email}
          </DialogTitle>
        </DialogHeader>

        {permissions.length === 0 ? (
          <div className="text-center py-8">
            <p className="text-gray-500 dark:text-gray-400">
              No permissions assigned to this user
            </p>
          </div>
        ) : (
          <div className="space-y-4">
            {Object.entries(groupedPermissions).map(([resource, perms]) => (
              <div
                key={resource}
                className="border border-gray-200 dark:border-gray-600 rounded-lg p-4"
              >
                <h3 className="font-semibold text-gray-900 dark:text-gray-100 mb-3 capitalize">
                  {resource}
                </h3>
                <div className="flex flex-wrap gap-2">
                  {perms.map((perm) => (
                    <span
                      key={perm.id}
                      className="px-3 py-1 bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 rounded text-sm"
                    >
                      {perm.action}
                    </span>
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}

        <div className="flex gap-3 pt-4 border-t border-gray-200 dark:border-gray-700 mt-6">
          <Button
            type="button"
            onClick={onClose}
            className="flex-1 bg-blue-600 hover:bg-blue-700 text-white"
          >
            Close
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}

