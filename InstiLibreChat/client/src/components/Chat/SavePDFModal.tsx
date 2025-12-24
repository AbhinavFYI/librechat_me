import { useState, useEffect } from 'react';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@librechat/client';
import { Button } from '@librechat/client';
import { saasApi } from '~/services/saasApi'; // API service - connects to insti-inquora backend
import { Loader2 } from 'lucide-react';
import html2canvas from 'html2canvas';
import jsPDF from 'jspdf';

interface SavePDFModalProps {
  conversationId: string;
  pdfContent: string;
  onClose: () => void;
}

interface FlatFolder {
  id: string;
  name: string;
  path: string;
  level: number;
}

export default function SavePDFModal({ conversationId, pdfContent, onClose }: SavePDFModalProps) {
  const [selectedFolderId, setSelectedFolderId] = useState<string>('');
  const [folders, setFolders] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [userInfo, setUserInfo] = useState<any>(null);
  const [organizations, setOrganizations] = useState<any[]>([]);
  const [selectedOrgId, setSelectedOrgId] = useState<string>('');
  const [pdfName, setPdfName] = useState<string>('');

  useEffect(() => {
    loadUserInfo();
  }, []);

  useEffect(() => {
    if (userInfo) {
      if (userInfo.is_super_admin) {
        loadOrganizations();
      } else {
        // For regular users, use their org_id
        if (userInfo.org_id) {
          setSelectedOrgId(userInfo.org_id);
          loadFolders(userInfo.org_id);
        } else {
          setError('You are not associated with an organization. Please contact your administrator.');
        }
      }
    }
  }, [userInfo]);

  useEffect(() => {
    // When super admin selects an org, load folders for that org
    if (userInfo?.is_super_admin && selectedOrgId) {
      loadFolders(selectedOrgId);
    }
  }, [selectedOrgId]);

  const loadUserInfo = async () => {
    try {
      // API call: GET /api/v1/auth/me (insti-inquora)
      const user: any = await saasApi.getMe();
      setUserInfo(user);
    } catch (err) {
      console.error('Error loading user info:', err);
      setError('Failed to load user information');
    }
  };

  const loadOrganizations = async () => {
    try {
      // API call: GET /api/v1/organizations (insti-inquora)
      const data = await saasApi.getOrganizations(true, undefined);
      const orgs = Array.isArray(data) ? data : (data as any).data || [];
      setOrganizations(orgs);
      if (orgs.length > 0) {
        // Select first org by default for super admin
        setSelectedOrgId(orgs[0].id);
      }
    } catch (err: any) {
      console.error('Error loading organizations:', err);
      setError('Failed to load organizations');
    }
  };

  const loadFolders = async (orgId: string) => {
    if (!orgId) {
      setFolders([]);
      return;
    }
    try {
      setLoading(true);
      setError(null);
      // API call: GET /api/v1/folders/tree?org_id={orgId} (insti-inquora)
      const data = await saasApi.getFolderTree(orgId);
      setFolders(Array.isArray(data) ? data : []);
    } catch (err: any) {
      setError(err.message || 'Failed to load folders');
      setFolders([]);
    } finally {
      setLoading(false);
    }
  };

  const flattenFolders = (folderNodes: any[], level = 0): FlatFolder[] => {
    let result: FlatFolder[] = [];
    folderNodes.forEach((folder) => {
      result.push({
        id: folder.id,
        name: folder.name,
        path: folder.path || folder.name,
        level,
      });
      if (folder.children && folder.children.length > 0) {
        result = result.concat(flattenFolders(folder.children, level + 1));
      }
    });
    return result;
  };

  const flatFolders = flattenFolders(folders);

  // Find or create Reports folder
  const findOrCreateReportsFolder = async (orgId: string): Promise<string | null> => {
    try {
      // First, try to find existing Reports folder
      const findReportsFolder = (folders: any[]): any | null => {
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

      const reportsFolder = findReportsFolder(folders);
      if (reportsFolder) {
        return reportsFolder.id;
      }

      // If not found, create Reports folder
      // API call: POST /api/v1/folders (insti-inquora)
      const newFolderResponse: any = await saasApi.createFolder({
        name: 'Reports',
        parent_id: undefined, // Root level
        org_id: orgId,
      });

      // Extract folder ID from response (handle multiple formats)
      const newFolderId = newFolderResponse?.id || newFolderResponse?.data?.id || newFolderResponse?.folder?.id;
      
      if (newFolderId) {
        // Reload folders to refresh the tree
        await loadFolders(orgId);
        return newFolderId;
      }

      // Fallback: Try to find the newly created folder
      const updatedFoldersResponse: any = await saasApi.getFolderTree(orgId);
      const updatedFolders = Array.isArray(updatedFoldersResponse) 
        ? updatedFoldersResponse 
        : (updatedFoldersResponse?.data || []);
      const updatedFlat = flattenFolders(updatedFolders);
      const newReportsFolder = updatedFlat.find(f => f.name.toLowerCase() === 'reports');
      
      return newReportsFolder?.id || null;
    } catch (err: any) {
      console.error('Error finding/creating Reports folder:', err);
      return null;
    }
  };

  const handleSave = async () => {
    try {
      setSaving(true);
      setError(null);

      // Create a temporary div to render HTML
      const tempDiv = document.createElement('div');
      tempDiv.innerHTML = pdfContent;
      tempDiv.style.position = 'absolute';
      tempDiv.style.left = '-9999px';
      tempDiv.style.width = '800px';
      tempDiv.style.backgroundColor = '#ffffff';
      tempDiv.style.overflow = 'hidden';
      tempDiv.style.boxSizing = 'border-box';
      document.body.appendChild(tempDiv);
      
      // Wait for DOM to be ready and get table positions
      await new Promise((resolve) => setTimeout(resolve, 200));
      
      // Get positions of all table wrappers before canvas conversion
      // This helps us avoid splitting tables across pages
      const tableWrappers = tempDiv.querySelectorAll('.table-wrapper, table');
      const tablePositions: Array<{ top: number; bottom: number; height: number }> = [];
      
      // Get the container's scroll height for accurate positioning
      const containerHeight = tempDiv.scrollHeight || tempDiv.offsetHeight;
      
      tableWrappers.forEach((wrapper) => {
        let element = wrapper as HTMLElement;
        // If it's a table, find its wrapper
        if (element.tagName === 'TABLE' && element.parentElement) {
          const wrapper = element.closest('.table-wrapper') || element.parentElement;
          element = wrapper as HTMLElement;
        }
        
        let top = 0;
        let currentElement: HTMLElement | null = element;
        
        // Calculate position relative to tempDiv
        while (currentElement && currentElement !== tempDiv) {
          top += currentElement.offsetTop;
          currentElement = currentElement.offsetParent as HTMLElement | null;
        }
        
        const height = element.offsetHeight || element.scrollHeight;
        const bottom = top + height;
        
        tablePositions.push({
          top: top,
          bottom: bottom,
          height: height,
        });
      });
      
      // Sort table positions by top
      tablePositions.sort((a, b) => a.top - b.top);

      // Wait for all images to fully load before capturing
      const images = tempDiv.querySelectorAll('img');
      const imagePromises = Array.from(images).map((img: HTMLImageElement) => {
        return new Promise<void>((resolve) => {
          if (img.complete && img.naturalHeight !== 0) {
            resolve();
          } else {
            img.onload = () => resolve();
            img.onerror = () => resolve(); // Continue even if image fails to load
            // Timeout after 5 seconds
            setTimeout(() => resolve(), 5000);
          }
        });
      });
      
      await Promise.all(imagePromises);
      
      // Additional wait to ensure rendering is complete
      await new Promise((resolve) => setTimeout(resolve, 500));

      // Try to load logo image for header (PNG format)
      let logoDataUrl: string | null = null;
      try {
        const logoUrl = `${window.location.origin}/assets/fyers-logo.jpg`;
        const logoResponse = await fetch(logoUrl);
        if (logoResponse.ok) {
          const logoBlob = await logoResponse.blob();
          logoDataUrl = await new Promise<string>((resolve) => {
            const reader = new FileReader();
            reader.onload = () => resolve(reader.result as string);
            reader.readAsDataURL(logoBlob);
          });
        }
      } catch (error) {
        console.log('Logo not available, using text fallback:', error);
      }

      // Convert HTML to canvas using html2canvas with better image handling
      const canvas = await html2canvas(tempDiv, {
        scale: 2,
        useCORS: true,
        logging: false,
        backgroundColor: '#ffffff',
        allowTaint: false,
        imageTimeout: 15000, // 15 seconds timeout for images
        removeContainer: false,
        onclone: (clonedDoc) => {
          // Ensure all images in cloned document are fully loaded
          const clonedImages = clonedDoc.querySelectorAll('img');
          clonedImages.forEach((img: HTMLImageElement) => {
            // Set display to block to prevent inline spacing issues
            img.style.display = 'block';
            img.style.maxWidth = '100%';
            img.style.height = 'auto';
            img.style.pageBreakInside = 'avoid';
            img.style.breakInside = 'avoid';
          });
          
          // Ensure tables are properly structured and styled
          const clonedTables = clonedDoc.querySelectorAll('table');
          clonedTables.forEach((table) => {
            const htmlTable = table as HTMLTableElement;
            
            // Wrap table in a container if not already wrapped to prevent page breaks
            let wrapper = htmlTable.parentElement;
            if (!wrapper || (!wrapper.classList.contains('table-wrapper') && wrapper.tagName !== 'DIV')) {
              wrapper = clonedDoc.createElement('div');
              wrapper.className = 'table-wrapper';
              wrapper.style.cssText = 'page-break-inside: avoid !important; break-inside: avoid !important; margin: 20px 0; padding: 10px 0; display: block; width: 100%; min-height: 50px;';
              
              const parent = htmlTable.parentElement;
              if (parent) {
                parent.insertBefore(wrapper, htmlTable);
                wrapper.appendChild(htmlTable);
              }
            } else {
              // Ensure wrapper has proper styles with extra spacing
              if (wrapper.classList.contains('table-wrapper')) {
                wrapper.style.cssText = 'page-break-inside: avoid !important; break-inside: avoid !important; margin: 20px 0; padding: 10px 0; display: block; width: 100%; min-height: 50px;';
              }
            }
            
            // Force table layout to ensure proper rendering
            htmlTable.style.display = 'table';
            htmlTable.style.width = '100%';
            htmlTable.style.maxWidth = '100%';
            htmlTable.style.borderCollapse = 'separate';
            htmlTable.style.borderSpacing = '0';
            htmlTable.style.tableLayout = 'auto';
            htmlTable.style.boxSizing = 'border-box';
            htmlTable.style.pageBreakInside = 'avoid';
            htmlTable.style.breakInside = 'avoid';
            
            // Add extra spacing before and after table to prevent cuts
            const tableWrapper = htmlTable.closest('.table-wrapper') || htmlTable.parentElement;
            if (tableWrapper) {
              (tableWrapper as HTMLElement).style.marginTop = '20px';
              (tableWrapper as HTMLElement).style.marginBottom = '20px';
              (tableWrapper as HTMLElement).style.paddingTop = '10px';
              (tableWrapper as HTMLElement).style.paddingBottom = '10px';
            }
            
            // Ensure all table cells have proper borders and styling
            const cells = htmlTable.querySelectorAll('th, td');
            cells.forEach((cell) => {
              const htmlCell = cell as HTMLTableCellElement;
              // Ensure borders are visible
              if (!htmlCell.style.border || htmlCell.style.border === 'none') {
                htmlCell.style.border = '1px solid #d1d5db';
              }
              // Ensure padding
              if (!htmlCell.style.padding) {
                htmlCell.style.padding = '0.5rem 0.75rem';
              }
              // Ensure proper box sizing
              htmlCell.style.boxSizing = 'border-box';
              // Preserve text alignment
              const alignAttr = htmlCell.getAttribute('style');
              if (alignAttr && alignAttr.includes('text-align')) {
                // Alignment is already set, preserve it
              } else if (htmlCell.tagName === 'TD') {
                // Check if cell content looks numeric for right alignment
                const cellText = htmlCell.textContent?.trim() || '';
                const isNumeric = /^[\d,.\s%₹()\-]+$/.test(cellText) && 
                                 cellText !== '' && 
                                 cellText !== 'NA' &&
                                 !/^[A-Za-z]+$/.test(cellText);
                if (isNumeric) {
                  htmlCell.style.textAlign = 'right';
                  htmlCell.style.fontVariantNumeric = 'tabular-nums';
                }
              }
            });
            
            // Ensure table headers have background and proper styling
            const headers = htmlTable.querySelectorAll('th');
            headers.forEach((th) => {
              const htmlTh = th as HTMLTableCellElement;
              if (!htmlTh.style.backgroundColor || htmlTh.style.backgroundColor === 'transparent') {
                htmlTh.style.backgroundColor = '#f3f4f6';
              }
              htmlTh.style.fontWeight = '600';
              // Preserve header alignment
              const alignAttr = htmlTh.getAttribute('style');
              if (alignAttr && alignAttr.includes('text-align: right')) {
                htmlTh.style.textAlign = 'right';
              }
            });
            
            // Ensure table body rows have proper styling
            const rows = htmlTable.querySelectorAll('tbody tr');
            rows.forEach((row) => {
              const htmlRow = row as HTMLTableRowElement;
              htmlRow.style.borderBottom = '1px solid #d1d5db';
            });
            
            // Prevent table from breaking across pages
            htmlTable.style.pageBreakInside = 'avoid';
            htmlTable.style.breakInside = 'avoid';
          });
        },
      });
      document.body.removeChild(tempDiv);

      // Calculate PDF dimensions
      const pdfWidth = 210; // A4 width in mm
      const pdfHeight = 297; // A4 height in mm
      const margin = 15; // Top and bottom margin in mm
      const headerHeight = 25; // Header height in mm
      const footerHeight = 15; // Footer height in mm
      const usableHeight = pdfHeight - margin * 2 - headerHeight - footerHeight;
      
      const imgWidth = pdfWidth - (margin * 2);
      const imgHeight = (canvas.height * imgWidth) / canvas.width;
      
      // Get report date for footer
      const reportDate = new Date().toLocaleDateString('en-US', {
        year: 'numeric',
        month: 'long',
        day: 'numeric',
      });

      // Create PDF
      const pdf = new jsPDF('p', 'mm', 'a4');
      // Use PNG format with maximum quality to preserve all image data
      const imgData = canvas.toDataURL('image/png', 1.0);
      
      // Function to add header to each page with gradient and logo
      const addHeader = (pageNum: number, totalPages: number) => {
        // Draw gradient background from RIGHT to LEFT
        // Light blue (#3b82f6 = RGB 59, 130, 246) on LEFT, Dark blue (#1e40af = RGB 30, 64, 175) on RIGHT
        const steps = 30;
        for (let i = 0; i < steps; i++) {
          const ratio = i / (steps - 1); // 0 at left, 1 at right
          // Interpolate from light blue (LEFT) to dark blue (RIGHT) - reversed
          const r = Math.round(59 - (59 - 30) * ratio);   // 59 -> 30 (light to dark)
          const g = Math.round(130 - (130 - 64) * ratio);  // 130 -> 64 (light to dark)
          const b = Math.round(246 - (246 - 175) * ratio); // 246 -> 175 (light to dark)
          pdf.setFillColor(r, g, b);
          pdf.rect((pdfWidth / steps) * i, 0, pdfWidth / steps + 1, headerHeight, 'F');
        }
        
        // Add logo if available (PNG format) - logo already contains FYERS text
        let logoX = margin;
        if (logoDataUrl) {
          try {
            const logoHeight = 18; // Proper logo size
            const logoWidth = (logoHeight * 27) / 22; // Maintain aspect ratio (adjust if needed based on actual logo)
            pdf.addImage(logoDataUrl, 'JPG', margin, (headerHeight - logoHeight) / 2, logoWidth, logoHeight);
            logoX = margin + logoWidth + 10; // Gap between logo and Chat Conversation Report
          } catch (error) {
            console.log('Error adding logo to PDF:', error);
            // If logo fails, logoX stays at margin
          }
        }
        
        // Add "Chat Conversation Report" after logo (FYERS is already in the logo)
        pdf.setFontSize(16);
        pdf.setFont('helvetica', 'normal');
        pdf.setTextColor(255, 255, 255);
        pdf.text('Chat Conversation Report', logoX, headerHeight - 10);
        
        // Add date and report type on right
        pdf.setFontSize(11);
        pdf.setFont('helvetica', 'normal');
        const dateWidth = pdf.getTextWidth(reportDate);
        pdf.text(reportDate, pdfWidth - margin - dateWidth, headerHeight - 8);
        
        pdf.setFontSize(10);
        const reportTypeWidth = pdf.getTextWidth('Research Report');
        pdf.text('Research Report', pdfWidth - margin - reportTypeWidth, headerHeight - 3);
      };
      
      // Function to add footer to each page
      const addFooter = (pageNum: number, totalPages: number) => {
        const footerY = pdfHeight - footerHeight;
        pdf.setFillColor(248, 250, 252); // Light gray background
        pdf.rect(0, footerY, pdfWidth, footerHeight, 'F');
        
        pdf.setFontSize(9);
        pdf.setTextColor(100, 116, 139); // Gray text
        pdf.setFont('helvetica', 'normal');
        
        const footerText = `Generated by Fyers Research Platform | Page ${pageNum} of ${totalPages}`;
        const footerDate = `Report Date: ${reportDate}`;
        
        const textWidth = pdf.getTextWidth(footerText);
        pdf.text(footerText, (pdfWidth - textWidth) / 2, footerY + 8);
        pdf.text(footerDate, (pdfWidth - pdf.getTextWidth(footerDate)) / 2, footerY + 12);
      };
      
      // Calculate total pages needed
      const totalPages = Math.ceil(imgHeight / usableHeight);
      
      // Handle multi-page content with proper page breaks
      if (imgHeight <= usableHeight) {
        // Content fits on one page
        addHeader(1, 1);
        pdf.addImage(imgData, 'PNG', margin, margin + headerHeight, imgWidth, imgHeight);
        addFooter(1, 1);
      } else {
        // Content spans multiple pages - split the image properly
        // Use PNG format for better quality and to preserve all image data
        let remainingHeight = imgHeight;
        let sourceY = 0;
        let pageNum = 1;
        
        while (remainingHeight > 0) {
          if (pageNum > 1) {
            pdf.addPage();
          }
          
          addHeader(pageNum, totalPages);
          
          // Calculate how much content fits on this page
          let pageContentHeight = Math.min(usableHeight, remainingHeight);
          
          // Check if we're about to split through a table
          // Convert sourceY (in mm) to pixels in the original HTML
          const containerHeight = tempDiv.scrollHeight || tempDiv.offsetHeight;
          const scaleFactor = canvas.height / containerHeight;
          
          const sourceYInPixels = sourceY / (imgHeight / canvas.height);
          const pageEndInPixels = sourceYInPixels + (pageContentHeight / (imgHeight / canvas.height));
          
          // Check each table position to see if we're cutting through it
          for (const tablePos of tablePositions) {
            const tableTopInCanvas = tablePos.top * scaleFactor;
            const tableBottomInCanvas = tablePos.bottom * scaleFactor;
            const tableHeightInCanvas = tablePos.height * scaleFactor;
            
            // Add a buffer zone (10% of table height) to avoid cutting too close
            const buffer = tableHeightInCanvas * 0.1;
            const safeTableTop = tableTopInCanvas - buffer;
            const safeTableBottom = tableBottomInCanvas + buffer;
            
            // If the split point is within a table or its buffer zone
            if (sourceYInPixels < safeTableBottom && pageEndInPixels > safeTableTop) {
              // We're cutting through a table - adjust to avoid it
              if (sourceYInPixels < safeTableTop) {
                // Start of page is before table, but end cuts through it
                // Reduce page content height to end before table
                const maxHeightBeforeTable = safeTableTop - sourceYInPixels;
                const maxHeightInMm = (maxHeightBeforeTable / scaleFactor) * (imgHeight / canvas.height);
                if (maxHeightInMm > 30) { // Only adjust if we have at least 30mm of space
                  pageContentHeight = Math.min(pageContentHeight, maxHeightInMm);
                } else {
                  // Not enough space, move to next page to start table on new page
                  const newSourceY = (safeTableTop / scaleFactor) * (imgHeight / canvas.height);
                  sourceY = Math.max(sourceY, newSourceY);
                  remainingHeight = imgHeight - sourceY;
                  pageContentHeight = Math.min(usableHeight, remainingHeight);
                }
              } else if (sourceYInPixels >= safeTableTop && sourceYInPixels < safeTableBottom) {
                // We're starting in the middle of a table - move to next page
                const newSourceY = (safeTableBottom / scaleFactor) * (imgHeight / canvas.height);
                sourceY = newSourceY;
                remainingHeight = imgHeight - sourceY;
                pageContentHeight = Math.min(usableHeight, remainingHeight);
              }
              break; // Only handle first table we encounter
            }
          }
          
          // Calculate the source Y position in pixels (ensure we don't go beyond canvas)
          const sourceYpx = Math.min(
            Math.floor((sourceY / imgHeight) * canvas.height),
            canvas.height - 1
          );
          const sourceHeightPx = Math.min(
            Math.ceil((pageContentHeight / imgHeight) * canvas.height),
            canvas.height - sourceYpx
          );
          
          // Ensure we have valid dimensions
          if (sourceHeightPx > 0 && sourceYpx < canvas.height) {
            // Create a temporary canvas for this page's slice
            const pageCanvas = document.createElement('canvas');
            pageCanvas.width = canvas.width;
            pageCanvas.height = sourceHeightPx;
            const pageCtx = pageCanvas.getContext('2d');
            
            if (pageCtx) {
              // Set white background for the slice
              pageCtx.fillStyle = '#ffffff';
              pageCtx.fillRect(0, 0, pageCanvas.width, pageCanvas.height);
              
              // Draw the slice from the original canvas with better quality
              pageCtx.imageSmoothingEnabled = true;
              pageCtx.imageSmoothingQuality = 'high';
              
              pageCtx.drawImage(
                canvas,
                0, sourceYpx, canvas.width, sourceHeightPx, // Source rectangle
                0, 0, canvas.width, sourceHeightPx // Destination rectangle
              );
              
              // Use PNG format to preserve all image data
              const pageImgData = pageCanvas.toDataURL('image/png', 1.0);
              
              // Add this page's content to PDF using PNG format
              pdf.addImage(pageImgData, 'PNG', margin, margin + headerHeight, imgWidth, pageContentHeight);
            }
          }
          
          addFooter(pageNum, totalPages);
          
          // Move to next page
          sourceY += pageContentHeight;
          remainingHeight -= pageContentHeight;
          pageNum++;
        }
      }

      // Convert PDF to blob - use arraybuffer for better compatibility
      const pdfArrayBuffer = pdf.output('arraybuffer');
      
      // Validate PDF (should start with %PDF)
      if (pdfArrayBuffer.byteLength === 0) {
        throw new Error('PDF generation failed: empty PDF');
      }
      
      const pdfBytes = new Uint8Array(pdfArrayBuffer);
      const pdfHeader = String.fromCharCode(...pdfBytes.slice(0, 4));
      if (pdfHeader !== '%PDF') {
        console.error('Invalid PDF header:', pdfHeader);
        throw new Error('Invalid PDF generated');
      }
      
      const pdfBlob = new Blob([pdfArrayBuffer], { type: 'application/pdf' });
      
      // Verify blob size
      if (pdfBlob.size === 0) {
        throw new Error('PDF blob is empty');
      }
      
      // Use provided name or generate default name
      const fileName = pdfName.trim() || `chat-report-${conversationId}-${new Date().toISOString().replace(/[:.]/g, '-').slice(0, -5)}`;
      const pdfFileName = fileName.endsWith('.pdf') ? fileName : `${fileName}.pdf`;
      
      const pdfFile = new File([pdfBlob], pdfFileName, {
        type: 'application/pdf',
        lastModified: Date.now(),
      });
      
      // Final validation
      if (pdfFile.size === 0) {
        throw new Error('PDF file is empty');
      }
      
      // Store PDF in local storage for quick access
      try {
        const pdfDataUrl = await new Promise<string>((resolve, reject) => {
          const reader = new FileReader();
          reader.onload = () => resolve(reader.result as string);
          reader.onerror = reject;
          reader.readAsDataURL(pdfBlob);
        });
        
        // Store in localStorage with conversation ID as key
        const timestamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, -5);
        const storageKey = `chat-pdf-${conversationId}-${timestamp}`;
        localStorage.setItem(storageKey, pdfDataUrl);
        
        // Also store metadata
        const pdfMetadata = {
          conversationId,
          timestamp,
          fileName: pdfFile.name,
          size: pdfFile.size,
          storageKey,
        };
        localStorage.setItem(`${storageKey}-meta`, JSON.stringify(pdfMetadata));
        
        console.log('PDF stored in local storage:', storageKey);
      } catch (storageError) {
        console.warn('Failed to store PDF in local storage:', storageError);
        // Continue even if localStorage fails
      }
      
      console.log('PDF generated successfully:', {
        size: pdfFile.size,
        type: pdfFile.type,
        name: pdfFile.name,
      });

      // Upload to Resources - always save to Reports folder
      // For super admin, use selectedOrgId; for regular users, use their org_id
      const orgId = userInfo?.is_super_admin ? selectedOrgId : userInfo?.org_id;
      
      if (!orgId) {
        setError('Organization ID is required. Please select an organization.');
        return;
      }

      // Find or create Reports folder
      const reportsFolderId = await findOrCreateReportsFolder(orgId);
      if (!reportsFolderId) {
        setError('Failed to find or create Reports folder. Please try again.');
        return;
      }
      
      console.log('Uploading PDF file:', {
        name: pdfFile.name,
        size: pdfFile.size,
        type: pdfFile.type,
        orgId,
        folderId: reportsFolderId,
      });

      // API call: POST /api/v1/documents/upload (insti-inquora)
      // This will save to Reports folder (skips AI processing)
      const uploadResponse: any = await saasApi.uploadFile(pdfFile, reportsFolderId, orgId);
      
      console.log('Upload response:', uploadResponse);

      // Handle both response formats: { file: {...} } or { data: { file: {...} } } or direct file object
      let fileData = uploadResponse?.file || uploadResponse?.data?.file || uploadResponse?.data || uploadResponse;
      
      if (!fileData || (!fileData.id && !fileData.document_id)) {
        throw new Error('Upload failed: No file returned from server');
      }

      // Normalize file ID (could be 'id' or 'document_id')
      const uploadedFileId = fileData.id || fileData.document_id;
      const uploadedFileName = fileData.name || fileData.filename || pdfFile.name;
      const uploadedFileSize = fileData.size_bytes || fileData.size;
      const uploadedStorageKey = fileData.storage_key || fileData.file_path;

      console.log('PDF uploaded successfully:', {
        fileId: uploadedFileId,
        fileName: uploadedFileName,
        fileSize: uploadedFileSize,
        storageKey: uploadedStorageKey,
      });

      // Store the file ID in localStorage for quick access
      try {
        const fileAccessKey = `chat-pdf-file-${conversationId}`;
        localStorage.setItem(fileAccessKey, JSON.stringify({
          fileId: uploadedFileId,
          fileName: uploadedFileName,
          uploadedAt: new Date().toISOString(),
        }));
      } catch (storageError) {
        console.warn('Failed to store file ID in localStorage:', storageError);
      }

      // Ask user if they want to download the PDF
      // Automatically download the PDF without showing popup
        try {
          // API call: GET /api/v1/documents/{id}/download (insti-inquora)
          const blob = await saasApi.downloadFile(uploadedFileId);
          const url = window.URL.createObjectURL(blob);
          const link = document.createElement('a');
          link.href = url;
          link.download = uploadedFileName || 'chat-report.pdf';
          document.body.appendChild(link);
          link.click();
          document.body.removeChild(link);
          window.URL.revokeObjectURL(url);
        } catch (downloadErr: any) {
          console.error('Error downloading PDF:', downloadErr);
          alert('Failed to download PDF. You can access it from the Resources tab.');
      }
      
      onClose();
    } catch (err: any) {
      setError(err.message || 'Failed to save report');
      console.error('Error saving report:', err);
    } finally {
      setSaving(false);
    }
  };

  const isSuperAdmin = userInfo?.is_super_admin || false;

  return (
    <Dialog open={true} onOpenChange={onClose}>
      <DialogContent className="max-w-md p-6">
        <DialogHeader className="mb-4">
          <DialogTitle className="text-xl font-semibold">Save PDF Report</DialogTitle>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
            Select a folder to save the PDF report
          </p>
        </DialogHeader>
        {error && (
          <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-3 text-red-700 dark:text-red-400 mb-4 text-sm">
            {error}
          </div>
        )}
        <div className="space-y-4">
          {/* Organization selector for super admin */}
          {isSuperAdmin && organizations.length > 0 && (
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                Select Organization *
              </label>
              <select
                value={selectedOrgId}
                onChange={(e) => setSelectedOrgId(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-400 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100"
              >
                {organizations.map((org) => (
                  <option key={org.id} value={org.id}>
                    {org.name} {org.legal_name ? `(${org.legal_name})` : ''}
                  </option>
                ))}
              </select>
              <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                Select an organization to save the report to its resources
              </p>
            </div>
          )}

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Report Name *
            </label>
            <input
              type="text"
              value={pdfName}
              onChange={(e) => setPdfName(e.target.value)}
              placeholder="Enter report name (e.g., Q4 Analysis Report)"
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-400 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100"
              disabled={saving}
            />
            <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
              The report will be saved to the "Reports" folder
            </p>
          </div>

          <div className="flex justify-end gap-3 pt-4">
            <Button type="button" onClick={onClose} variant="outline" disabled={saving} className="bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 font-medium border border-gray-300 dark:border-gray-400 rounded-lg">
              Cancel
            </Button>
            <Button 
              onClick={handleSave} 
              disabled={saving || loading || (isSuperAdmin && !selectedOrgId) || !pdfName.trim()}
              className="bg-blue-600 hover:bg-blue-700 text-white font-medium"
            >
              {saving ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin mr-2" />
                  Saving...
                </>
              ) : (
                'Save Report'
              )}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}


