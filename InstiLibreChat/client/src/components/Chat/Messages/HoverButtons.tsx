import React, { useState, useMemo, memo } from 'react';
import { useRecoilState } from 'recoil';
import { useParams } from 'react-router-dom';
import type { TConversation, TMessage, TFeedback } from 'librechat-data-provider';
import { EditIcon, Clipboard, CheckMark, ContinueIcon, RegenerateIcon } from '@librechat/client';
import { FileText } from 'lucide-react';
import { useGenerationsByLatest, useLocalize } from '~/hooks';
import { useGetMessagesByConvoId } from '~/data-provider';
import { buildTree } from 'librechat-data-provider';
import { useFileMapContext } from '~/Providers';
import { Fork } from '~/components/Conversations';
import MessageAudio from './MessageAudio';
import Feedback from './Feedback';
import SavePDFModal from '../SavePDFModal';
import { cn } from '~/utils';
import store from '~/store';

type THoverButtons = {
  isEditing: boolean;
  enterEdit: (cancel?: boolean) => void;
  copyToClipboard: (setIsCopied: React.Dispatch<React.SetStateAction<boolean>>) => void;
  conversation: TConversation | null;
  isSubmitting: boolean;
  message: TMessage;
  regenerate: () => void;
  handleContinue: (e: React.MouseEvent<HTMLButtonElement>) => void;
  latestMessage: TMessage | null;
  isLast: boolean;
  index: number;
  handleFeedback?: ({ feedback }: { feedback: TFeedback | undefined }) => void;
};

type HoverButtonProps = {
  id?: string;
  onClick: (e?: React.MouseEvent<HTMLButtonElement>) => void;
  title: string;
  icon: React.ReactNode;
  isActive?: boolean;
  isVisible?: boolean;
  isDisabled?: boolean;
  isLast?: boolean;
  className?: string;
  buttonStyle?: string;
};

const extractMessageContent = (message: TMessage): string => {
  let rawContent = '';
  
  if (typeof message.content === 'string') {
    rawContent = message.content;
  } else if (Array.isArray(message.content)) {
    rawContent = message.content
      .map((part) => {
        if (part == null) {
          return '';
        }
        if (typeof part === 'string') {
          return part;
        }
        if ('text' in part) {
          return part.text || '';
        }
        if ('think' in part) {
          const think = part.think;
          if (typeof think === 'string') {
            return think;
          }
          return think && 'text' in think ? think.text || '' : '';
        }
        return '';
      })
      .join('');
  } else {
    rawContent = message.text || '';
  }

  // For user messages, extract only the query from structured JSON format
  if (message.isCreatedByUser && rawContent.trim().startsWith('{') && rawContent.includes('"query"')) {
    try {
      const parsed = JSON.parse(rawContent);
      if (parsed.query && typeof parsed.query === 'string') {
        return parsed.query;
      }
    } catch (e) {
      // If parsing fails, return original content
    }
  }

  return rawContent;
};

