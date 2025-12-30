import { memo, Suspense, useMemo } from 'react';
import { useRecoilValue } from 'recoil';
import { DelayedRender } from '@librechat/client';
import type { TMessage } from 'librechat-data-provider';
import type { TMessageContentProps, TDisplayProps } from '~/common';
import Error from '~/components/Messages/Content/Error';
import { useMessageContext } from '~/Providers';
import MarkdownLite from './MarkdownLite';
import EditMessage from './EditMessage';
import Thinking from './Parts/Thinking';
import { useLocalize } from '~/hooks';
import Container from './Container';
import Markdown from './Markdown';
import Files from './Files';
import { cn } from '~/utils';
import store from '~/store';

const ERROR_CONNECTION_TEXT = 'Error connecting to server, try refreshing the page.';
const DELAYED_ERROR_TIMEOUT = 5500;
const UNFINISHED_DELAY = 250;

const parseThinkingContent = (text: string) => {
  const thinkingMatch = text.match(/:::thinking([\s\S]*?):::/);
  return {
    thinkingContent: thinkingMatch ? thinkingMatch[1].trim() : '',
    regularContent: thinkingMatch ? text.replace(/:::thinking[\s\S]*?:::/, '').trim() : text,
  };
};

const LoadingFallback = () => (
  <div className="text-message mb-[0.625rem] flex min-h-[20px] flex-col items-start gap-3 overflow-visible">
    <div className="markdown prose dark:prose-invert light w-full break-words dark:text-gray-100">
      <div className="absolute">
        <p className="submitting relative">
          <span className="result-thinking" />
        </p>
      </div>
    </div>
  </div>
);

const ErrorBox = ({
  children,
  className = '',
}: {
  children: React.ReactNode;
  className?: string;
}) => (
  <div
    role="alert"
    aria-live="assertive"
    className={cn(
      'rounded-xl border border-red-500/20 bg-red-500/5 px-3 py-2 text-sm text-gray-600 dark:text-gray-200',
      className,
    )}
  >
    {children}
  </div>
);

const ConnectionError = ({ message }: { message?: TMessage }) => {
  const localize = useLocalize();

  return (
    <Suspense fallback={<LoadingFallback />}>
      <DelayedRender delay={DELAYED_ERROR_TIMEOUT}>
        <Container message={message}>
          <div className="mt-2 rounded-xl border border-red-500/20 bg-red-50/50 px-4 py-3 text-sm text-red-700 shadow-sm transition-all dark:bg-red-950/30 dark:text-red-100">
            {localize('com_ui_error_connection')}
          </div>
        </Container>
      </DelayedRender>
    </Suspense>
  );
};

export const ErrorMessage = ({
  text,
  message,
  className = '',
}: Pick<TDisplayProps, 'text' | 'className'> & { message?: TMessage }) => {
  if (text === ERROR_CONNECTION_TEXT) {
    return <ConnectionError message={message} />;
  }

  return (
    <Container message={message}>
      <ErrorBox className={className}>
        <Error text={text} />
      </ErrorBox>
    </Container>
  );
};

