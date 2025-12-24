import React from 'react';
import type { IconMapProps } from '~/common';
import { cn } from '~/utils';

export const FIAIcon: React.FC<IconMapProps> = ({ 
  size = 24, 
  className,
  context,
  endpoint,
  iconURL,
  assistantName,
  agentName,
  avatar,
  endpointType,
}) => {
  console.log('FIAIcon size:', size, 'context:', context);
  return (
    <img
      src="/assets/FIA.svg"
      alt="FIA"
      width={size}
      height={size}
      className={cn('object-contain', className)}
      style={{ width: size, height: size }}
    />
  );
};

export default FIAIcon;
