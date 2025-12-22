import { useState } from 'react';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@librechat/client';
import { Input } from '@librechat/client';
import { Button } from '@librechat/client';
import { saasApi } from '~/services/saasApi';

interface CreateFolderModalProps {
  parentId?: string;
  orgId?: string | null;
  onClose: () => void;
  onSuccess: () => void;
}

export default function CreateFolderModal({ parentId, orgId, onClose, onSuccess }: CreateFolderModalProps) {
  const [formData, setFormData] = useState({
    name: '',
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);

    try {
      await saasApi.createFolder({
        name: formData.name,
        parent_id: parentId || undefined,
        org_id: orgId || undefined,
      });
      onSuccess();
    } catch (err: any) {
      setError(err.message || 'Failed to create folder');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={true} onOpenChange={onClose}>
      <DialogContent className="max-w-md p-6">
        <DialogHeader className="mb-4">
          <DialogTitle className="text-xl font-semibold">
            {parentId ? 'Create New Folder Inside' : 'Create New Folder'}
          </DialogTitle>
          {parentId && (
            <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
              This folder will be created inside the selected folder
            </p>
          )}
        </DialogHeader>
        {error && (
          <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-3 text-red-700 dark:text-red-400 mb-4 text-sm">
            {error}
          </div>
        )}
        <form onSubmit={handleSubmit} className="space-y-5">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Folder Name *
            </label>
            <Input
              type="text"
              required
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              placeholder="Enter folder name"
              className="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-400 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100"
            />
          </div>

          <div className="flex justify-end gap-3 pt-4">
            <Button type="button" onClick={onClose} variant="outline" className="flex-1 bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100">
              Cancel
            </Button>
            <Button type="submit" disabled={loading} className="flex-1 bg-blue-600 hover:bg-blue-700 text-white disabled:bg-blue-400">
              {loading ? 'Creating...' : 'New folder'}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}

