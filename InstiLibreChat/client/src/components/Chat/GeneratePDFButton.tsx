import { useState } from 'react';
import { useParams } from 'react-router-dom';
import { FileText, Loader2 } from 'lucide-react';
import { Button } from '@librechat/client';
import { useGetMessagesByConvoId } from '~/data-provider';
import { buildTree } from 'librechat-data-provider';
import type { TMessage } from 'librechat-data-provider';
import { useFileMapContext } from '~/Providers';
import SavePDFModal from './SavePDFModal';

export default function GeneratePDFButton() {
  const { conversationId } = useParams<{ conversationId?: string }>();
  const [showModal, setShowModal] = useState(false);
  const [generating, setGenerating] = useState(false);
  const fileMap = useFileMapContext();

  const { data: messages } = useGetMessagesByConvoId(conversationId ?? '', {
    enabled: !!conversationId && conversationId !== 'new',
  });

  const handleGeneratePDF = async () => {
    if (!messages || messages.length === 0) {
      alert('No messages to generate PDF from');
      return;
    }
    setShowModal(true);
  };

  const generatePDFContent = (): string => {
    if (!messages || messages.length === 0) return '';
    
    const messagesTree = buildTree({ messages, fileMap });
    let html = `
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>Chat Conversation</title>
  <style>
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
      line-height: 1.6;
      color: #333;
      max-width: 800px;
      margin: 0 auto;
      padding: 20px;
    }
    .message {
      margin-bottom: 20px;
      padding: 15px;
      border-radius: 8px;
    }
    .user-message {
      background-color: #f0f0f0;
      border-left: 4px solid #3b82f6;
    }
    .assistant-message {
      background-color: #f9fafb;
      border-left: 4px solid #10b981;
    }
    .message-header {
      font-weight: 600;
      margin-bottom: 8px;
      color: #666;
      font-size: 14px;
    }
    .message-content {
      color: #1f2937;
    }
    .message-content pre {
      background-color: #f3f4f6;
      padding: 12px;
      border-radius: 4px;
      overflow-x: auto;
      font-size: 13px;
    }
    .message-content code {
      background-color: #f3f4f6;
      padding: 2px 6px;
      border-radius: 3px;
      font-size: 13px;
    }
    .message-content table {
      width: 100%;
      border-collapse: separate;
      border-spacing: 0;
      margin: 12px 0;
      font-size: 13px;
      table-layout: auto;
      display: table;
      border: 1px solid #d1d5db;
      border-radius: 0.5rem;
      overflow: hidden;
    }
    .message-content table thead {
      background-color: #f3f4f6;
    }
    .message-content table th {
      background-color: #f3f4f6;
      font-weight: 600;
      color: #1e293b;
      padding: 0.5rem 0.75rem;
      text-align: left;
      border: 1px solid #d1d5db;
      border-top: none;
      border-bottom: 1px solid #d1d5db;
      vertical-align: bottom;
    }
    .message-content table th:first-child {
      border-left: none;
    }
    .message-content table th:last-child {
      border-right: none;
    }
    .message-content table tbody tr {
      border-bottom: 1px solid #d1d5db;
    }
    .message-content table tbody tr:last-child {
      border-bottom: none;
    }
    .message-content table tbody tr:last-child td {
      border-bottom: none;
    }
    .message-content table td {
      padding: 0.5rem 0.75rem;
      border: 1px solid #d1d5db;
      border-top: none;
      border-left: 1px solid #d1d5db;
      text-align: left;
      vertical-align: baseline;
    }
    .message-content table td:first-child {
      border-left: none;
    }
    .message-content table td:last-child {
      border-right: none;
    }
    .message-content table tbody tr:last-child td:first-child {
      border-bottom-left-radius: 0.5rem;
    }
    .message-content table tbody tr:last-child td:last-child {
      border-bottom-right-radius: 0.5rem;
    }
    .message-content table tbody tr:hover {
      background-color: #f9fafb;
    }
    h1 {
      color: #111827;
      border-bottom: 2px solid #e5e7eb;
      padding-bottom: 10px;
      margin-bottom: 30px;
    }
  </style>
</head>
<body>
  <h1>Chat Conversation Report</h1>
`;

    const formatMessage = (message: TMessage, level = 0): string => {
      const isUser = (message as any).role === 'user' || (message as any).sender === 'User' || !(message as any).isCreatedByUser === false;
      const className = isUser ? 'user-message' : 'assistant-message';
      const header = isUser ? 'User' : 'Assistant';
      const timestamp = (message as any).createdAt 
        ? new Date((message as any).createdAt).toLocaleString()
        : '';
      
      let content = (message as any).text || (message as any).content || '';
      
      // For user messages, extract only the query from structured JSON format
      if ((message as any).isCreatedByUser && typeof content === 'string' && content.trim().startsWith('{') && content.includes('"query"')) {
        try {
          const parsed = JSON.parse(content);
          if (parsed.query && typeof parsed.query === 'string') {
            content = parsed.query;
          }
        } catch (e) {
          // If parsing fails, use original content
        }
      }
      
      // Basic markdown to HTML conversion
      content = content
        .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
        .replace(/\*(.*?)\*/g, '<em>$1</em>')
        .replace(/`([^`]+)`/g, '<code>$1</code>')
        .replace(/\n/g, '<br>');

      return `
  <div class="message ${className}">
    <div class="message-header">${header}${timestamp ? ` - ${timestamp}` : ''}</div>
    <div class="message-content">${content}</div>
  </div>
`;
    };

    const traverseMessages = (msgs: TMessage[]) => {
      msgs.forEach((msg) => {
        html += formatMessage(msg);
        if (msg.children && msg.children.length > 0) {
          traverseMessages(msg.children);
        }
      });
    };

    if (messagesTree) {
      traverseMessages(messagesTree);
    }

    html += `
</body>
</html>
`;

    return html;
  };

  if (!conversationId || conversationId === 'new' || !messages || messages.length === 0) {
    return null;
  }

  return (
    <>
      <Button
        onClick={handleGeneratePDF}
        disabled={generating}
        className="flex items-center gap-2"
        variant="outline"
        size="sm"
      >
        {generating ? (
          <>
            <Loader2 className="h-4 w-4 animate-spin" />
            Generating...
          </>
        ) : (
          <>
            <FileText className="h-4 w-4" />
            Generate PDF Report
          </>
        )}
      </Button>

      {showModal && (
        <SavePDFModal
          conversationId={conversationId}
          pdfContent={generatePDFContent()}
          onClose={() => setShowModal(false)}
        />
      )}
    </>
  );
}