// Convert markdown tables to HTML tables
const convertMarkdownTablesToHTML = (content: string): string => {
  // More robust regex to match markdown tables
  // Matches tables that start with a header row, have a separator row, and data rows
  // Handles tables with or without leading/trailing pipes
  const tableRegex = /(?:^|\n)((?:\|?[^\n|]*\|[^\n|]*(?:\|[^\n|]*)*\|?\s*\n)+(?:\|?[\s-:|]+\|[\s-:|]*(?:\|[\s-:|]*)*\|?\s*\n)?(?:(?:\|?[^\n|]*\|[^\n|]*(?:\|[^\n|]*)*\|?\s*\n)+))/gm;
  
  return content.replace(tableRegex, (match) => {
    const lines = match.trim().split('\n').filter(line => {
      const trimmed = line.trim();
      // Filter out empty lines and lines that don't look like table rows
      return trimmed.length > 0 && trimmed.includes('|');
    });
    
    if (lines.length < 2) return match; // Need at least header and separator or data
    
    // Find separator line (line with dashes, colons, pipes)
    let separatorIndex = -1;
    for (let i = 0; i < lines.length; i++) {
      const line = lines[i].trim();
      // Check if line looks like a separator (mostly dashes, colons, pipes, spaces)
      if (/^[\s|:\-]+$/.test(line) && line.includes('-')) {
        separatorIndex = i;
        break;
      }
    }
    
    // Parse header (first line or line before separator)
    const headerLineIndex = separatorIndex > 0 ? separatorIndex - 1 : 0;
    const headerLine = lines[headerLineIndex];
    
    // Extract headers - handle both |header| and header|header formats
    const headerParts = headerLine.split('|').map(h => h.trim()).filter(h => h.length > 0);
    
    if (headerParts.length === 0) return match;
    
    // Data rows start after separator (if exists) or after header
    const dataStartIndex = separatorIndex >= 0 ? separatorIndex + 1 : headerLineIndex + 1;
    const dataLines = lines.slice(dataStartIndex);
    
    // Build HTML table wrapped in a container to prevent page breaks
    let html = '<div class="table-wrapper" style="page-break-inside: avoid; break-inside: avoid; margin: 12px 0;">';
    html += '<table><thead><tr>';
    headerParts.forEach((header, index) => {
      // Check if header suggests numeric column (contains numbers, %, etc.)
      const isNumericColumn = /[\d%₹]/.test(header);
      const align = isNumericColumn ? 'right' : 'left';
      html += `<th style="text-align: ${align}">${header}</th>`;
    });
    html += '</tr></thead><tbody>';
    
    dataLines.forEach(line => {
      const cells = line.split('|').map(c => c.trim()).filter(c => c.length > 0);
      
      // Only process if we have matching number of cells (or at least some cells)
      if (cells.length > 0) {
        html += '<tr>';
        cells.forEach((cell, index) => {
          // Check if cell contains numbers, percentages, currency symbols, or "NA"
          // Right-align numeric content, left-align text
          const cellTrimmed = cell.trim();
          const isNumeric = /^[\d,.\s%₹()\-]+$/.test(cellTrimmed) && 
                           cellTrimmed !== '' && 
                           cellTrimmed !== 'NA' &&
                           !/^[A-Za-z]+$/.test(cellTrimmed);
          const isNA = cellTrimmed === 'NA' || cellTrimmed === 'N/A';
          const align = isNumeric ? 'right' : 'left';
          
          // Escape HTML in cell content
          const escapedCell = cell
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#39;');
          
          html += `<td style="text-align: ${align}">${escapedCell}</td>`;
        });
        html += '</tr>';
      }
    });
    
    html += '</tbody></table></div>';
    return html;
  });
};

const HoverButton = memo(
  ({
    id,
    onClick,
    title,
    icon,
    isActive = false,
    isVisible = true,
    isDisabled = false,
    isLast = false,
    className = '',
  }: HoverButtonProps) => {
    const buttonStyle = cn(
      'hover-button rounded-lg p-1.5 text-text-secondary-alt transition-colors duration-200',
      'hover:text-text-primary hover:bg-surface-hover',
      'md:group-hover:visible md:group-focus-within:visible md:group-[.final-completion]:visible',
      !isLast && 'md:opacity-0 md:group-hover:opacity-100 md:group-focus-within:opacity-100',
      !isVisible && 'opacity-0',
      'focus-visible:ring-2 focus-visible:ring-black dark:focus-visible:ring-white focus-visible:outline-none',
      isActive && isVisible && 'active text-text-primary bg-surface-hover',
      className,
    );

    return (
      <button
        id={id}
        className={buttonStyle}
        onClick={onClick}
        type="button"
        title={title}
        disabled={isDisabled}
      >
        {icon}
      </button>
    );
  },
);

HoverButton.displayName = 'HoverButton';