const DisplayMessage = ({ text, isCreatedByUser, message, showCursor }: TDisplayProps) => {
  const { isSubmitting = false, isLatestMessage = false } = useMessageContext();
  const enableUserMsgMarkdown = useRecoilValue(store.enableUserMsgMarkdown);

  const showCursorState = useMemo(
    () => showCursor === true && isSubmitting,
    [showCursor, isSubmitting],
  );

  const content = useMemo(() => {
    // Parse new structured JSON format: { documents: [...], template: {...}, persona: {...}, query: "..." }
    if (isCreatedByUser && text.trim().startsWith('{')) {
      let documentNames = '';
      let userQuery = '';
      let templateName = '';
      let personaName = '';
      
      try {
        // Try to parse as JSON
        const requestObject = JSON.parse(text);
        
        // Check if this is our structured format (has query field)
        if (requestObject.query !== undefined && requestObject.query !== null) {

          // Extract documents
          if (requestObject.documents && Array.isArray(requestObject.documents) && requestObject.documents.length > 0) {
            documentNames = requestObject.documents.map((doc: any) => doc.name).join(', ');
          }
          
          // Extract query
          userQuery = requestObject.query;
          
          // Extract template name (for display)
          if (requestObject.template && requestObject.template.name) {
            templateName = requestObject.template.name;
          }
          
          // Extract persona name (for display)
          if (requestObject.persona && requestObject.persona.name) {
            personaName = requestObject.persona.name;
          }
        } else {
          // Not our format, show as-is
          userQuery = text;
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
        </>
      );
    }
    
    if (!isCreatedByUser) {
      return <Markdown content={text} isLatestMessage={isLatestMessage} />;
    }
    if (enableUserMsgMarkdown) {
      return <MarkdownLite content={text} />;
    }
    return <>{text}</>;
  }, [isCreatedByUser, enableUserMsgMarkdown, text, isLatestMessage]);

  // Extract metadata for user messages
  const metadata = useMemo(() => {
    if (!isCreatedByUser) return null;
    
    let documentNames = '';
    let personaName = '';
    let templateName = '';
    
    // Parse new structured JSON format
    if (text.trim().startsWith('{') && (text.includes('"documents"') || text.includes('"query"'))) {
      try {
        const requestObject = JSON.parse(text);
        
        if (requestObject.documents && Array.isArray(requestObject.documents) && requestObject.documents.length > 0) {
          documentNames = requestObject.documents.map((doc: any) => doc.name).join(', ');
        }
        
        if (requestObject.template && requestObject.template.name) {
          templateName = requestObject.template.name;
        }
        
        if (requestObject.persona && requestObject.persona.name) {
          personaName = requestObject.persona.name;
        }
      } catch (e) {
        // Continue to fallback parsing
      }
    }
    
    // Parse request block format
    if (!documentNames && !personaName && !templateName && text.includes('request: {')) {
      const documentsMatch = text.match(/Documents:\s*\[\s*\{\s*name:"([^"]*)"[^}]*\}/s);
      if (documentsMatch) {
        documentNames = documentsMatch[1];
      }
      
      // Extract persona name from request block
      const personaMatch = text.match(/persona:\s*"((?:[^"\\]|\\.)*)"/s);
      if (personaMatch && personaMatch[1].trim()) {
        const personaContent = personaMatch[1].replace(/\\n/g, '\n').replace(/\\"/g, '"');
        // Try to extract just the name (first line typically contains the name)
        personaName = personaContent.split('\n')[0].trim() || 'Persona';
      }
      
      // Extract template name from request block
      const templateMatch = text.match(/template:\s*"((?:[^"\\]|\\.)*)"/s);
      if (templateMatch && templateMatch[1].trim()) {
        const templateContent = templateMatch[1].replace(/\\n/g, '\n').replace(/\\"/g, '"');
        // Try to extract just the name (first line typically contains the name)
        templateName = templateContent.split('\n')[0].trim() || 'Template';
      }
    }
    
    // Parse Document{} format
    if (!documentNames && text.includes('Document{')) {
      const documentBlockMatch = text.match(/Document\{[^}]*\}/s);
      if (documentBlockMatch) {
        const nameMatch = documentBlockMatch[0].match(/name:"([^"]*)"/);
        if (nameMatch) {
          documentNames = nameMatch[1];
        }
      }
    }
    
    // Parse <documents> format
    if (!documentNames && text.includes('<documents>')) {
      const documentsMatch = text.match(/<documents>(.*?)<\/documents>/s);
      if (documentsMatch) {
        documentNames = documentsMatch[1].trim();
      }
    }
    
    if (!documentNames && !personaName && !templateName) return null;
    
    return (
      <div 
        className="mt-2 flex flex-wrap gap-2 text-xs text-gray-500 dark:text-gray-400"
        style={{
          justifyContent: isCreatedByUser ? 'flex-end' : 'flex-start'
        }}
      >
        {documentNames && (
          <span className="flex items-center gap-1">
            <span>📄</span>
            <span>{documentNames}</span>
          </span>
        )}
        {personaName && (
          <span className="flex items-center gap-1">
            <span>👤</span>
            <span>{personaName}</span>
          </span>
        )}
        {templateName && (
          <span className="flex items-center gap-1">
            <span>📝</span>
            <span>{templateName}</span>
          </span>
        )}
      </div>
    );
  }, [isCreatedByUser, text]);

  return (
    <div
      className="text-message flex min-h-[20px] flex-col gap-3 overflow-visible [.text-message+&]:mt-5"
      style={{
        alignItems: isCreatedByUser ? 'flex-end' : 'flex-start'
      }}
      dir="auto"
    >
      <div
        className={cn(
          'markdown prose message-content dark:prose-invert light break-words',
          isSubmitting && 'submitting',
          showCursorState && text.length > 0 && 'result-streaming',
          isCreatedByUser && !enableUserMsgMarkdown && 'whitespace-pre-wrap',
          isCreatedByUser ? 'dark:text-gray-20' : 'dark:text-gray-100',
          isCreatedByUser && 'rounded-2xl px-4 py-3 bg-[#F7F7F7] dark:bg-[#222222] inline-block',
        )}
        style={{ width: isCreatedByUser ? 'fit-content' : '100%', maxWidth: '100%' }}
      >
        {content}
      </div>
      {metadata}
      {isCreatedByUser && <Files message={message} />}
    </div>
  );
};

export const UnfinishedMessage = ({ message }: { message: TMessage }) => (
  <ErrorMessage
    message={message}
    text="The response is incomplete; it's either still processing, was cancelled, or censored. Refresh or try a different prompt."
  />
);

const MessageContent = ({
  text,
  edit,
  error,
  unfinished,
  isSubmitting,
  isLast,
  ...props
}: TMessageContentProps) => {
  const { message } = props;
  const { messageId } = message;

  const { thinkingContent, regularContent } = useMemo(() => parseThinkingContent(text), [text]);
  const showRegularCursor = useMemo(() => isLast && isSubmitting, [isLast, isSubmitting]);

  const unfinishedMessage = useMemo(
    () =>
      !isSubmitting && unfinished ? (
        <Suspense>
          <DelayedRender delay={UNFINISHED_DELAY}>
            <UnfinishedMessage message={message} />
          </DelayedRender>
        </Suspense>
      ) : null,
    [isSubmitting, unfinished, message],
  );

  if (error) {
    return <ErrorMessage message={message} text={text} />;
  }

  if (edit) {
    return <EditMessage text={text} isSubmitting={isSubmitting} {...props} />;
  }

  return (
    <>
      {thinkingContent.length > 0 && (
        <Thinking key={`thinking-${messageId}`}>{thinkingContent}</Thinking>
      )}
      <DisplayMessage
        key={`display-${messageId}`}
        showCursor={showRegularCursor}
        text={regularContent}
        {...props}
      />
      {unfinishedMessage}
    </>
  );
};

export default memo(MessageContent);
