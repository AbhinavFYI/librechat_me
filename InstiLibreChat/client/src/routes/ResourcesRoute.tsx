import { useState, useEffect, useMemo, useRef } from 'react';
import { createPortal } from 'react-dom';
import { Button } from '@librechat/client';
import { saasApi } from '~/services/saasApi';
import { PermissionManager } from '~/utils/permissions';
import { 
  Folder, File, Plus, Upload, Trash2, Edit, ChevronRight, Home, 
  FileText, Image, FileSpreadsheet, FileCode, FileVideo, FileAudio, FileArchive,
  Download, Eye, Grid3x3, List, MoreVertical, Search, X
} from 'lucide-react';
import CreateFolderModal from '~/components/Resources/Modals/CreateFolderModal';
import EditFolderModal from '~/components/Resources/Modals/EditFolderModal';
import UploadFileModal from '~/components/Resources/Modals/UploadFileModal';
import EditFileModal from '~/components/Resources/Modals/EditFileModal';

interface FolderNode {
  id: string;
  name: string;
  path: string;
  parent_id?: string;
  children?: FolderNode[];
  files?: FileNode[];
  created_by_name?: string;
  created_at: string;
}

interface FileNode {
  id: string;
  name: string;
  extension?: string;
  size_bytes?: number;
  created_at: string;
  storage_key?: string;
  created_by_name?: string;
}

// Get file icon based on extension
const getFileIcon = (extension?: string) => {
  if (!extension) return FileText;
  const ext = extension.toLowerCase();
  if (['jpg', 'jpeg', 'png', 'gif', 'svg', 'webp'].includes(ext)) return Image;
  if (['mp4', 'avi', 'mov', 'wmv'].includes(ext)) return FileVideo;
  if (['mp3', 'wav', 'flac'].includes(ext)) return FileAudio;
  if (['xls', 'xlsx', 'csv'].includes(ext)) return FileSpreadsheet;
  if (['js', 'ts', 'py', 'java', 'html', 'css', 'json'].includes(ext)) return FileCode;
  if (['zip', 'rar', '7z', 'tar'].includes(ext)) return FileArchive;
  if (['pdf', 'doc', 'docx'].includes(ext)) return FileText;
  return FileText;
};

// Format date for display
const formatDate = (dateString: string) => {
  try {
    const date = new Date(dateString);
    return date.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' });
  } catch {
    return dateString;
  }
};

