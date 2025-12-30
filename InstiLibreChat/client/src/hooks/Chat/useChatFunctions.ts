import { v4 } from 'uuid';
import { cloneDeep } from 'lodash';
import { useQueryClient } from '@tanstack/react-query';
import {
  Constants,
  QueryKeys,
  ContentTypes,
  EModelEndpoint,
  getEndpointField,
  isAgentsEndpoint,
  parseCompactConvo,
  replaceSpecialVars,
  isAssistantsEndpoint,
} from 'librechat-data-provider';
import { useSetRecoilState, useResetRecoilState, useRecoilValue } from 'recoil';
import type {
  TMessage,
  TSubmission,
  TConversation,
  TEndpointOption,
  TEndpointsConfig,
  EndpointSchemaKey,
} from 'librechat-data-provider';
import type { SetterOrUpdater } from 'recoil';
import type { TAskFunction, ExtendedFile } from '~/common';
import useSetFilesToDelete from '~/hooks/Files/useSetFilesToDelete';
import useGetSender from '~/hooks/Conversations/useGetSender';
import store, { useGetEphemeralAgent } from '~/store';
import useUserKey from '~/hooks/Input/useUserKey';
import { useNavigate } from 'react-router-dom';
import { useAuthContext } from '~/hooks';
import { logger } from '~/utils';

const logChatRequest = (request: Record<string, unknown>) => {
  logger.log('=====================================\nAsk function called with:');
  logger.dir(request);
  logger.log('=====================================');
};

