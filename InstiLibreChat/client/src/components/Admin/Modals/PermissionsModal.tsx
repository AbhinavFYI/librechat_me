import { Dialog, DialogContent, DialogHeader, DialogTitle, Button } from '@librechat/client';

interface PermissionsModalProps {
  user: any;
  permissions: any[];
  onClose: () => void;
}

export default function PermissionsModal({ user, permissions, onClose }: PermissionsModalProps) {
  // Group permissions by resource
  const groupedPermissions = permissions.reduce((acc: any, perm: any) => {
    if (!acc[perm.resource]) {
      acc[perm.resource] = [];
    }
    acc[perm.resource].push(perm);
    return acc;
  }, {});

  return (
    <Dialog open={true} onOpenChange={onClose}>
      <DialogContent className="max-w-2xl max-h-[80vh] overflow-y-auto p-6">
        <DialogHeader className="mb-4">
          <DialogTitle className="text-xl font-semibold">Permissions for {user.email}</DialogTitle>
        </DialogHeader>

        {Object.keys(groupedPermissions).length === 0 ? (
          <p className="text-gray-500 dark:text-gray-400 text-center py-8">No permissions assigned</p>
        ) : (
          <div className="space-y-4">
            {Object.entries(groupedPermissions).map(([resource, perms]: [string, any]) => (
              <div
                key={resource}
                className="border border-gray-200 dark:border-gray-700 rounded-lg p-4"
              >
                <h3 className="font-semibold text-gray-900 dark:text-gray-100 mb-2 capitalize">
                  {resource.replace('_', ' ')}
                </h3>
                <div className="flex flex-wrap gap-2">
                  {perms.map((perm: any) => (
                    <span
                      key={perm.id}
                      className="px-3 py-1 bg-blue-100 dark:bg-blue-900 text-blue-800 dark:text-blue-200 rounded-full text-sm"
                    >
                      {perm.action}
                    </span>
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}

        <div className="mt-6">
          <Button onClick={onClose} variant="outline" className="w-full">
            Close
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}

