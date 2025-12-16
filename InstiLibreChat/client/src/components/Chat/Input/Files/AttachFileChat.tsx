import { memo } from 'react';
import { Constants } from 'librechat-data-provider';
import type { TConversation } from 'librechat-data-provider';
import UploadButton from './UploadButton';

function AttachFileChat({
  disableInputs,
  conversation,
}: {
  disableInputs: boolean;
  conversation: TConversation | null;
}) {
  const conversationId = conversation?.conversationId ?? Constants.NEW_CONVO;

  // Always show UploadButton for document uploads
  return <UploadButton disabled={disableInputs} conversationId={conversationId} />;
}

export default memo(AttachFileChat);