export default function useChatFunctions({
  index = 0,
  files,
  setFiles,
  getMessages,
  setMessages,
  isSubmitting,
  latestMessage,
  setSubmission,
  setLatestMessage,
  conversation: immutableConversation,
}: {
  index?: number;
  isSubmitting: boolean;
  paramId?: string | undefined;
  conversation: TConversation | null;
  latestMessage: TMessage | null;
  getMessages: () => TMessage[] | undefined;
  setMessages: (messages: TMessage[]) => void;
  files?: Map<string, ExtendedFile>;
  setFiles?: SetterOrUpdater<Map<string, ExtendedFile>>;
  setSubmission: SetterOrUpdater<TSubmission | null>;
  setLatestMessage?: SetterOrUpdater<TMessage | null>;
}) {
  const navigate = useNavigate();
  const getSender = useGetSender();
  const { user } = useAuthContext();
  const queryClient = useQueryClient();
  const setFilesToDelete = useSetFilesToDelete();
  const getEphemeralAgent = useGetEphemeralAgent();
  const isTemporary = useRecoilValue(store.isTemporary);
  const { getExpiry } = useUserKey(immutableConversation?.endpoint ?? '');
  const setShowStopButton = useSetRecoilState(store.showStopButtonByIndex(index));
  const resetLatestMultiMessage = useResetRecoilState(store.latestMessageFamily(index + 1));

  const ask: TAskFunction = (
    {
      text,
      overrideConvoId,
      overrideUserMessageId,
      parentMessageId = null,
      conversationId = null,
      messageId = null,
    },
    {
      editedContent = null,
      editedMessageId = null,
      isRegenerate = false,
      isContinued = false,
      isEdited = false,
      overrideMessages,
      overrideFiles,
    } = {},
  ) => {
    setShowStopButton(false);
    resetLatestMultiMessage();
    if (!!isSubmitting || text === '') {
      return;
    }

    const conversation = cloneDeep(immutableConversation);

    const endpoint = conversation?.endpoint;
    if (endpoint === null) {
      console.error('No endpoint available');
      return;
    }

    conversationId = conversationId ?? conversation?.conversationId ?? null;
    if (conversationId == 'search') {
      console.error('cannot send any message under search view!');
      return;
    }

    if (isContinued && !latestMessage) {
      console.error('cannot continue AI message without latestMessage!');
      return;
    }

    const ephemeralAgent = getEphemeralAgent(conversationId ?? Constants.NEW_CONVO);
    const isEditOrContinue = isEdited || isContinued;

    let currentMessages: TMessage[] | null = overrideMessages ?? getMessages() ?? [];

    if (conversation?.promptPrefix) {
      conversation.promptPrefix = replaceSpecialVars({
        text: conversation.promptPrefix,
        user,
      });
    }

    // construct the query message
    // this is not a real messageId, it is used as placeholder before real messageId returned
    text = text.trim();
    const intermediateId = overrideUserMessageId ?? v4();
    parentMessageId = parentMessageId ?? latestMessage?.messageId ?? Constants.NO_PARENT;

    logChatRequest({
      index,
      conversation,
      latestMessage,
      conversationId,
      intermediateId,
      parentMessageId,
      currentMessages,
    });

    if (conversationId == Constants.NEW_CONVO) {
      parentMessageId = Constants.NO_PARENT;
      currentMessages = [];
      conversationId = null;
      navigate('/c/new', { state: { focusChat: true } });
    }

    const targetParentMessageId = isRegenerate ? messageId : latestMessage?.parentMessageId;
    /**
     * If the user regenerated or resubmitted the message, the current parent is technically
     * the latest user message, which is passed into `ask`; otherwise, we can rely on the
     * latestMessage to find the parent.
     */
    const targetParentMessage = currentMessages.find(
      (msg) => msg.messageId === targetParentMessageId,
    );

    let thread_id = targetParentMessage?.thread_id ?? latestMessage?.thread_id;
    if (thread_id == null) {
      thread_id = currentMessages.find((message) => message.thread_id)?.thread_id;
    }

    const endpointsConfig = queryClient.getQueryData<TEndpointsConfig>([QueryKeys.endpoints]);
    const endpointType = getEndpointField(endpointsConfig, endpoint, 'type');
    const iconURL = conversation?.iconURL;

    /** This becomes part of the `endpointOption` */
    const convo = parseCompactConvo({
      endpoint: endpoint as EndpointSchemaKey,
      endpointType: endpointType as EndpointSchemaKey,
      conversation: conversation ?? {},
    });

    const { modelDisplayLabel } = endpointsConfig?.[endpoint ?? ''] ?? {};
    const endpointOption = Object.assign(
      {
        endpoint,
        endpointType,
        overrideConvoId,
        overrideUserMessageId,
      },
      convo,
    ) as TEndpointOption;
    if (endpoint !== EModelEndpoint.agents) {
      endpointOption.key = getExpiry();
      endpointOption.thread_id = thread_id;
      endpointOption.modelDisplayLabel = modelDisplayLabel;
    } else {
      endpointOption.key = new Date(Date.now() + 60 * 60 * 1000).toISOString();
    }
    const responseSender = getSender({ model: conversation?.model, ...endpointOption });

    const currentMsg: TMessage = {
      text,
      sender: 'User',
      clientTimestamp: new Date().toLocaleString('sv').replace(' ', 'T'),
      isCreatedByUser: true,
      parentMessageId,
      conversationId,
      messageId: isContinued && messageId != null && messageId ? messageId : intermediateId,
      thread_id,
      error: false,
    };

    const submissionFiles = overrideFiles ?? targetParentMessage?.files;
    const reuseFiles =
      (isRegenerate || (overrideFiles != null && overrideFiles.length)) &&
      submissionFiles &&
      submissionFiles.length > 0;

    if (setFiles && reuseFiles === true) {
      currentMsg.files = [...submissionFiles];
      setFiles(new Map());
      setFilesToDelete({});
    } else if (setFiles && files && files.size > 0) {
      currentMsg.files = Array.from(files.values()).map((file) => ({
        file_id: file.file_id,
        filepath: file.filepath,
        type: file.type ?? '', // Ensure type is not undefined
        height: file.height,
        width: file.width,
      }));
      setFiles(new Map());
      setFilesToDelete({});
    }

    const responseMessageId =
      editedMessageId ??
      (latestMessage?.messageId && isRegenerate
        ? latestMessage.messageId.replace(/_+$/, '') + '_'
        : null) ??
      null;
    const initialResponseId =
      responseMessageId ?? `${isRegenerate ? messageId : intermediateId}`.replace(/_+$/, '') + '_';

    const initialResponse: TMessage = {
      sender: responseSender,
      text: '',
      endpoint: endpoint ?? '',
      parentMessageId: isRegenerate ? messageId : intermediateId,
      messageId: initialResponseId,
      thread_id,
      conversationId,
      unfinished: false,
      isCreatedByUser: false,
      model: convo?.model,
      error: false,
      iconURL,
    };

    if (isAssistantsEndpoint(endpoint)) {
      initialResponse.model = conversation?.assistant_id ?? '';
      initialResponse.text = '';
      initialResponse.content = [
        {
          type: ContentTypes.TEXT,
          [ContentTypes.TEXT]: {
            value: '',
          },
        },
      ];
    } else if (endpoint != null) {
      initialResponse.model = isAgentsEndpoint(endpoint)
        ? (conversation?.agent_id ?? '')
        : (conversation?.model ?? '');
      initialResponse.text = '';

      if (editedContent && latestMessage?.content) {
        initialResponse.content = cloneDeep(latestMessage.content);
        const { index, type, ...part } = editedContent;
        if (initialResponse.content && index >= 0 && index < initialResponse.content.length) {
          const contentPart = initialResponse.content[index];
          if (type === ContentTypes.THINK && contentPart.type === ContentTypes.THINK) {
            contentPart[ContentTypes.THINK] = part[ContentTypes.THINK];
          } else if (type === ContentTypes.TEXT && contentPart.type === ContentTypes.TEXT) {
            contentPart[ContentTypes.TEXT] = part[ContentTypes.TEXT];
          }
        }
      } else {
        initialResponse.content = [
          {
            type: ContentTypes.TEXT,
            [ContentTypes.TEXT]: {
              value: '',
            },
          },
        ];
      }
      setShowStopButton(true);
    }

    if (isContinued) {
      currentMessages = currentMessages.filter((msg) => msg.messageId !== responseMessageId);
    }

    logger.log('message_state', initialResponse);

    // Get persona and template data from localStorage
    const convoId = conversationId ?? Constants.NEW_CONVO;
    const storedPersonaData = localStorage.getItem(`persona_data_${convoId}`);
    const storedTemplateData = localStorage.getItem(`template_data_${convoId}`);
    
    // Get document data and build document prompt
    let documentPrompt = '';
    let documentsList: any[] = [];
    let enhancedEphemeralAgent = ephemeralAgent ? { ...ephemeralAgent } : {};
    const documentDataStr = localStorage.getItem(`persona_documents_${convoId}`);
    
    if (documentDataStr) {
      try {
        const documentData = JSON.parse(documentDataStr);
        console.log('[DOCUMENT SEARCH] Raw documentData from localStorage:', documentData);
        
        if (documentData.documents && documentData.documents.length > 0) {
          documentsList = documentData.documents;
          const documentNames = documentsList.map((doc: any) => doc.filename).join(', ');
          // documentPrompt = `Documents:\n${documentNames}`;
          
          // Always prepare ephemeral agent with document search info when documents are selected
          // Extract document_ids from the documents list - CRITICAL: document_id must be passed correctly
          const documentIds = documentsList.map((doc: any) => {
            // Log each document to debug
            console.log(`[DOCUMENT SEARCH] Processing document:`, {
              filename: doc.filename,
              document_id: doc.document_id,
              document_id_type: typeof doc.document_id,
              raw_doc: doc
            });
            
            // Ensure document_id is a number
            const docId = typeof doc.document_id === 'number' ? doc.document_id : parseInt(doc.document_id, 10);
            if (isNaN(docId)) {
              console.error(`[DOCUMENT SEARCH] ❌ Invalid document_id: ${doc.document_id} (type: ${typeof doc.document_id}) for document ${doc.filename}`);
              return null;
            }
            return docId;
          }).filter((id: any) => id !== null && !isNaN(id));
          
          console.log('[DOCUMENT SEARCH] ✅ Extracted document_ids (will be used as collection IDs):', documentIds);
          console.log('[DOCUMENT SEARCH] Document IDs details:', {
            count: documentIds.length,
            ids: documentIds,
            types: documentIds.map(id => typeof id),
            first_id: documentIds[0],
            first_id_type: typeof documentIds[0],
          });
          
          if (documentIds.length === 0) {
            console.error('[DOCUMENT SEARCH] ❌ No valid document_ids found! Cannot pass collection to MCP.');
          } else {
            // Ensure all document_ids are numbers
            const numericDocumentIds = documentIds.map((id: any) => {
              const numId = typeof id === 'number' ? id : parseInt(String(id), 10);
              if (isNaN(numId)) {
                console.error(`[DOCUMENT SEARCH] ❌ Invalid document_id in array: ${id} (type: ${typeof id})`);
                return null;
              }
              return numId;
            }).filter((id: any) => id !== null && !isNaN(id));
            
            if (numericDocumentIds.length === 0) {
              console.error('[DOCUMENT SEARCH] ❌ No valid numeric document_ids after conversion!');
          } else {
            enhancedEphemeralAgent = {
              ...(ephemeralAgent || {}),
              // Add document search capability to MCP tools
              mcp: [...(ephemeralAgent?.mcp || []), 'document_search'].filter((v, i, a) => a.indexOf(v) === i), // Remove duplicates
              // @ts-ignore - Adding documentSearch property for MCP
              documentSearch: {
                enabled: true,
                documents: documentsList.map((doc: any) => doc.filename),
                  document_ids: numericDocumentIds, // Pass document IDs as numbers for MCP collection parameter
                selected_files: documentsList.map((doc: any) => doc.filename) // Add this for MCP compatibility
              }
            };
            
            // Log document search configuration
            console.log('%c[DOCUMENT SEARCH] ✅ Enabling document search:', 'color: #4CAF50; font-weight: bold;');
            console.log('Documents:', documentsList.map((doc: any) => ({
              filename: doc.filename,
              document_id: doc.document_id,
              document_id_type: typeof doc.document_id
            })));
              console.log('%c[DOCUMENT SEARCH] Document IDs (collection IDs) to pass to MCP:', 'color: #FF6B6B; font-weight: bold;', numericDocumentIds);
              console.log('%c[DOCUMENT SEARCH] First document_id (will be used as collection):', 'color: #FF6B6B; font-weight: bold;', numericDocumentIds[0]);
            console.log('EphemeralAgent documentSearch:', {
              enabled: true,
                document_ids: numericDocumentIds,
                first_document_id: numericDocumentIds[0],
              documents: documentsList.map((doc: any) => doc.filename)
            });
              console.log('Full enhancedEphemeralAgent:', JSON.stringify(enhancedEphemeralAgent, null, 2));
            }
          }
        }
      } catch (error) {
        console.error('[DOCUMENT SEARCH] ❌ Error parsing document data:', error);
      }
    }

    // Helper function to parse template structure from content and framework
    const parseTemplateStructure = (templateData: any) => {
      const framework = templateData.framework || '';
      const content = templateData.content || {};
      const guidelines: string[] = [];
      
      // If we have framework-specific content fields, extract them
      if (framework === 'T-A-G' && content) {
        // Extract T (Task), A (Action), G (Goal) from content
        // Check both uppercase keys (T, A, G) and lowercase keys (task, action, goal)
        const task = content.T || content.task || '';
        const action = content.A || content.action || '';
        const goal = content.G || content.goal || '';
        
        // ALWAYS use the actual template descriptions if they exist
        // Make it very explicit that these are the actual instructions, not generic definitions
        if (task || action || goal) {
          // Add header to make it crystal clear
          guidelines.push('T-A-G Framework Instructions (Task-Action-Goal):');
          guidelines.push('The following are the EXACT instructions for this template. Do NOT interpret T-A-G as theme-answer-gist or any other variation.');
          guidelines.push('');
          
          if (task) {
            guidelines.push(`T = Task: ${task}`);
          }
          if (action) {
            guidelines.push(`A = Action: ${action}`);
          }
          if (goal) {
            guidelines.push(`G = Goal: ${goal}`);
          }
          
          guidelines.push('');
          guidelines.push('CRITICAL: The assistant MUST use these EXACT descriptions above. T means Task (as defined above), A means Action (as defined above), G means Goal (as defined above).');
          guidelines.push('Always format with proper sections, no raw metadata, no unnecessary prefixes.');
          
          return {
            format: 'T-A-G',
            guidelines: guidelines
          };
        }
      } else if (framework === 'R-T-F' && content) {
        // Extract R (Role), T (Task), F (Format) from content
        const role = content.R || content.role || '';
        const task = content.T || content.task || '';
        const format = content.F || content.format || '';
        
        if (role) {
          guidelines.push(`R = Role: ${role}`);
        }
        if (task) {
          guidelines.push(`T = Task: ${task}`);
        }
        if (format) {
          guidelines.push(`F = Format: ${format}`);
        }
        
        if (guidelines.length > 0) {
          return {
            format: 'R-T-F',
            guidelines: guidelines
          };
        }
      } else if (framework === 'B-A-B' && content) {
        // Extract B1 (Before), A (After), B2 (Bridge) from content
        const before = content.B1 || content.before || content.B || '';
        const after = content.A || content.after || '';
        const bridge = content.B2 || content.bridge || '';
        
        if (before) {
          guidelines.push(`B = Before: ${before}`);
        }
        if (after) {
          guidelines.push(`A = After: ${after}`);
        }
        if (bridge) {
          guidelines.push(`B = Bridge: ${bridge}`);
        }
        
        if (guidelines.length > 0) {
          return {
            format: 'B-A-B',
            guidelines: guidelines
          };
        }
      } else if (framework === 'C-A-R-E' && content) {
        // Extract C (Context), A (Action), R (Result), E (Example) from content
        const context = content.C || content.context || '';
        const action = content.A || content.action || '';
        const result = content.R || content.result || '';
        const example = content.E || content.example || '';
        
        if (context) {
          guidelines.push(`C = Context: ${context}`);
        }
        if (action) {
          guidelines.push(`A = Action: ${action}`);
        }
        if (result) {
          guidelines.push(`R = Result: ${result}`);
        }
        if (example) {
          guidelines.push(`E = Example: ${example}`);
        }
        
        if (guidelines.length > 0) {
          return {
            format: 'C-A-R-E',
            guidelines: guidelines
          };
        }
      } else if (framework === 'R-I-S-E' && content) {
        // Extract R (Role), I (Input), S (Steps), E (Expectation) from content
        const role = content.R || content.role || '';
        const input = content.I || content.input || '';
        const steps = content.S || content.steps || '';
        const expectation = content.E || content.expectation || '';
        
        if (role) {
          guidelines.push(`R = Role: ${role}`);
        }
        if (input) {
          guidelines.push(`I = Input: ${input}`);
        }
        if (steps) {
          guidelines.push(`S = Steps: ${steps}`);
        }
        if (expectation) {
          guidelines.push(`E = Expectation: ${expectation}`);
        }
        
        if (guidelines.length > 0) {
          return {
            format: 'R-I-S-E',
            guidelines: guidelines
          };
        }
      } else if (framework && content) {
        // For custom or unknown frameworks, extract all content fields
        Object.keys(content).forEach((key) => {
          if (key !== 'custom' && content[key]) {
            guidelines.push(`${key}: ${content[key]}`);
          }
        });
        
        if (guidelines.length > 0) {
          return {
            format: framework,
            guidelines: guidelines
          };
        }
      }
      
      // Fallback: try to parse from detailedPrompt if no content fields found
      const detailedPrompt = templateData.detailedPrompt || '';
      if (detailedPrompt) {
        const lines = detailedPrompt.split('\n').map(line => line.trim()).filter(line => line);
        let format = '';
        
        // Try to extract format (e.g., "T-A-G", "R-T-F")
        const formatMatch = detailedPrompt.match(/(?:format|Format|FORMAT)[:\s]*([A-Z\-]+)/i);
        if (formatMatch) {
          format = formatMatch[1];
        } else if (framework) {
          format = framework;
        } else {
          // Try to infer from structure
          const structureMatch = detailedPrompt.match(/([A-Z])\s*[-=]\s*([A-Z])\s*[-=]\s*([A-Z])/);
          if (structureMatch) {
            format = `${structureMatch[1]}-${structureMatch[2]}-${structureMatch[3]}`;
          }
        }
        
        // Extract guidelines from lines
        lines.forEach((line) => {
          if (line.match(/^[A-Z]\s*=\s*/) || line.match(/^[A-Z]\s*[-=]/)) {
            guidelines.push(line);
          } else if (line.toLowerCase().includes('guideline') || line.toLowerCase().includes('instruction')) {
            guidelines.push(line);
          }
        });
        
        if (format || guidelines.length > 0) {
          return {
            format: format || 'STRUCTURED',
            guidelines: guidelines.length > 0 ? guidelines : [
              'Follow the structure defined in the template.',
              'Present information clearly and concisely.',
              'Use proper formatting and sections.'
            ]
          };
        }
      }
      
      // Default structure if nothing found
      return {
        format: framework || 'STRUCTURED',
        guidelines: [
          'Follow the structure defined in response_structure_instructions.',
          'Present information clearly and concisely.',
          'Use proper formatting and sections.'
        ]
      };
    };
    
    // Helper function to parse persona instructions from detailedPrompt
    const parsePersonaInstructions = (detailedPrompt: string) => {
      if (!detailedPrompt) return null;
      
      let role = '';
      let voiceAndTone = '';
      const behavior: string[] = [];
      
      const lines = detailedPrompt.split('\n').map(line => line.trim()).filter(line => line);
      
      lines.forEach((line) => {
        const lowerLine = line.toLowerCase();
        if (lowerLine.includes('you are') || lowerLine.includes('role:')) {
          role = line.replace(/.*(?:you are|role:)[:\s]*/i, '').trim();
        } else if (lowerLine.includes('voice') || lowerLine.includes('tone')) {
          voiceAndTone = line.replace(/.*(?:voice|tone)[:\s]*/i, '').trim();
        } else if (lowerLine.includes('behavior') || lowerLine.includes('focus') || lowerLine.includes('use')) {
          behavior.push(line);
        }
      });
      
      // If no role found, try to extract from first line
      if (!role && lines.length > 0) {
        const firstLine = lines[0];
        if (firstLine.toLowerCase().includes('assistant') || firstLine.toLowerCase().includes('you are')) {
          role = firstLine;
        } else {
          role = firstLine.substring(0, 100); // Use first line as role
        }
      }
      
      // Default values if not found
      if (!voiceAndTone) {
        voiceAndTone = 'Professional, clear, and helpful';
      }
      
      if (behavior.length === 0) {
        behavior.push('Provide accurate and helpful responses.');
        behavior.push('Use evidence from documents when available.');
      }
      
      return {
        role: role || 'AI Assistant',
        voice_and_tone: voiceAndTone,
        behavior: behavior
      };
    };

    // Build the structured request object
    const requestObject: any = {};
    let structuredPrompt = '';
    
    // Build documents array
    if (documentsList.length > 0) {
      requestObject.documents = documentsList.map((doc: any) => {
        const docId = typeof doc.document_id === 'number' ? doc.document_id : parseInt(doc.document_id, 10);
        return {
          name: doc.filename,
          collection: isNaN(docId) ? null : docId
        };
      }).filter((doc: any) => doc.collection !== null);
    } else {
      requestObject.documents = [];
    }
    
    // Build template object
    if (storedTemplateData) {
      try {
        const templateData = JSON.parse(storedTemplateData);
        const framework = templateData.framework || '';
        
        // Build description with explicit framework explanation
        let description = templateData.description || 'Defines how the assistant must format, structure, and present the final answer.';
        if (framework === 'T-A-G') {
          description = 'Defines how the assistant must format, structure, and present the final answer. T-A-G stands for Task-Action-Goal. The assistant MUST strictly follow the EXACT instructions provided in response_structure_instructions.guidelines for T (Task), A (Action), and G (Goal). Do NOT interpret T-A-G as theme-answer-gist or any other variation. Use ONLY the specific descriptions provided in the guidelines.';
        } else if (framework) {
          description = `Defines how the assistant must format, structure, and present the final answer using the ${framework} framework. The assistant MUST strictly follow the EXACT instructions provided in response_structure_instructions.guidelines.`;
        } else {
          description = 'Defines how the assistant must format, structure, and present the final answer. The assistant MUST strictly follow the structure defined in response_structure_instructions.';
          }
        
        const templateObj: any = {
          name: templateData.name || templateData.template || 'Template',
          description: description
        };
        
        // Parse response_structure_instructions from template data (content and framework)
        const structureInstructions = parseTemplateStructure(templateData);
        
        if (structureInstructions) {
          templateObj.response_structure_instructions = structureInstructions;
        } else {
          // Default structure if parsing fails
          templateObj.response_structure_instructions = {
            format: framework || 'STRUCTURED',
            guidelines: [
              'Follow the structure defined in response_structure_instructions.',
              'Present information clearly and concisely.',
              'Use proper formatting and sections.'
            ]
          };
        }
        
        requestObject.template = templateObj;
      } catch (error) {
        console.error('Error parsing template data:', error);
      }
    }

    // Build persona object
    if (storedPersonaData) {
      try {
        const personaData = JSON.parse(storedPersonaData);
        const personaObj: any = {
          name: personaData.name || personaData.persona || 'Persona',
          description: personaData.description || 'Defines the role and behavioral style of the assistant.'
        };
        
        // Parse persona_instructions from detailedPrompt
        const detailedPrompt = personaData.detailedPrompt || personaData.content?.custom || '';
        const personaInstructions = parsePersonaInstructions(detailedPrompt);
        
        if (personaInstructions) {
          personaObj.persona_instructions = personaInstructions;
        } else {
          // Default persona if parsing fails
          personaObj.persona_instructions = {
            role: personaData.persona || personaData.name || 'AI Assistant',
            voice_and_tone: 'Professional, clear, and helpful',
            behavior: [
              'Provide accurate and helpful responses.',
              'Use evidence from documents when available.'
            ]
          };
            }
        
        requestObject.persona = personaObj;
      } catch (error) {
        console.error('Error parsing persona data:', error);
      }
    }
    
    // Add query
    requestObject.query = text;
    
    // Build the structured prompt as JSON string
    structuredPrompt = JSON.stringify(requestObject, null, 2);

    const submission: TSubmission = {
      conversation: {
        ...conversation,
        conversationId,
      },
      endpointOption,
      userMessage: {
        ...currentMsg,
        responseMessageId,
        overrideParentMessageId: isRegenerate ? messageId : null,
        // Send the full structured prompt as the main text
        text: structuredPrompt || text,
      },
      messages: currentMessages,
      isEdited: isEditOrContinue,
      isContinued,
      isRegenerate,
      initialResponse,
      isTemporary,
      ephemeralAgent: enhancedEphemeralAgent,
      editedContent,
    };

    if (isRegenerate) {
      setMessages([...submission.messages, initialResponse]);
    } else {
      setMessages([...submission.messages, currentMsg, initialResponse]);
    }
    if (index === 0 && setLatestMessage) {
      setLatestMessage(initialResponse);
    }

    setSubmission(submission);
    logger.dir('message_stream', submission, { depth: null });
  };

  const regenerate = ({ parentMessageId }) => {
    const messages = getMessages();
    const parentMessage = messages?.find((element) => element.messageId == parentMessageId);

    if (parentMessage && parentMessage.isCreatedByUser) {
      ask({ ...parentMessage }, { isRegenerate: true });
    } else {
      console.error(
        'Failed to regenerate the message: parentMessage not found or not created by user.',
      );
    }
  };

  return {
    ask,
    regenerate,
  };
}
