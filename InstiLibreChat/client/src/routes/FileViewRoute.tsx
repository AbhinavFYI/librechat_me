import { useEffect, useState } from 'react';
import { useParams, useNavigate, useSearchParams } from 'react-router-dom';
import { saasApi } from '~/services/saasApi';
import { Loader2, FileText, Download, AlertCircle } from 'lucide-react';

export default function FileViewRoute() {
  const { fileId } = useParams<{ fileId: string }>();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [fileInfo, setFileInfo] = useState<any>(null);
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  
  // Check if this is a direct file view (no UI wrapper)
  const directView = searchParams.get('direct') === 'true';

  // Check authentication via localStorage token
  useEffect(() => {
    const token = localStorage.getItem('access_token');
    if (!token) {
      navigate('/login');
    }
  }, [navigate]);

  useEffect(() => {
    if (!fileId) {
      setError('File ID is required');
      setLoading(false);
      return;
    }

    loadFile();
  }, [fileId]);

  const loadFile = async () => {
    try {
      setLoading(true);
      setError(null);

      console.log('Loading file:', fileId);

      // Get file metadata
      const file: any = await saasApi.getFile(fileId!);
      console.log('File metadata:', file);
      setFileInfo(file);

      // Use static file route for preview
      const token = localStorage.getItem('access_token');
      if (!token) {
        throw new Error('Authentication required');
      }
      
      // Construct static URL from storage_key
      // storage_key format: {storagePath}/{org_id}/{folder_path}/{file_name}
      // static route format: /static/resources/folder/file/{org_id}/{folder_path}/{file_name}
      let staticUrl = '';
      if (file.storage_key) {
        // Remove storage path prefix (default is "uploads")
        // Extract path after storage path
        const storagePath = 'uploads'; // Default storage path
        let filePath = file.storage_key;
        
        // Remove storage path prefix if present
        if (filePath.startsWith(storagePath + '/')) {
          filePath = filePath.substring(storagePath.length + 1);
        } else if (filePath.startsWith('/' + storagePath + '/')) {
          filePath = filePath.substring(storagePath.length + 2);
        }
        
        // Append token as query parameter for authentication (needed for img/iframe tags)
        staticUrl = `/static/resources/folder/file/${filePath}${token ? `?token=${encodeURIComponent(token)}` : ''}`;
      } else {
        // Fallback to old route if storage_key not available
        staticUrl = `/files/${fileId}?download=true${token ? `&token=${encodeURIComponent(token)}` : ''}`;
      }
      
      console.log('Loading file from static route:', staticUrl);
      
      // Try with Authorization header first (for fetch requests)
      const response = await fetch(staticUrl, {
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${token}`,
        },
        credentials: 'include', // Include cookies if needed
      });
      
      if (!response.ok) {
        // Check content-type before trying to parse error
        const contentType = response.headers.get('content-type') || '';
        let errorMessage = response.statusText;
        
        // Only try to parse as JSON if it's actually JSON
        if (contentType.includes('application/json')) {
          try {
            const errorData = await response.json();
            errorMessage = errorData.message || errorData.error || response.statusText;
          } catch {
            // If JSON parsing fails, just use status text
          }
        } else {
          // For non-JSON responses (like PDFs), just read as text but don't try to parse
          try {
            const errorText = await response.text();
            // Only use error text if it's short and looks like an error message
            if (errorText && errorText.length < 500 && !errorText.startsWith('%PDF')) {
              errorMessage = errorText;
            }
          } catch {
            // If reading fails, just use status text
          }
        }
        
        console.error('Download failed:', {
          status: response.status,
          statusText: response.statusText,
          contentType,
          error: errorMessage,
        });
        throw new Error(`Failed to download file: ${errorMessage}`);
      }
      
      const contentType = response.headers.get('content-type') || 'application/octet-stream';
      console.log('Response Content-Type:', contentType);
      
      const blob = await response.blob();
      console.log('Downloaded blob:', {
        size: blob.size,
        type: blob.type,
        contentType,
      });
      
      if (!blob || blob.size === 0) {
        throw new Error('File is empty or could not be loaded');
      }

      // Create a new blob with the correct MIME type if the blob type is wrong
      let finalBlob = blob;
      if (blob.type === 'application/octet-stream' || blob.type === '' || !blob.type) {
        // Recreate blob with correct type from Content-Type header
        finalBlob = new Blob([blob], { type: contentType });
        console.log('Recreated blob with type:', contentType, 'from Content-Type header');
      }

      // Validate blob size
      if (finalBlob.size === 0) {
        throw new Error('Downloaded file is empty');
      }

      // For images, validate the blob is actually an image
      if (file.extension && ['png', 'jpg', 'jpeg', 'gif', 'svg'].includes(file.extension)) {
        // Read first few bytes to verify it's an image
        const arrayBuffer = await finalBlob.slice(0, 8).arrayBuffer();
        const bytes = new Uint8Array(arrayBuffer);
        const isValidImage = 
          // PNG: starts with 89 50 4E 47
          (bytes[0] === 0x89 && bytes[1] === 0x50 && bytes[2] === 0x4E && bytes[3] === 0x47) ||
          // JPEG: starts with FF D8 FF
          (bytes[0] === 0xFF && bytes[1] === 0xD8 && bytes[2] === 0xFF) ||
          // GIF: starts with GIF87a or GIF89a
          (bytes[0] === 0x47 && bytes[1] === 0x49 && bytes[2] === 0x46) ||
          // SVG: starts with <svg or <?xml
          (bytes[0] === 0x3C && (bytes[1] === 0x73 || bytes[1] === 0x3F));
        
        if (!isValidImage && finalBlob.size > 8) {
          console.warn('Blob does not appear to be a valid image file');
        }
      }

      const url = window.URL.createObjectURL(finalBlob);
      console.log('Created object URL:', url, 'for blob type:', finalBlob.type, 'size:', finalBlob.size);
      setPreviewUrl(url);
    } catch (err: any) {
      console.error('Error loading file:', err);
      setError(err.message || 'Failed to load file');
    } finally {
      setLoading(false);
    }
  };

  const handleDownload = async () => {
    if (!fileId) return;
    
    try {
      const blob = await saasApi.downloadFile(fileId);
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = fileInfo?.name || 'file';
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);
    } catch (err: any) {
      alert(err.message || 'Failed to download file');
    }
  };

  // Cleanup URL on unmount
  useEffect(() => {
    return () => {
      if (previewUrl) {
        window.URL.revokeObjectURL(previewUrl);
      }
    };
  }, [previewUrl]);

  // Hide ALL app UI when in direct view mode - this route is outside AuthLayout
  useEffect(() => {
    if (directView) {
      // Hide everything except our file view element
      const style = document.createElement('style');
      style.id = 'direct-file-view-style';
      style.textContent = `
        html, body {
          margin: 0 !important;
          padding: 0 !important;
          overflow: hidden !important;
          height: 100% !important;
          width: 100% !important;
        }
        #root {
          margin: 0 !important;
          padding: 0 !important;
          height: 100vh !important;
          width: 100vw !important;
          overflow: hidden !important;
          position: fixed !important;
          top: 0 !important;
          left: 0 !important;
        }
        #root > *:not([data-file-view]) {
          display: none !important;
        }
        [data-file-view] {
          display: block !important;
          position: fixed !important;
          top: 0 !important;
          left: 0 !important;
          width: 100vw !important;
          height: 100vh !important;
          z-index: 999999 !important;
        }
        [data-file-view] img,
        [data-file-view] iframe {
          display: block !important;
          visibility: visible !important;
        }
      `;
      document.head.appendChild(style);
      
      return () => {
        const existingStyle = document.getElementById('direct-file-view-style');
        if (existingStyle) {
          existingStyle.remove();
        }
      };
    }
  }, [directView]);

  if (loading) {
    // For direct view, show minimal loading with data-file-view
    if (directView) {
      return (
        <div data-file-view style={{ 
          position: 'fixed', 
          top: 0, 
          left: 0, 
          width: '100vw', 
          height: '100vh', 
          display: 'flex', 
          alignItems: 'center', 
          justifyContent: 'center', 
          backgroundColor: '#000',
          color: '#fff'
        }}>
          <div style={{ textAlign: 'center' }}>
            <Loader2 className="h-8 w-8 animate-spin mx-auto mb-4" style={{ color: '#fff' }} />
            <p>Loading file...</p>
          </div>
        </div>
      );
    }
    return (
      <div className="flex items-center justify-center min-h-screen bg-gray-50 dark:bg-gray-900">
        <div className="text-center">
          <Loader2 className="h-8 w-8 animate-spin mx-auto mb-4 text-blue-600" />
          <p className="text-gray-600 dark:text-gray-400">Loading file...</p>
        </div>
      </div>
    );
  }

  if (error) {
    // For direct view, show error with data-file-view
    if (directView) {
      return (
        <div data-file-view style={{ 
          position: 'fixed', 
          top: 0, 
          left: 0, 
          width: '100vw', 
          height: '100vh', 
          display: 'flex', 
          alignItems: 'center', 
          justifyContent: 'center', 
          backgroundColor: '#000',
          color: '#fff',
          padding: '20px'
        }}>
          <div style={{ textAlign: 'center', maxWidth: '500px' }}>
            <AlertCircle className="h-12 w-12 mx-auto mb-4" style={{ color: '#ef4444' }} />
            <h2 style={{ fontSize: '20px', fontWeight: 'bold', marginBottom: '10px' }}>Error Loading File</h2>
            <p style={{ marginBottom: '20px', color: '#ccc' }}>{error}</p>
            <button
              onClick={() => window.close()}
              style={{
                padding: '10px 20px',
                backgroundColor: '#3b82f6',
                color: '#fff',
                border: 'none',
                borderRadius: '8px',
                cursor: 'pointer'
              }}
            >
              Close
            </button>
          </div>
        </div>
      );
    }
    return (
      <div className="flex items-center justify-center min-h-screen bg-gray-50 dark:bg-gray-900">
        <div className="text-center max-w-md p-6">
          <AlertCircle className="h-12 w-12 mx-auto mb-4 text-red-600" />
          <h2 className="text-xl font-semibold mb-2 text-gray-900 dark:text-gray-100">Error Loading File</h2>
          <p className="text-gray-600 dark:text-gray-400 mb-4">{error}</p>
          <button
            onClick={() => navigate('/resources')}
            className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
          >
            Go to Resources
          </button>
        </div>
      </div>
    );
  }

  const canPreview = fileInfo?.extension === 'pdf' || 
                     fileInfo?.extension === 'html' || 
                     fileInfo?.extension === 'htm' ||
                     fileInfo?.extension === 'png' ||
                     fileInfo?.extension === 'jpg' ||
                     fileInfo?.extension === 'jpeg' ||
                     fileInfo?.extension === 'gif' ||
                     fileInfo?.extension === 'svg';

  // For direct view with preview URL, show just the file without any UI
  if (directView && previewUrl && canPreview && fileInfo) {
    console.log('Rendering direct view:', {
      extension: fileInfo.extension,
      previewUrl,
      canPreview,
    });

    // For images, show them directly without iframe
    if (fileInfo.extension === 'png' || fileInfo.extension === 'jpg' || 
        fileInfo.extension === 'jpeg' || fileInfo.extension === 'gif' || 
        fileInfo.extension === 'svg') {
      // Use static route for images - construct URL from storage_key
      const token = localStorage.getItem('access_token');
      let staticImageUrl = '';
      if (fileInfo.storage_key) {
        const storagePath = 'uploads';
        let filePath = fileInfo.storage_key;
        if (filePath.startsWith(storagePath + '/')) {
          filePath = filePath.substring(storagePath.length + 1);
        } else if (filePath.startsWith('/' + storagePath + '/')) {
          filePath = filePath.substring(storagePath.length + 2);
        }
        // Append token as query parameter for authentication
        staticImageUrl = `${window.location.origin}/static/resources/folder/file/${filePath}${token ? `?token=${encodeURIComponent(token)}` : ''}`;
      } else {
        staticImageUrl = `${window.location.origin}/files/${fileId}?download=true${token ? `&token=${encodeURIComponent(token)}` : ''}`;
      }
      
      return (
        <div data-file-view style={{ 
          position: 'fixed', 
          top: 0, 
          left: 0, 
          width: '100vw', 
          height: '100vh', 
          display: 'flex', 
          alignItems: 'center', 
          justifyContent: 'center', 
          backgroundColor: '#000',
          margin: 0,
          padding: 0,
          zIndex: 99999
        }}>
          <img 
            src={staticImageUrl}
            alt={fileInfo.name || 'Image'}
            style={{ maxWidth: '100%', maxHeight: '100%', objectFit: 'contain' }}
            crossOrigin="anonymous"
            onError={(e) => {
              console.error('Image load error:', e);
              console.error('Static URL:', staticImageUrl);
              console.error('File info:', fileInfo);
              setError(`Failed to load image: ${fileInfo.name || 'Unknown file'}`);
            }}
            onLoad={() => {
              console.log('Image loaded successfully from static route:', staticImageUrl);
            }}
          />
        </div>
      );
    }
    
    // For PDF and HTML, use static route directly in iframe
    const token = localStorage.getItem('access_token');
    let staticFileUrl = '';
    if (fileInfo.storage_key) {
      const storagePath = 'uploads';
      let filePath = fileInfo.storage_key;
      if (filePath.startsWith(storagePath + '/')) {
        filePath = filePath.substring(storagePath.length + 1);
      } else if (filePath.startsWith('/' + storagePath + '/')) {
        filePath = filePath.substring(storagePath.length + 2);
      }
      // Append token as query parameter for authentication
      staticFileUrl = `${window.location.origin}/static/resources/folder/file/${filePath}${token ? `?token=${encodeURIComponent(token)}` : ''}`;
    } else {
      staticFileUrl = previewUrl || `${window.location.origin}/files/${fileId}?download=true${token ? `&token=${encodeURIComponent(token)}` : ''}`;
    }
    
    return (
      <iframe
        data-file-view
        src={staticFileUrl}
        style={{ 
          position: 'fixed', 
          top: 0, 
          left: 0, 
          width: '100vw', 
          height: '100vh', 
          border: 'none', 
          zIndex: 99999,
          margin: 0,
          padding: 0
        }}
        title={fileInfo.name || 'File'}
        onError={(e) => {
          console.error('Iframe load error:', e);
          setError('Failed to load file');
        }}
        onLoad={() => {
          console.log('Iframe loaded successfully from static route:', staticFileUrl);
        }}
      />
    );
  }

  // If direct view but no preview URL or file info, show error
  if (directView && (!previewUrl || !fileInfo)) {
    return (
      <div data-file-view style={{ 
        position: 'fixed', 
        top: 0, 
        left: 0, 
        width: '100vw', 
        height: '100vh', 
        display: 'flex', 
        alignItems: 'center', 
        justifyContent: 'center', 
        backgroundColor: '#000',
        color: '#fff',
        padding: '20px'
      }}>
        <div style={{ textAlign: 'center' }}>
          <AlertCircle className="h-12 w-12 mx-auto mb-4" style={{ color: '#ef4444' }} />
          <p>File not available for preview</p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col h-screen bg-gray-50 dark:bg-gray-900">
      {/* Header */}
      <div className="bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 px-6 py-4 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <FileText className="h-5 w-5 text-gray-600 dark:text-gray-400" />
          <div>
            <h1 className="text-lg font-semibold text-gray-900 dark:text-gray-100">
              {fileInfo?.name || 'File'}
            </h1>
            {fileInfo?.size_bytes && (
              <p className="text-sm text-gray-500 dark:text-gray-400">
                {(fileInfo.size_bytes / 1024).toFixed(1)} KB
              </p>
            )}
          </div>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={handleDownload}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
          >
            <Download className="h-4 w-4" />
            Download
          </button>
          <button
            onClick={() => navigate('/resources')}
            className="px-4 py-2 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
          >
            Back to Resources
          </button>
        </div>
      </div>

      {/* Preview Area */}
      <div className="flex-1 overflow-hidden">
        {previewUrl && canPreview ? (
          <iframe
            src={previewUrl}
            className="w-full h-full border-0"
            title={fileInfo?.name}
          />
        ) : (
          <div className="flex items-center justify-center h-full">
            <div className="text-center max-w-md p-6">
              <FileText className="h-16 w-16 mx-auto mb-4 text-gray-400" />
              <h3 className="text-lg font-semibold mb-2 text-gray-900 dark:text-gray-100">
                {fileInfo?.name || 'File'}
              </h3>
              <p className="text-gray-600 dark:text-gray-400 mb-4">
                This file type cannot be previewed in the browser.
              </p>
              <button
                onClick={handleDownload}
                className="flex items-center gap-2 px-6 py-3 bg-blue-600 text-white rounded-lg hover:bg-blue-700 mx-auto"
              >
                <Download className="h-5 w-5" />
                Download File
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