const HoverButtons = ({
  index,
  isEditing,
  enterEdit,
  copyToClipboard,
  conversation,
  isSubmitting,
  message,
  regenerate,
  handleContinue,
  latestMessage,
  isLast,
  handleFeedback,
}: THoverButtons) => {
  const localize = useLocalize();
  const [isCopied, setIsCopied] = useState(false);
  const [TextToSpeech] = useRecoilState<boolean>(store.textToSpeech);
  const { conversationId } = useParams<{ conversationId?: string }>();
  const [showPDFModal, setShowPDFModal] = useState(false);
  const fileMap = useFileMapContext();

  const { data: messages } = useGetMessagesByConvoId(conversationId ?? '', {
    enabled: !!conversationId && conversationId !== 'new' && isLast,
  });

  const endpoint = useMemo(() => {
    if (!conversation) {
      return '';
    }
    return conversation.endpointType ?? conversation.endpoint;
  }, [conversation]);

  const generationCapabilities = useGenerationsByLatest({
    isEditing,
    isSubmitting,
    error: message.error,
    endpoint: endpoint ?? '',
    messageId: message.messageId,
    searchResult: message.searchResult,
    finish_reason: message.finish_reason,
    isCreatedByUser: message.isCreatedByUser,
    latestMessageId: latestMessage?.messageId,
  });

  const {
    hideEditButton,
    regenerateEnabled,
    continueSupported,
    forkingSupported,
    isEditableEndpoint,
  } = generationCapabilities;

  if (!conversation) {
    return null;
  }

  const { isCreatedByUser, error } = message;

  if (error === true) {
    return (
      <div className="visible flex justify-center self-end lg:justify-start">
        {regenerateEnabled && (
          <HoverButton
            onClick={regenerate}
            title={localize('com_ui_regenerate')}
            icon={<RegenerateIcon size="19" />}
            isLast={isLast}
          />
        )}
      </div>
    );
  }

  const onEdit = () => {
    if (isEditing) {
      return enterEdit(true);
    }
    enterEdit();
  };

  const handleCopy = () => copyToClipboard(setIsCopied);

  return (
    <div className="group visible flex justify-center gap-0.5 self-end focus-within:outline-none lg:justify-start">
      {/* Text to Speech */}
      {TextToSpeech && (
        <MessageAudio
          index={index}
          isLast={isLast}
          messageId={message.messageId}
          content={extractMessageContent(message)}
          renderButton={(props) => (
            <HoverButton
              onClick={props.onClick}
              title={props.title}
              icon={props.icon}
              isActive={props.isActive}
              isLast={isLast}
            />
          )}
        />
      )}

      {/* Copy Button */}
      <HoverButton
        onClick={handleCopy}
        title={
          isCopied ? localize('com_ui_copied_to_clipboard') : localize('com_ui_copy_to_clipboard')
        }
        icon={isCopied ? <CheckMark className="h-[18px] w-[18px]" /> : <Clipboard size="19" />}
        isLast={isLast}
        className={`ml-0 flex items-center gap-1.5 text-xs ${isSubmitting && isCreatedByUser ? 'md:opacity-0 md:group-hover:opacity-100' : ''}`}
      />

      {/* Edit Button */}
      {isEditableEndpoint && (
        <HoverButton
          id={`edit-${message.messageId}`}
          onClick={onEdit}
          title={localize('com_ui_edit')}
          icon={<EditIcon size="19" />}
          isActive={isEditing}
          isVisible={!hideEditButton}
          isDisabled={hideEditButton}
          isLast={isLast}
          className={isCreatedByUser ? '' : 'active'}
        />
      )}

      {/* Fork Button */}
      <Fork
        messageId={message.messageId}
        conversationId={conversation.conversationId}
        forkingSupported={forkingSupported}
        latestMessageId={latestMessage?.messageId}
        isLast={isLast}
      />

      {/* Feedback Buttons */}
      {!isCreatedByUser && handleFeedback != null && (
        <Feedback handleFeedback={handleFeedback} feedback={message.feedback} isLast={isLast} />
      )}

      {/* Regenerate Button */}
      {regenerateEnabled && (
        <HoverButton
          onClick={regenerate}
          title={localize('com_ui_regenerate')}
          icon={<RegenerateIcon size="19" />}
          isLast={isLast}
          className="active"
        />
      )}

      {/* Generate PDF Report Button - Show on all messages to generate PDF for selected chat */}
      {conversationId && conversationId !== 'new' && messages && messages.length > 0 && (
        <HoverButton
          onClick={() => setShowPDFModal(true)}
          title="Generate PDF Report"
          icon={<FileText size="19" />}
          isLast={isLast}
          className="active"
        />
      )}

      {/* Continue Button */}
      {continueSupported && (
        <HoverButton
          onClick={(e) => e && handleContinue(e)}
          title={localize('com_ui_continue')}
          icon={<ContinueIcon className="w-19 h-19 -rotate-180" />}
          isLast={isLast}
          className="active"
        />
      )}

      {/* PDF Modal */}
      {showPDFModal && conversationId && messages && (
        <SavePDFModal
          conversationId={conversationId}
          pdfContent={generatePDFContentFromSelectedMessage(message, messages, fileMap)}
          onClose={() => setShowPDFModal(false)}
        />
      )}
    </div>
  );
};

