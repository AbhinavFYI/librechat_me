import { memo } from 'react';
import { Feather } from 'lucide-react';
import { EModelEndpoint, isAssistantsEndpoint, alternateName } from 'librechat-data-provider';
import {
  Plugin,
  GPTIcon,
  PaLMIcon,
  CodeyIcon,
  GeminiIcon,
  BedrockIcon,
  AssistantIcon,
  AnthropicIcon,
  AzureMinimalIcon,
  CustomMinimalIcon,
} from '@librechat/client';
import UnknownIcon from '~/hooks/Endpoint/UnknownIcon';
import FIAIcon from '~/components/Endpoints/FIAIcon';
import { IconProps } from '~/common';
import { cn } from '~/utils';

type EndpointIcon = {
  icon: React.ReactNode | React.JSX.Element;
  bg?: string;
  name?: string | null;
};

function getOpenAIColor(_model: string | null | undefined) {
  const model = _model?.toLowerCase() ?? '';
  if (model && (/\b(o\d)\b/i.test(model) || /\bgpt-[5-9](?:\.\d+)?\b/i.test(model))) {
    return '#000000';
  }
  return model.includes('gpt-4') ? '#AB68FF' : '#19C37D';
}

function getGoogleIcon(model: string | null | undefined, size: number) {
  if (model?.toLowerCase().includes('code') === true) {
    return <CodeyIcon size={size * 0.75} />;
  } else if (/gemini|learnlm|gemma/.test(model?.toLowerCase() ?? '')) {
    return <GeminiIcon size={size * 0.7} />;
  } else {
    return <PaLMIcon size={size * 0.7} />;
  }
}

function getGoogleModelName(model: string | null | undefined) {
  if (model?.toLowerCase().includes('code') === true) {
    return 'Codey';
  } else if (
    model?.toLowerCase().includes('gemini') === true ||
    model?.toLowerCase().includes('learnlm') === true
  ) {
    return 'Gemini';
  } else if (model?.toLowerCase().includes('gemma') === true) {
    return 'Gemma';
  } else {
    return 'PaLM2';
  }
}

const MessageEndpointIcon: React.FC<IconProps> = (props) => {
  const {
    error,
    button,
    iconURL = '',
    endpoint,
    size = 30,
    model = '',
    assistantName,
    agentName,
  } = props;

  const assistantsIcon = {
    icon: iconURL ? (
      <div className="relative flex items-center justify-center" style={{ width: size, height: size }}>
        <div
          title={assistantName}
          style={{
            width: size,
            height: size,
          }}
          className={cn('overflow-hidden rounded-full', props.className ?? '')}
        >
          <img
            className="shadow-stroke h-full w-full object-cover"
            src={iconURL}
            alt={assistantName}
            style={{ height: '80', width: '80' }}
          />
        </div>
      </div>
    ) : (
      <div style={{ width: size, height: size }}>
        <div className="shadow-stroke flex items-center justify-center overflow-hidden rounded-full" style={{ width: size, height: size }}>
          <AssistantIcon className="h-2/3 w-2/3 text-gray-400" />
        </div>
      </div>
    ),
    name: endpoint,
  };

  const agentsIcon = {
    icon: iconURL ? (
      <div className="relative flex items-center justify-center" style={{ width: size, height: size }}>
        <div
          title={agentName}
          style={{
            width: size,
            height: size,
          }}
          className={cn('overflow-hidden rounded-full', props.className ?? '')}
        >
          <img
            className="shadow-stroke h-full w-full object-cover"
            src={iconURL}
            alt={agentName}
            style={{ height: '80', width: '80' }}
          />
        </div>
      </div>
    ) : (
      <div style={{ width: size, height: size }}>
        <div className="shadow-stroke flex items-center justify-center overflow-hidden rounded-full" style={{ width: size, height: size }}>
          <Feather className="h-2/3 w-2/3 text-gray-400" />
        </div>
      </div>
    ),
    name: endpoint,
  };

  const endpointIcons: {
    [key: string]: EndpointIcon | undefined;
  } = {
    [EModelEndpoint.assistants]: assistantsIcon,
    [EModelEndpoint.agents]: agentsIcon,
    [EModelEndpoint.azureAssistants]: assistantsIcon,
    [EModelEndpoint.azureOpenAI]: {
      icon: <FIAIcon size={size} />,
      bg: 'transparent',
      name: 'ChatGPT',
    },
    [EModelEndpoint.openAI]: {
      icon: <FIAIcon size={size} />,
      bg: 'transparent',
      name: 'ChatGPT',
    },
    [EModelEndpoint.gptPlugins]: {
      icon: <Plugin size={size * 0.7} />,
      bg: `rgba(69, 89, 164, ${button === true ? 0.75 : 1})`,
      name: 'Plugins',
    },
    [EModelEndpoint.google]: {
      icon: getGoogleIcon(model, size),
      name: getGoogleModelName(model),
    },
    [EModelEndpoint.anthropic]: {
      icon: <FIAIcon size={size} />,
      bg: 'transparent',
      name: 'Claude',
    },
    [EModelEndpoint.bedrock]: {
      icon: <FIAIcon size={size} />,
      bg: 'transparent',
      name: 'FIA',
    },
    [EModelEndpoint.custom]: {
      icon: <CustomMinimalIcon size={size * 0.7} />,
      name: 'Custom',
    },
    null: { icon: <FIAIcon size={size} />, bg: 'transparent', name: 'N/A' },
    default: {
      icon: (
        <div style={{ width: size, height: size }}>
          <div className="overflow-hidden rounded-full" style={{ width: size, height: size }}>
            <UnknownIcon
              iconURL={iconURL}
              endpoint={endpoint ?? ''}
              className="h-full w-full object-contain"
              context="message"
            />
          </div>
        </div>
      ),
      name: endpoint,
    },
  };

  let { icon, bg, name } =
    endpoint != null && endpoint && endpointIcons[endpoint]
      ? (endpointIcons[endpoint] ?? {})
      : (endpointIcons.default as EndpointIcon);

  if (iconURL && endpointIcons[iconURL]) {
    ({ icon, bg, name } = endpointIcons[iconURL]);
  }

  if (isAssistantsEndpoint(endpoint)) {
    return icon;
  }

  return (
    <div
      title={name ?? ''}
      style={{
        background: bg != null ? bg || 'transparent' : 'transparent',
        width: size,
        height: size,
      }}
      className={cn(
        'relative flex items-center justify-center rounded-sm text-white',
        props.className ?? '',
      )}
    >
      {icon}
      {error === true && (
        <span className="absolute right-0 top-[20px] -mr-2 flex h-3 w-3 items-center justify-center rounded-full border border-white bg-red-500 text-[10px] text-white">
          !
        </span>
      )}
    </div>
  );
};

export default memo(MessageEndpointIcon);
