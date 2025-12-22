import { useState } from 'react';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@librechat/client';
import { Button } from '@librechat/client';
import { saasApi } from '~/services/saasApi';
import { Loader2 } from 'lucide-react';

interface EditFileModalProps {
  file: {
    id: string;
    name: string;
  };
  onClose: () => void;
  onSuccess: () => void;
}

export default function EditFileModal({ file, onClose, onSuccess }: EditFileModalProps) {
  const [formData, setFormData] = useState({
    name: file.name,
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);

    try {
      // Extract name without extension
      const nameWithoutExt = formData.name.replace(/\.[^/.]+$/, '');
      const extension = file.name.split('.').pop() || '';
      const newName = extension ? `${nameWithoutExt}.${extension}` : nameWithoutExt;

      await saasApi.updateFile(file.id, {
        name: newName,
      });
      onSuccess();
    } catch (err: any) {
      setError(err.message || 'Failed to update file');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={true} onOpenChange={onClose}>
      <DialogContent className="max-w-md p-6">
        <DialogHeader className="mb-4">
          <DialogTitle className="text-xl font-semibold">Rename File</DialogTitle>
        </DialogHeader>
        {error && (
          <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-3 text-red-700 dark:text-red-400 mb-4 text-sm">
            {error}
          </div>
        )}
        <form onSubmit={handleSubmit} className="space-y-5">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              File Name
            </label>
            <input
              type="text"
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              className="w-full px-4 py-2.5 border border-gray-300 dark:border-gray-400 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100"
              placeholder="Enter file name"
              required
              disabled={loading}
            />
          </div>
          <div className="flex justify-end gap-3 pt-4">
            <Button type="button" onClick={onClose} variant="outline" disabled={loading}>
              Cancel
            </Button>
            <Button type="submit" disabled={loading || !formData.name.trim()}>
              {loading ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin mr-2" />
                  Updating...
                </>
              ) : (
                'Update'
              )}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}