// Generate PDF content from selected message only (not entire conversation)
function generatePDFContentFromSelectedMessage(selectedMessage: TMessage, allMessages: TMessage[], fileMap: any): string {
    if (!selectedMessage || !allMessages || allMessages.length === 0) return '';
    
    // Find the selected message and its related messages (parent/children)
    const selectedMessageId = selectedMessage.messageId;
    const selectedMessages: TMessage[] = [];
    
    // If it's a user message, include it and its response (assistant message)
    if (selectedMessage.isCreatedByUser) {
      selectedMessages.push(selectedMessage);
      // Find the assistant response (next message that's not created by user)
      const selectedIndex = allMessages.findIndex(m => m.messageId === selectedMessageId);
      if (selectedIndex >= 0 && selectedIndex < allMessages.length - 1) {
        const nextMessage = allMessages[selectedIndex + 1];
        if (!nextMessage.isCreatedByUser) {
          selectedMessages.push(nextMessage);
        }
      }
    } else {
      // If it's an assistant message, include it and its parent (user message)
      selectedMessages.push(selectedMessage);
      // Find the parent user message (previous message that is created by user)
      const selectedIndex = allMessages.findIndex(m => m.messageId === selectedMessageId);
      if (selectedIndex > 0) {
        const prevMessage = allMessages[selectedIndex - 1];
        if (prevMessage.isCreatedByUser) {
          selectedMessages.unshift(prevMessage); // Add at beginning
        }
      }
    }
    
    if (selectedMessages.length === 0) {
      selectedMessages.push(selectedMessage); // Fallback: just the selected message
    }
    
    const messagesTree = buildTree({ messages: selectedMessages, fileMap });
    
    // Get current date for report
    const reportDate = new Date().toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'long',
      day: 'numeric',
    });
    
    let html = `
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>Fyers - Chat Conversation Report</title>
  <style>
    * {
      margin: 0;
      padding: 0;
      box-sizing: border-box;
    }
    
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
      line-height: 1.7;
      color: #1f2937;
      background-color: #ffffff;
      padding: 0;
      margin: 0;
      width: 800px;
    }
    
    .content-wrapper {
      width: 100%;
      margin: 0;
      padding: 0 15px;
    }
    
    .message {
      margin-bottom: 20px;
      padding: 18px 20px;
      border-radius: 8px;
      box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08);
      page-break-inside: avoid;
      break-inside: avoid;
    }
    
    .user-message {
      background-color: #f8fafc;
      border-left: 5px solid #3b82f6;
      border-top: 1px solid #e2e8f0;
    }
    
    .assistant-message {
      background-color: #f0fdf4;
      border-left: 5px solid #10b981;
      border-top: 1px solid #d1fae5;
    }
    
    .message-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: 12px;
      padding-bottom: 8px;
      border-bottom: 1px solid rgba(0, 0, 0, 0.08);
    }
    
    .message-header .role {
      font-weight: 700;
      font-size: 14px;
      text-transform: uppercase;
      letter-spacing: 0.5px;
      color: #475569;
    }
    
    .user-message .message-header .role {
      color: #3b82f6;
    }
    
    .assistant-message .message-header .role {
      color: #10b981;
    }
    
    .message-header .timestamp {
      font-size: 12px;
      color: #64748b;
      font-weight: 400;
    }
    
    .message-content {
      color: #1e293b;
      font-size: 14px;
      line-height: 1.8;
    }
    
    .message-content p {
      margin-bottom: 12px;
    }
    
    .message-content p:last-child {
      margin-bottom: 0;
    }
    
    .message-content pre {
      background-color: #1e293b;
      color: #f1f5f9;
      padding: 16px;
      border-radius: 8px;
      overflow-x: auto;
      font-size: 13px;
      line-height: 1.6;
      margin: 12px 0;
      font-family: 'Courier New', Courier, monospace;
    }
    
    .message-content code {
      background-color: #f1f5f9;
      color: #1e293b;
      padding: 3px 8px;
      border-radius: 4px;
      font-size: 13px;
      font-family: 'Courier New', Courier, monospace;
      border: 1px solid #e2e8f0;
    }
    
    .message-content pre code {
      background-color: transparent;
      color: inherit;
      padding: 0;
      border: none;
    }
    
    .message-content img {
      max-width: 100%;
      height: auto;
      border-radius: 4px;
      margin: 12px 0;
      page-break-inside: avoid;
      break-inside: avoid;
      page-break-after: avoid;
      break-after: avoid;
      display: block;
      object-fit: contain;
    }
    
    .message-content ul,
    .message-content ol {
      margin-left: 24px;
      margin-bottom: 12px;
    }
    
    .message-content li {
      margin-bottom: 6px;
    }
    
    .message-content blockquote {
      border-left: 4px solid #cbd5e1;
      padding-left: 16px;
      margin: 12px 0;
      color: #64748b;
      font-style: italic;
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
      min-width: 100%;
      word-wrap: break-word;
    }
    
    .message-content .table-wrapper {
      width: 100%;
      page-break-inside: avoid;
      break-inside: avoid;
      page-break-after: auto;
      break-after: auto;
      margin: 12px 0;
      display: block;
    }
    
    .message-content table-container {
      width: 100%;
      overflow-x: auto;
      margin: 12px 0;
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
    
    .message-content table th[style*="text-align: right"] {
      text-align: right !important;
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
      white-space: normal;
      word-wrap: break-word;
    }
    
    .message-content table td[style*="text-align: right"] {
      text-align: right !important;
      font-variant-numeric: tabular-nums;
      font-feature-settings: "tnum";
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
    
  </style>
</head>
<body>
  <div class="content-wrapper">
`;

    const formatMessage = (message: TMessage): string => {
      const isUser = message.isCreatedByUser || (message as any).role === 'user' || (message as any).sender === 'User';
      const className = isUser ? 'user-message' : 'assistant-message';
      const role = isUser ? 'User' : 'Assistant';
      const timestamp = message.createdAt 
        ? new Date(message.createdAt).toLocaleString('en-US', {
            year: 'numeric',
            month: 'short',
            day: 'numeric',
            hour: '2-digit',
            minute: '2-digit',
          })
        : '';
      
      // Use the existing extractMessageContent function to properly handle content
      let content = extractMessageContent(message);
      
      // Ensure content is a string before calling replace
      if (typeof content !== 'string') {
        content = String(content || '');
      }
      
      // Check if content already contains HTML tables (from ReactMarkdown)
      const hasHTMLTables = /<table[\s>]/.test(content);
      
      // Only convert markdown tables if no HTML tables are present
      if (!hasHTMLTables) {
        // Convert markdown tables to HTML tables
        content = convertMarkdownTablesToHTML(content);
      }
      
      // Enhanced markdown to HTML conversion
      content = content
        // Headers
        .replace(/^### (.*$)/gim, '<h3>$1</h3>')
        .replace(/^## (.*$)/gim, '<h2>$1</h2>')
        .replace(/^# (.*$)/gim, '<h1>$1</h1>')
        // Bold
        .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
        .replace(/__(.*?)__/g, '<strong>$1</strong>')
        // Italic
        .replace(/\*(.*?)\*/g, '<em>$1</em>')
        .replace(/_(.*?)_/g, '<em>$1</em>')
        // Inline code
        .replace(/`([^`]+)`/g, '<code>$1</code>')
        // Code blocks (handle before line breaks)
        .replace(/```([\s\S]*?)```/g, '<pre><code>$1</code></pre>')
        // Line breaks
        .replace(/\n\n/g, '</p><p>')
        .replace(/\n/g, '<br>');

      // Wrap content in paragraph if it doesn't start with a tag
      if (!content.trim().startsWith('<')) {
        content = '<p>' + content + '</p>';
      }

      return `
    <div class="message ${className}">
      <div class="message-header">
        <span class="role">${role}</span>
        ${timestamp ? `<span class="timestamp">${timestamp}</span>` : ''}
      </div>
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
  </div>
</body>
</html>
`;

    return html;
}

export default memo(HoverButtons);
