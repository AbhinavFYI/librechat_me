import React, { useMemo, memo } from 'react';
import { getEndpointField } from 'librechat-data-provider';
import type { Assistant, Agent } from 'librechat-data-provider';
import type { TMessageIcon } from '~/common';
import ConvoIconURL from '~/components/Endpoints/ConvoIconURL';
import { useGetEndpointsQuery } from '~/data-provider';
import { getIconEndpoint, logger } from '~/utils';
import { useChatContext } from '~/Providers';
import Icon from '~/components/Endpoints/Icon';

const MessageIcon = memo(
  ({
    iconData,
    assistant,
    agent,
    isSubmitting = false,
  }: {
    iconData?: TMessageIcon;
    assistant?: Assistant;
    agent?: Agent;
    isSubmitting?: boolean;
  }) => {
    const { data: endpointsConfig } = useGetEndpointsQuery();

    const agentName = useMemo(() => agent?.name ?? '', [agent]);
    const agentAvatar = useMemo(() => agent?.avatar?.filepath ?? '', [agent]);
    const assistantName = useMemo(() => assistant?.name ?? '', [assistant]);
    const assistantAvatar = useMemo(() => assistant?.metadata?.avatar ?? '', [assistant]);

    const avatarURL = useMemo(() => {
      let result = '';
      if (assistant) {
        result = assistantAvatar;
      } else if (agent) {
        result = agentAvatar;
      }
      return result;
    }, [assistant, agent, assistantAvatar, agentAvatar]);

    const iconURL = iconData?.iconURL;
    const endpoint = useMemo(
      () => getIconEndpoint({ endpointsConfig, iconURL, endpoint: iconData?.endpoint }),
      [endpointsConfig, iconURL, iconData?.endpoint],
    );

    const endpointIconURL = useMemo(
      () => getEndpointField(endpointsConfig, endpoint, 'iconURL'),
      [endpointsConfig, endpoint],
    );

    // Show loading animation for AI messages that are being generated
    // Use chatIsSubmitting from ChatContext as fallback since this component
    // is rendered outside MessageContext
    const { isSubmitting: chatIsSubmitting } = useChatContext();
    const effectiveIsSubmitting = isSubmitting || chatIsSubmitting;
    const shouldShowLoader = effectiveIsSubmitting && iconData?.isCreatedByUser === false;

    if (shouldShowLoader) {
      return (
        <div className="flex h-full w-full items-center justify-center">
          <img
            src="/assets/loader.gif"
            alt="Loading..."
            className="object-contain"
            style={{ width: '32px', height: '32px' }}
          />
        </div>
      );
    }

    if (iconData?.isCreatedByUser !== true && iconURL != null && iconURL.includes('http')) {
      return (
        <ConvoIconURL
          iconURL={iconURL}
          modelLabel={iconData?.modelLabel}
          context="message"
          assistantAvatar={assistantAvatar}
          agentAvatar={agentAvatar}
          endpointIconURL={endpointIconURL}
          assistantName={assistantName}
          agentName={agentName}
        />
      );
    }

    return (
      <Icon
        isCreatedByUser={iconData?.isCreatedByUser ?? false}
        endpoint={endpoint}
        iconURL={avatarURL || endpointIconURL}
        model={iconData?.model}
        assistantName={assistantName}
        agentName={agentName}
        size={32}
      />
    );
  },
);

MessageIcon.displayName = 'MessageIcon';

export default MessageIcon;