export default function ResourcesRoute() {
  const [allFolders, setAllFolders] = useState<FolderNode[]>([]);
  const [currentFolderId, setCurrentFolderId] = useState<string | null>(null);
  const [breadcrumbs, setBreadcrumbs] = useState<Array<{ id: string | null; name: string }>>([{ id: null, name: 'Home' }]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedItem, setSelectedItem] = useState<{ type: 'folder' | 'file'; id: string } | null>(null);
  const [dropdownPosition, setDropdownPosition] = useState<{ top: number; right: number } | null>(null);
  const buttonRefs = useRef<Map<string, HTMLButtonElement>>(new Map());
  const [showCreateFolderModal, setShowCreateFolderModal] = useState(false);
  const [showEditFolderModal, setShowEditFolderModal] = useState(false);
  const [showUploadFileModal, setShowUploadFileModal] = useState(false);
  const [showEditFileModal, setShowEditFileModal] = useState(false);
  const [permissionManager, setPermissionManager] = useState<PermissionManager | null>(null);
  const [userInfo, setUserInfo] = useState<any>(null);
  const [organizations, setOrganizations] = useState<any[]>([]);
  const [selectedOrgId, setSelectedOrgId] = useState<string>('');
  const [viewMode, setViewMode] = useState<'list' | 'grid'>('list');
  const [activeTab, setActiveTab] = useState<'documents' | 'reports'>('documents');
  const [searchQuery, setSearchQuery] = useState<string>('');
  const [showSearch, setShowSearch] = useState<boolean>(false);

  const isSuperAdmin = userInfo?.is_super_admin || false;
  const userOrgId = userInfo?.org_id || null;

  useEffect(() => {
    loadUserInfo();
  }, []);

  useEffect(() => {
    if (userInfo) {
      if (isSuperAdmin) {
        loadOrganizations();
      } else {
        setSelectedOrgId(userOrgId || '');
        loadFolders(userOrgId);
      }
    }
  }, [userInfo]);

  useEffect(() => {
    if (isSuperAdmin && selectedOrgId) {
      localStorage.setItem('resources_selected_org_id', selectedOrgId);
      loadFolders(selectedOrgId);
    }
  }, [selectedOrgId]);

  // Close dropdown when clicking outside
  useEffect(() => {
    if (!selectedItem) {
      setDropdownPosition(null);
      return;
    }

    const handleClickOutside = (event: MouseEvent) => {
      const target = event.target as HTMLElement;
      // Don't close if clicking on dropdown trigger, menu, or modal
      if (
        target.closest('.dropdown-trigger') ||
        target.closest('[role="dialog"]') ||
        target.closest('.modal') ||
        showEditFolderModal ||
        showEditFileModal
      ) {
        return;
      }
      // Check if clicking on the portal dropdown
      if (target.closest('.fixed.w-48')) {
        return;
      }
      setSelectedItem(null);
      setDropdownPosition(null);
    };

    // Use a small delay to avoid closing immediately when opening
    const timeoutId = setTimeout(() => {
      document.addEventListener('mousedown', handleClickOutside);
    }, 100);

    return () => {
      clearTimeout(timeoutId);
      document.removeEventListener('mousedown', handleClickOutside);
    };
  }, [selectedItem, showEditFolderModal, showEditFileModal]);

  const loadUserInfo = async () => {
    try {
      const user: any = await saasApi.getMe();
      setUserInfo(user);
      if (user) {
        let permissions = user.permissions || [];
        if (!permissions || permissions.length === 0) {
          const storedPerms = localStorage.getItem('permissions');
          if (storedPerms) {
            try {
              permissions = JSON.parse(storedPerms);
            } catch (e) {
              console.error('Error parsing stored permissions:', e);
            }
          }
        }
        const pm = new PermissionManager(permissions as any[]);
        setPermissionManager(pm);
      }
    } catch (err) {
      console.error('Error loading user info:', err);
    }
  };

  const loadOrganizations = async () => {
    try {
      const data = await saasApi.getOrganizations(isSuperAdmin, undefined);
      const orgs = Array.isArray(data) ? data : (data as any).data || ((data as any).id ? [data] : []);
      setOrganizations(orgs);
      
      if (orgs.length > 0) {
        const savedOrgId = localStorage.getItem('resources_selected_org_id');
        if (savedOrgId && orgs.some((org: any) => org.id === savedOrgId)) {
          setSelectedOrgId(savedOrgId);
          return;
        }
        if (userOrgId) {
          const hasUserOrg = orgs.some((org: any) => org.id === userOrgId);
          if (hasUserOrg) {
            setSelectedOrgId(userOrgId);
            return;
          }
        }
        setSelectedOrgId(orgs[0].id);
      }
    } catch (err: any) {
      console.error('Error loading organizations:', err);
    }
  };

  const loadFolders = async (orgId?: string | null) => {
    try {
      setLoading(true);
      setError(null);
      
      if (isSuperAdmin) {
        if (organizations.length > 0 && !orgId && !selectedOrgId) {
          setAllFolders([]);
          setLoading(false);
          return;
        }
        const targetOrgId = orgId || selectedOrgId;
        if (!targetOrgId) {
          if (organizations.length > 0) {
            setError('Please select an organization to view its resources.');
          }
          setAllFolders([]);
          setLoading(false);
          return;
        }
        const data = await saasApi.getFolderTree(targetOrgId);
        setAllFolders(Array.isArray(data) ? data : []);
        return;
      }
      
      const targetOrgId = orgId || userOrgId;
      if (!targetOrgId) {
        setError('Organization ID is required.');
        setAllFolders([]);
        setLoading(false);
        return;
      }
      
      const data = await saasApi.getFolderTree(targetOrgId);
      setAllFolders(Array.isArray(data) ? data : []);
    } catch (err: any) {
      setError(err.message || 'Failed to load folders');
      console.error('Error loading folders:', err);
    } finally {
      setLoading(false);
    }
  };

  // Find folder by ID in tree
  const findFolder = (folders: FolderNode[], id: string | null): FolderNode | null => {
    if (!id) return null;
    for (const folder of folders) {
      if (folder.id === id) return folder;
      if (folder.children) {
        const found = findFolder(folder.children, id);
        if (found) return found;
      }
    }
    return null;
  };

  // Find Reports folder
  const findReportsFolder = (folders: FolderNode[]): FolderNode | null => {
    for (const folder of folders) {
      if (folder.name.toLowerCase() === 'reports') {
        return folder;
      }
      if (folder.children && folder.children.length > 0) {
        const found = findReportsFolder(folder.children);
        if (found) return found;
      }
    }
    return null;
  };

  // Get current folder and its contents
  const currentFolder = useMemo(() => {
    // If Reports tab is active, show Reports folder content
    if (activeTab === 'reports') {
      const reportsFolder = findReportsFolder(allFolders);
      if (reportsFolder) {
        return {
          folders: reportsFolder.children || [],
          files: reportsFolder.files || [],
        };
      }
      return {
        folders: [],
        files: [],
      };
    }

    // Documents tab - normal behavior
    // Completely exclude Reports folder from Documents tab (recursively)
    const reportsFolder = findReportsFolder(allFolders);
    const reportsFolderId = reportsFolder?.id;
    
    // Helper function to recursively filter out Reports folder
    const filterReportsFolder = (folders: FolderNode[]): FolderNode[] => {
      return folders
        .filter(f => f.id !== reportsFolderId)
        .map(folder => ({
          ...folder,
          children: folder.children ? filterReportsFolder(folder.children) : undefined,
        }));
    };
    
    if (!currentFolderId) {
      // Root level - return all root folders except Reports
      const filteredRootFolders = allFolders.filter(f => !f.parent_id && f.id !== reportsFolderId);
      return {
        folders: filterReportsFolder(filteredRootFolders),
        files: [] as FileNode[],
      };
    }
    const folder = findFolder(allFolders, currentFolderId);
    // Filter out Reports folder from subfolders recursively
    const filteredChildren = folder?.children ? filterReportsFolder(folder.children) : [];
    return {
      folders: filteredChildren,
      files: folder?.files || [],
    };
  }, [currentFolderId, allFolders, activeTab]);

  // Filter folders and files based on search query
  const filteredContent = useMemo(() => {
    if (!searchQuery.trim()) {
      return currentFolder;
    }
    const query = searchQuery.toLowerCase();
    return {
      folders: currentFolder.folders.filter(f => f.name.toLowerCase().includes(query)),
      files: currentFolder.files.filter(f => f.name.toLowerCase().includes(query)),
    };
  }, [currentFolder, searchQuery]);

  const navigateToFolder = (folderId: string | null, folderName: string) => {
    setCurrentFolderId(folderId);
    if (folderId === null) {
      setBreadcrumbs([{ id: null, name: 'Home' }]);
    } else {
      // Build breadcrumbs by finding path to folder
      const buildBreadcrumbs = (folders: FolderNode[], targetId: string, path: Array<{ id: string; name: string }> = []): Array<{ id: string; name: string }> | null => {
        for (const folder of folders) {
          if (folder.id === targetId) {
            return [...path, { id: folder.id, name: folder.name }];
          }
          if (folder.children) {
            const result = buildBreadcrumbs(folder.children, targetId, [...path, { id: folder.id, name: folder.name }]);
            if (result) return result;
          }
        }
        return null;
      };
      const crumbs = buildBreadcrumbs(allFolders, folderId);
      setBreadcrumbs([{ id: null, name: 'Home' }, ...(crumbs || [{ id: folderId, name: folderName }])]);
    }
    setSelectedItem(null);
  };

  const handleDeleteFolder = async (folder: FolderNode) => {
    if (!confirm(`Delete folder "${folder.name}" and all its contents?`)) return;
    try {
      await saasApi.deleteFolder(folder.id);
      loadFolders(isSuperAdmin ? selectedOrgId : userOrgId);
      if (currentFolderId === folder.id) {
        navigateToFolder(null, 'Home');
      }
    } catch (err: any) {
      alert(err.message || 'Failed to delete folder');
    }
  };

  const handleDeleteFile = async (file: FileNode) => {
    if (!confirm(`Delete file "${file.name}"?`)) return;
    try {
      await saasApi.deleteFile(file.id);
      loadFolders(isSuperAdmin ? selectedOrgId : userOrgId);
    } catch (err: any) {
      alert(err.message || 'Failed to delete file');
    }
  };

  const handleDownloadFile = async (file: FileNode) => {
    try {
      const blob = await saasApi.downloadFile(file.id);
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = file.name;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);
    } catch (err: any) {
      alert(err.message || 'Failed to download file');
    }
  };

  const handlePreviewFile = async (file: FileNode) => {
    try {
      // Get access token for authentication
      const token = localStorage.getItem('access_token');
      
      // Use static route if storage_key is available, otherwise fallback to file ID route
      if (file.storage_key) {
        const storagePath = 'uploads';
        let filePath = file.storage_key;
        // Remove storage path prefix if present
        if (filePath.startsWith(storagePath + '/')) {
          filePath = filePath.substring(storagePath.length + 1);
        } else if (filePath.startsWith('/' + storagePath + '/')) {
          filePath = filePath.substring(storagePath.length + 2);
        }
        // Use full URL to avoid React Router intercepting it
        // Append token as query parameter for authentication
        const staticUrl = `${window.location.origin}/static/resources/folder/file/${filePath}${token ? `?token=${encodeURIComponent(token)}` : ''}`;
        window.open(staticUrl, '_blank');
      } else {
        // Fallback to file ID route
        const fileUrl = `${window.location.origin}/files/${file.id}?direct=true${token ? `&token=${encodeURIComponent(token)}` : ''}`;
        window.open(fileUrl, '_blank');
      }
    } catch (err: any) {
      console.error('Preview error:', err);
      alert(err.message || 'Failed to preview file');
    }
  };


  const isOrgAdmin = userInfo?.org_role === 'admin';
  const hasFolderPermission = permissionManager?.canCreate('folders') || false;
  const hasFilePermission = permissionManager?.canCreate('files') || false;
  const hasFileUpdatePermission = permissionManager?.canUpdate('files') || false;
  const hasFileDeletePermission = permissionManager?.canDelete('files') || false;
  const canManage = isSuperAdmin || isOrgAdmin || hasFolderPermission || hasFilePermission;
  const canManageFiles = isSuperAdmin || isOrgAdmin || hasFileUpdatePermission || hasFileDeletePermission;

  if (loading) {
    return (
      <div className="flex h-screen items-center justify-center">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500 mx-auto mb-4"></div>
          <p className="text-gray-600 dark:text-gray-400">Loading resources...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="h-screen flex flex-col bg-gray-50 dark:bg-gray-900">
      {/* Header */}
      <div className="bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 px-6 py-4">
        <div className="flex justify-between items-center">
          <div className="flex items-center gap-4">
            {isSuperAdmin && organizations.length > 0 && (
              <div className="flex items-center gap-2">
                <label className="text-sm font-medium text-gray-700 dark:text-gray-300 whitespace-nowrap">
                  Organization:
                </label>
                <select
                  value={selectedOrgId}
                  onChange={(e) => {
                    const newOrgId = e.target.value;
                    setSelectedOrgId(newOrgId);
                    if (newOrgId) {
                      loadFolders(newOrgId);
                      navigateToFolder(null, 'Home');
                    }
                  }}
                  className="px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 min-w-[300px] max-w-[500px]"
                >
                  {organizations.map((org) => (
                    <option key={org.id} value={org.id}>
                      {org.name} {org.legal_name ? `(${org.legal_name})` : ''}
                    </option>
                  ))}
                </select>
              </div>
            )}
          </div>
          {canManage && (
            <div className="flex items-center gap-3">
              <button
                onClick={() => {
                  setShowCreateFolderModal(true);
                }}
                className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg font-medium transition-colors"
              >
                <Plus className="h-4 w-4" />
                New folder
              </button>
              <button
                onClick={() => {
                  setShowUploadFileModal(true);
                }}
                className="flex items-center gap-2 px-4 py-2 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300 rounded-lg font-medium transition-colors"
              >
                <Upload className="h-4 w-4" />
                Upload document
              </button>
              {showSearch ? (
                <div className="relative">
                  <input
                    id="resources-search"
                    type="text"
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    placeholder="Search files and folders..."
                    className="pl-10 pr-10 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 w-64"
                    autoFocus
                    onKeyDown={(e) => {
                      if (e.key === 'Escape') {
                        setShowSearch(false);
                        setSearchQuery('');
                      }
                    }}
                  />
                  <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-gray-400" />
                  <button
                    onClick={() => {
                      setShowSearch(false);
                      setSearchQuery('');
                    }}
                    className="absolute right-3 top-1/2 transform -translate-y-1/2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
                    title="Close search"
                  >
                    <X className="h-4 w-4" />
                  </button>
                </div>
              ) : (
                <button
                  onClick={() => {
                    setShowSearch(true);
                  }}
                  className="p-2 text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
                  title="Search files and folders"
                >
                  <Search className="h-5 w-5" />
                </button>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Tabs and Breadcrumbs */}
      <div className="bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700">
        <div className="px-6 py-3">
          <div className="flex items-center justify-between">
            {/* Tabs */}
            <div className="flex items-center gap-6 border-b border-gray-200 dark:border-gray-700 -mb-3">
              <button
                onClick={() => {
                  setActiveTab('documents');
                  navigateToFolder(null, 'Home');
                  setShowSearch(false); // Hide search when switching tabs
                }}
                className={`px-1 pb-3 text-sm font-medium transition-colors ${
                  activeTab === 'documents'
                    ? 'text-blue-600 dark:text-blue-400 border-b-2 border-blue-600 dark:border-blue-400'
                    : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'
                }`}
              >
                Documents
              </button>
              <button
                onClick={() => {
                  setActiveTab('reports');
                  setSearchQuery(''); // Clear search when switching tabs
                  setShowSearch(false); // Hide search when switching tabs
                }}
                className={`px-1 pb-3 text-sm font-medium transition-colors ${
                  activeTab === 'reports'
                    ? 'text-blue-600 dark:text-blue-400 border-b-2 border-blue-600 dark:border-blue-400'
                    : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'
                }`}
              >
                Reports
              </button>
            </div>
            
            {/* View Toggle and Breadcrumbs */}
            <div className="flex items-center gap-4">
              {/* View Toggle */}
              <div className="flex items-center gap-1 bg-gray-100 dark:bg-gray-700 rounded-lg p-1">
                <button
                  onClick={() => setViewMode('list')}
                  className={`p-1.5 rounded transition-colors ${
                    viewMode === 'list'
                      ? 'bg-white dark:bg-gray-600 text-blue-600 dark:text-blue-400 shadow-sm'
                      : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200'
                  }`}
                  title="List View"
                >
                  <List className="h-4 w-4" />
                </button>
                <button
                  onClick={() => setViewMode('grid')}
                  className={`p-1.5 rounded transition-colors ${
                    viewMode === 'grid'
                      ? 'bg-white dark:bg-gray-600 text-blue-600 dark:text-blue-400 shadow-sm'
                      : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200'
                  }`}
                  title="Grid View"
                >
                  <Grid3x3 className="h-4 w-4" />
                </button>
              </div>
              
              {/* Breadcrumbs - only show for Documents tab */}
              {activeTab === 'documents' && (
                <div className="flex items-center gap-2 text-sm">
                  {breadcrumbs.map((crumb, index) => (
                    <div key={crumb.id || 'home'} className="flex items-center gap-2">
                      {index > 0 && <ChevronRight className="h-4 w-4 text-gray-400" />}
                      <button
                        onClick={() => navigateToFolder(crumb.id, crumb.name)}
                        className={`px-2 py-1 rounded hover:bg-gray-100 dark:hover:bg-gray-700 ${
                          index === breadcrumbs.length - 1 ? 'font-semibold' : ''
                        }`}
                      >
                        {index === 0 ? <Home className="h-4 w-4 inline mr-1" /> : null}
                        {crumb.name}
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Content View */}
      <div className="flex-1 overflow-auto bg-gray-50 dark:bg-gray-900">
        {error && (
          <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-4 m-6 text-red-700 dark:text-red-400">
            {error}
          </div>
        )}

        {filteredContent.folders.length === 0 && filteredContent.files.length === 0 ? (
          <div className="text-center py-12">
            <Folder className="h-16 w-16 text-gray-400 mx-auto mb-4" />
            <p className="text-gray-600 dark:text-gray-400 mb-4">This folder is empty</p>
          </div>
        ) : viewMode === 'list' ? (
          /* List/Table View */
          <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 overflow-visible mx-6 my-4 rounded-lg">
            <table className="w-full">
              <thead className="bg-gray-50 dark:bg-gray-900 border-b border-gray-200 dark:border-gray-700">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Name</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Created by</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Date</th>
                  <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Actions</th>
                </tr>
              </thead>
              <tbody className="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
                {/* Folders */}
                {filteredContent.folders.map((folder) => {
                  const folderFileCount = folder.files?.length || 0;
                  return (
                    <tr
                      key={folder.id}
                      className="hover:bg-gray-50 dark:hover:bg-gray-700/50 cursor-pointer"
                      onDoubleClick={(e) => {
                        if (!(e.target as HTMLElement).closest('.dropdown-trigger, .dropdown-menu')) {
                          navigateToFolder(folder.id, folder.name);
                        }
                      }}
                    >
                      <td className="px-6 py-4 whitespace-nowrap">
                        <div className="flex items-center gap-3">
                          <Folder className="h-5 w-5 text-blue-500 flex-shrink-0" />
                          <div>
                            <div className="text-sm font-medium text-gray-900 dark:text-gray-100">{folder.name}</div>
                            {folderFileCount > 0 && (
                              <div className="text-xs text-gray-500 dark:text-gray-400">{folderFileCount} document{folderFileCount !== 1 ? 's' : ''}</div>
                            )}
                          </div>
                        </div>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                        {folder.created_by_name || 'Unknown'}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                        {formatDate(folder.created_at)}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                        <div className="flex items-center justify-end gap-2">
                          {canManage && (
                            <div className="relative">
                              <button
                                ref={(el) => {
                                  if (el) {
                                    buttonRefs.current.set(`folder-${folder.id}`, el);
                                  } else {
                                    buttonRefs.current.delete(`folder-${folder.id}`);
                                  }
                                }}
                                type="button"
                                onClick={(e) => {
                                  e.stopPropagation();
                                  e.preventDefault();
                                  const isCurrentlySelected = selectedItem?.type === 'folder' && selectedItem.id === folder.id;
                                  if (isCurrentlySelected) {
                                    setSelectedItem(null);
                                    setDropdownPosition(null);
                                  } else {
                                    const button = buttonRefs.current.get(`folder-${folder.id}`);
                                    if (button) {
                                      const rect = button.getBoundingClientRect();
                                      setDropdownPosition({
                                        top: rect.top - 8, // Position above button
                                        right: window.innerWidth - rect.right,
                                      });
                                    }
                                    setSelectedItem({ type: 'folder', id: folder.id });
                                  }
                                }}
                                className="dropdown-trigger p-1 hover:bg-gray-100 dark:hover:bg-gray-700 rounded"
                                title="More options"
                              >
                                <MoreVertical className="h-4 w-4 text-gray-500 dark:text-gray-400" />
                              </button>
                              {selectedItem?.type === 'folder' && selectedItem.id === folder.id && dropdownPosition && createPortal(
                                <div 
                                  className="fixed w-48 bg-white dark:bg-gray-800 rounded-md shadow-lg border border-gray-200 dark:border-gray-700 z-[9999]"
                                  style={{
                                    top: `${dropdownPosition.top}px`,
                                    right: `${dropdownPosition.right}px`,
                                    transform: 'translateY(-100%)',
                                  }}
                                  onClick={(e) => e.stopPropagation()}
                                >
                                  <div className="py-1">
                                    <button
                                      type="button"
                                      onClick={(e) => {
                                        e.stopPropagation();
                                        e.preventDefault();
                                        setShowEditFolderModal(true);
                                        setSelectedItem(null);
                                        setDropdownPosition(null);
                                      }}
                                      className="w-full text-left px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
                                    >
                                      <Edit className="h-4 w-4" />
                                      Rename
                                    </button>
                                    <button
                                      type="button"
                                      onClick={(e) => {
                                        e.stopPropagation();
                                        e.preventDefault();
                                        handleDeleteFolder(folder);
                                        setSelectedItem(null);
                                        setDropdownPosition(null);
                                      }}
                                      className="w-full text-left px-4 py-2 text-sm text-red-600 dark:text-red-400 hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
                                    >
                                      <Trash2 className="h-4 w-4" />
                                      Delete
                                    </button>
                                  </div>
                                </div>,
                                document.body
                              )}
                            </div>
                          )}
                        </div>
                      </td>
                    </tr>
                  );
                })}
                {/* Files */}
                {filteredContent.files.map((file) => {
                  const FileIcon = getFileIcon(file.extension);
                  return (
                    <tr
                      key={file.id}
                      className="hover:bg-gray-50 dark:hover:bg-gray-700/50 cursor-pointer"
                      onDoubleClick={() => handlePreviewFile(file)}
                    >
                      <td className="px-6 py-4 whitespace-nowrap">
                        <div className="flex items-center gap-3">
                          <FileIcon className="h-5 w-5 text-gray-500 flex-shrink-0" />
                          <div className="text-sm font-medium text-gray-900 dark:text-gray-100">{file.name}</div>
                        </div>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                        {file.created_by_name || 'Unknown'}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                        {formatDate(file.created_at)}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                        <div className="flex items-center justify-end gap-2">
                          <button
                            onClick={(e) => {
                              e.stopPropagation();
                              handlePreviewFile(file);
                            }}
                            className="p-1 hover:bg-blue-100 dark:hover:bg-blue-900/30 rounded"
                            title="Preview"
                          >
                            <Eye className="h-4 w-4 text-blue-600 dark:text-blue-400" />
                          </button>
                          <button
                            onClick={(e) => {
                              e.stopPropagation();
                              handleDownloadFile(file);
                            }}
                            className="p-1 hover:bg-green-100 dark:hover:bg-green-900/30 rounded"
                            title="Download"
                          >
                            <Download className="h-4 w-4 text-green-600 dark:text-green-400" />
                          </button>
                          {canManageFiles && (
                            <div className="relative">
                              <button
                                ref={(el) => {
                                  if (el) {
                                    buttonRefs.current.set(`file-${file.id}`, el);
                                  } else {
                                    buttonRefs.current.delete(`file-${file.id}`);
                                  }
                                }}
                                type="button"
                                onClick={(e) => {
                                  e.stopPropagation();
                                  e.preventDefault();
                                  const isCurrentlySelected = selectedItem?.type === 'file' && selectedItem.id === file.id;
                                  if (isCurrentlySelected) {
                                    setSelectedItem(null);
                                    setDropdownPosition(null);
                                  } else {
                                    const button = buttonRefs.current.get(`file-${file.id}`);
                                    if (button) {
                                      const rect = button.getBoundingClientRect();
                                      setDropdownPosition({
                                        top: rect.top - 8, // Position above button
                                        right: window.innerWidth - rect.right,
                                      });
                                    }
                                    setSelectedItem({ type: 'file', id: file.id });
                                  }
                                }}
                                className="dropdown-trigger p-1 hover:bg-gray-100 dark:hover:bg-gray-700 rounded"
                                title="More options"
                              >
                                <MoreVertical className="h-4 w-4 text-gray-500 dark:text-gray-400" />
                              </button>
                              {selectedItem?.type === 'file' && selectedItem.id === file.id && dropdownPosition && createPortal(
                                <div 
                                  className="fixed w-48 bg-white dark:bg-gray-800 rounded-md shadow-lg border border-gray-200 dark:border-gray-700 z-[9999]"
                                  style={{
                                    top: `${dropdownPosition.top}px`,
                                    right: `${dropdownPosition.right}px`,
                                    transform: 'translateY(-100%)',
                                  }}
                                  onClick={(e) => e.stopPropagation()}
                                >
                                  <div className="py-1">
                                    {(isSuperAdmin || isOrgAdmin || hasFileUpdatePermission) && (
                                      <button
                                        type="button"
                                        onClick={(e) => {
                                          e.stopPropagation();
                                          e.preventDefault();
                                          localStorage.setItem('editing_file_id', file.id);
                                          setShowEditFileModal(true);
                                          setSelectedItem(null);
                                          setDropdownPosition(null);
                                        }}
                                        className="w-full text-left px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
                                      >
                                        <Edit className="h-4 w-4" />
                                        Rename
                                      </button>
                                    )}
                                    {(isSuperAdmin || isOrgAdmin || hasFileDeletePermission) && (
                                      <button
                                        type="button"
                                        onClick={(e) => {
                                          e.stopPropagation();
                                          e.preventDefault();
                                          handleDeleteFile(file);
                                          setSelectedItem(null);
                                          setDropdownPosition(null);
                                        }}
                                        className="w-full text-left px-4 py-2 text-sm text-red-600 dark:text-red-400 hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
                                      >
                                        <Trash2 className="h-4 w-4" />
                                        Delete
                                      </button>
                                    )}
                                  </div>
                                </div>,
                                document.body
                              )}
                            </div>
                          )}
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        ) : (
          /* Grid View */
          <div className="p-6">
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
            {/* Folders */}
            {filteredContent.folders.map((folder) => {
              const FileIcon = Folder;
              const isSelected = selectedItem?.type === 'folder' && selectedItem.id === folder.id;
              return (
                <div
                  key={folder.id}
                  className={`flex flex-col items-center p-4 rounded-lg border-2 cursor-pointer transition-all ${
                    isSelected
                      ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20'
                      : 'border-transparent hover:border-gray-300 dark:hover:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-800'
                  }`}
                  onClick={() => setSelectedItem({ type: 'folder', id: folder.id })}
                  onDoubleClick={() => navigateToFolder(folder.id, folder.name)}
                >
                  <FileIcon className="h-16 w-16 text-blue-500 mb-2" />
                  <span className="text-xs text-center text-gray-900 dark:text-gray-100 truncate w-full px-1">
                    {folder.name}
                  </span>
                  {canManage && isSelected && (
                    <div className="flex gap-1 mt-2">
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          setShowEditFolderModal(true);
                        }}
                        className="p-1 hover:bg-gray-200 rounded"
                        title="Rename"
                      >
                        <Edit className="h-3 w-3" />
                      </button>
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          handleDeleteFolder(folder);
                        }}
                        className="p-1 hover:bg-red-100 rounded"
                        title="Delete"
                      >
                        <Trash2 className="h-3 w-3 text-red-600" />
                      </button>
                    </div>
                  )}
                </div>
              );
            })}

            {/* Files */}
            {filteredContent.files.map((file) => {
              const FileIcon = getFileIcon(file.extension);
              const isSelected = selectedItem?.type === 'file' && selectedItem.id === file.id;
              return (
                <div
                  key={file.id}
                  className={`flex flex-col items-center p-4 rounded-lg border-2 cursor-pointer transition-all ${
                    isSelected
                      ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20'
                      : 'border-transparent hover:border-gray-300 dark:hover:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-800'
                  }`}
                  onClick={() => setSelectedItem({ type: 'file', id: file.id })}
                  onDoubleClick={() => handlePreviewFile(file)}
                >
                  <FileIcon className="h-16 w-16 text-gray-500 mb-2" />
                  <span className="text-xs text-center text-gray-900 dark:text-gray-100 truncate w-full px-1">
                    {file.name}
                  </span>
                  {file.size_bytes && (
                    <span className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                      {(file.size_bytes / 1024).toFixed(1)} KB
                    </span>
                  )}
                  {isSelected && (
                    <div className="flex gap-1 mt-2">
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          handlePreviewFile(file);
                        }}
                        className="p-1 hover:bg-blue-100 rounded"
                        title="Preview"
                      >
                        <Eye className="h-3 w-3 text-blue-600" />
                      </button>
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          handleDownloadFile(file);
                        }}
                        className="p-1 hover:bg-green-100 rounded"
                        title="Download"
                      >
                        <Download className="h-3 w-3 text-green-600" />
                      </button>
                      {canManage && (
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            handleDeleteFile(file);
                          }}
                          className="p-1 hover:bg-red-100 rounded"
                          title="Delete"
                        >
                          <Trash2 className="h-3 w-3 text-red-600" />
                        </button>
                      )}
                    </div>
                  )}
                </div>
              );
            })}
            </div>
          </div>
        )}
      </div>

      {/* Modals */}
      {showCreateFolderModal && (
        <CreateFolderModal
          parentId={currentFolderId || undefined}
          orgId={isSuperAdmin ? selectedOrgId : userOrgId}
          onClose={() => setShowCreateFolderModal(false)}
          onSuccess={() => {
            setShowCreateFolderModal(false);
            loadFolders(isSuperAdmin ? selectedOrgId : userOrgId);
          }}
        />
      )}

      {showEditFolderModal && (
        <EditFolderModal
          folder={selectedItem?.type === 'folder' ? (findFolder(allFolders, selectedItem.id) || { id: '', name: '', path: '' }) : { id: '', name: '', path: '' }}
          onClose={() => {
            setShowEditFolderModal(false);
            setSelectedItem(null);
          }}
          onSuccess={() => {
            setShowEditFolderModal(false);
            setSelectedItem(null);
            // Reload folders to get updated names
            loadFolders(isSuperAdmin ? selectedOrgId : userOrgId);
          }}
        />
      )}

      {showUploadFileModal && (
        <UploadFileModal
          folderId={currentFolderId || undefined}
          orgId={isSuperAdmin ? selectedOrgId : userOrgId}
          folders={allFolders}
          onClose={() => setShowUploadFileModal(false)}
          onSuccess={() => {
            setShowUploadFileModal(false);
            loadFolders(isSuperAdmin ? selectedOrgId : userOrgId);
          }}
        />
      )}

      {showEditFileModal && (
        <EditFileModal
          file={(() => {
            if (selectedItem?.type === 'file') {
              // Find file in current folder
              const foundFile = filteredContent.files.find(f => f.id === selectedItem.id);
              return foundFile || { id: selectedItem.id, name: '' };
            }
            // Fallback - try to find by stored file ID
            const storedFileId = localStorage.getItem('editing_file_id');
            if (storedFileId) {
              const foundFile = filteredContent.files.find(f => f.id === storedFileId);
              localStorage.removeItem('editing_file_id');
              return foundFile || { id: storedFileId, name: '' };
            }
            return { id: '', name: '' };
          })()}
          onClose={() => {
            setShowEditFileModal(false);
            setSelectedItem(null);
            localStorage.removeItem('editing_file_id');
          }}
          onSuccess={() => {
            setShowEditFileModal(false);
            setSelectedItem(null);
            localStorage.removeItem('editing_file_id');
            // Reload folders to get updated file names
            loadFolders(isSuperAdmin ? selectedOrgId : userOrgId);
          }}
        />
      )}
    </div>
  );
}
