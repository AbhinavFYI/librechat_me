import { useState } from 'react';
import { Dialog, DialogContent, DialogHeader, DialogTitle, Button, Input, TextareaAutosize } from '@librechat/client';
import { saasApi } from '~/services/saasApi';

interface EditRoleModalProps {
  role: any;
  onClose: () => void;
  onSuccess: () => void;
}

export default function EditRoleModal({ role, onClose, onSuccess }: EditRoleModalProps) {
  const [formData, setFormData] = useState({
    name: role.name || '',
    description: role.description || '',
    is_default: role.is_default || false,
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);

    try {
      // Prepare payload - name is required, others are optional
      const payload: any = {
        name: formData.name.trim(),
      };

      // Description is optional - send null if empty
      if (formData.description && formData.description.trim() !== '') {
        payload.description = formData.description.trim();
      } else {
        payload.description = null;
      }

      // is_default is optional
      payload.is_default = formData.is_default;

      await saasApi.updateRole(role.id, payload);
      onSuccess();
    } catch (err: any) {
      setError(err.message || 'Failed to update role. Please try again.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={true} onOpenChange={onClose}>
      <DialogContent className="max-w-md p-6">
        <DialogHeader className="mb-4">
          <DialogTitle className="text-xl font-semibold">Edit Role</DialogTitle>
        </DialogHeader>
        {error && (
          <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-3 text-red-700 dark:text-red-400 mb-4 text-sm">
            {error}
          </div>
        )}
        <form onSubmit={handleSubmit} className="space-y-5">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Role Name
            </label>
            <Input
              type="text"
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              className="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-400 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100"
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
              className="w-full px-4 py-3 border border-gray-300 dark:border-gray-400 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 resize-none"
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

          <div className="flex gap-3 pt-4 border-t border-gray-200 dark:border-gray-700 mt-6">
            <Button type="button" onClick={onClose} variant="outline" className="flex-1 bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 border border-gray-300 dark:border-gray-400 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm">
              Cancel
            </Button>
            <Button type="submit" disabled={loading} className="flex-1 bg-blue-600 hover:bg-blue-700 text-white disabled:bg-blue-400 border border-gray-300 dark:border-gray-400 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm">
              {loading ? 'Updating...' : 'Update Role'}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}

