import { memo, useMemo, ReactElement } from 'react';
import { useRecoilValue } from 'recoil';
import MarkdownLite from '~/components/Chat/Messages/Content/MarkdownLite';
import Markdown from '~/components/Chat/Messages/Content/Markdown';
import { useMessageContext } from '~/Providers';
import { cn } from '~/utils';
import store from '~/store';

type TextPartProps = {
  text: string;
  showCursor: boolean;
  isCreatedByUser: boolean;
};

type ContentType =
  | ReactElement<React.ComponentProps<typeof Markdown>>
  | ReactElement<React.ComponentProps<typeof MarkdownLite>>
  | ReactElement;

const TextPart = memo(({ text, isCreatedByUser, showCursor }: TextPartProps) => {
  const { isSubmitting = false, isLatestMessage = false } = useMessageContext();
  const enableUserMsgMarkdown = useRecoilValue(store.enableUserMsgMarkdown);
  const showCursorState = useMemo(() => showCursor && isSubmitting, [showCursor, isSubmitting]);

  const content: ContentType = useMemo(() => {
    // Parse new structured JSON format: { documents: [...], template: {...}, persona: {...}, query: "..." }
    if (isCreatedByUser && (text.trim().startsWith('{') && (text.includes('"documents"') || text.includes('"query"')))) {
      let documentNames = '';
      let userQuery = '';
      let templateName = '';
      let personaName = '';
      
      try {
        // Try to parse as JSON
        const requestObject = JSON.parse(text);
        
        // Extract documents
        if (requestObject.documents && Array.isArray(requestObject.documents) && requestObject.documents.length > 0) {
          documentNames = requestObject.documents.map((doc: any) => doc.name).join(', ');
        }
        
        // Extract query
        if (requestObject.query) {
          userQuery = requestObject.query;
        }
        
        // Extract template description (for display)
        if (requestObject.template && requestObject.template.description) {
          templateName = requestObject.template.description.substring(0, 50);
          if (requestObject.template.description.length > 50) templateName += '...';
        }
        
        // Extract persona description (for display)
        if (requestObject.persona && requestObject.persona.description) {
          personaName = requestObject.persona.description.substring(0, 50);
          if (requestObject.persona.description.length > 50) personaName += '...';
        }
      } catch (e) {
        // Fallback: try to parse old format
        if (text.includes('request: {')) {
          // Extract Documents array - handle multiline format
          const documentsMatch = text.match(/Documents:\s*\[\s*\{\s*name:"([^"]*)"[^}]*\}/s);
          if (documentsMatch) {
            documentNames = documentsMatch[1];
          }
          
          // Extract persona from request block: persona: "..."
          const personaMatch = text.match(/persona:\s*"((?:[^"\\]|\\.)*)"/s);
          if (personaMatch && personaMatch[1].trim()) {
            // Try to extract persona name from the content (first line or name field)
            const personaContent = personaMatch[1].replace(/\\n/g, '\n').replace(/\\"/g, '"');
            const firstLine = personaContent.split('\n')[0].trim();
            personaName = firstLine || 'Persona';
          }
          
          // Extract template from request block: template: "..."
          const templateMatch = text.match(/template:\s*"((?:[^"\\]|\\.)*)"/s);
          if (templateMatch && templateMatch[1].trim()) {
            // Try to extract template name from the content (first line or name field)
            const templateContent = templateMatch[1].replace(/\\n/g, '\n').replace(/\\"/g, '"');
            const firstLine = templateContent.split('\n')[0].trim();
            templateName = firstLine || 'Template';
          }
          
          // Extract query from request block: query: "..."
          const queryMatch = text.match(/query:\s*"((?:[^"\\]|\\.)*)"/s);
          if (queryMatch) {
            // Unescape the query (handle \n and \")
            userQuery = queryMatch[1].replace(/\\n/g, '\n').replace(/\\"/g, '"');
          } else {
            // Fallback: try to extract from old format
            const userPromptMatch = text.match(/User Prompt:\s*(.*)/s);
            if (userPromptMatch) {
              userQuery = userPromptMatch[1].trim();
            } else {
              userQuery = text.replace(/request:\s*\{[^}]*\}/s, '').trim();
            }
          }
        } else {
          // If parsing fails completely, use text as-is
          userQuery = text;
        }
      }
      
      return (
        <>
          {userQuery && (
            enableUserMsgMarkdown ? (
              <MarkdownLite content={userQuery} />
            ) : (
              <>{userQuery}</>
            )
          )}
          {(documentNames || personaName || templateName) && (
            <div className="mt-2 flex flex-wrap gap-2 text-xs text-gray-500 dark:text-gray-400">
              {documentNames && (
                <span className="flex items-center gap-1">
                  <span>📄</span>
                  <span>{documentNames}</span>
                </span>
              )}
              {personaName && (
                <span className="flex items-center gap-1">

                  <span>{personaName}</span>
                </span>
              )}
              {templateName && (
                <span className="flex items-center gap-1">

                  <span>{templateName}</span>
                </span>
              )}
            </div>
          )}
        </>
      );
    }
    
    // Fallback: Parse old Document block format if present
    if (isCreatedByUser && text.includes('Document{')) {
      // Extract Document block using regex
      const documentBlockMatch = text.match(/Document\{[^}]*\}/s);
      let documentNames = '';
      
      if (documentBlockMatch) {
        // Extract name from Document block: name:"...";
        const nameMatch = documentBlockMatch[0].match(/name:"([^"]*)"/);
        if (nameMatch) {
          documentNames = nameMatch[1];
        }
      }
      
      // Extract user query (after "User Prompt:")
      const userPromptMatch = text.match(/User Prompt:\s*(.*)/s);
      const userQuery = userPromptMatch ? userPromptMatch[1].trim() : text.replace(/Document\{[^}]*\}/s, '').replace(/User Prompt:\s*/s, '').trim();
      
      return (
        <>
          {userQuery && (
            enableUserMsgMarkdown ? (
              <MarkdownLite content={userQuery} />
            ) : (
              <>{userQuery}</>
            )
          )}
          {documentNames && (
            <div className="mt-2 text-xs text-gray-500 dark:text-gray-400">
              {documentNames}
            </div>
          )}
        </>
      );
    }
    
    // Fallback: Parse old <documents> format if present
    if (isCreatedByUser && text.includes('<documents>')) {
      // Extract documents using regex
      const documentsMatch = text.match(/<documents>(.*?)<\/documents>/s);
      const documents = documentsMatch ? documentsMatch[1].trim() : '';
      
      // Remove the documents tag from text to get user query
      const userQuery = text.replace(/<documents>.*?<\/documents>/s, '').trim();
      
      return (
        <>
          {userQuery && (
            enableUserMsgMarkdown ? (
              <MarkdownLite content={userQuery} />
            ) : (
              <>{userQuery}</>
            )
          )}
          {documents && (
            <div className="mt-2 text-xs text-gray-500 dark:text-gray-400">
              {documents}
            </div>
          )}
        </>
      );
    }
    
    if (!isCreatedByUser) {
      return <Markdown content={text} isLatestMessage={isLatestMessage} />;
    } else if (enableUserMsgMarkdown) {
      return <MarkdownLite content={text} />;
    } else {
      return <>{text}</>;
    }
  }, [isCreatedByUser, enableUserMsgMarkdown, text, isLatestMessage]);

  return (
    <div
      className={cn(
        isSubmitting ? 'submitting' : '',
        showCursorState && !!text.length ? 'result-streaming' : '',
        'markdown prose message-content dark:prose-invert light w-full break-words',
        isCreatedByUser && !enableUserMsgMarkdown && 'whitespace-pre-wrap',
        isCreatedByUser ? 'dark:text-gray-20' : 'dark:text-gray-100',
      )}
    >
      {content}
    </div>
  );
});

export default TextPart;
