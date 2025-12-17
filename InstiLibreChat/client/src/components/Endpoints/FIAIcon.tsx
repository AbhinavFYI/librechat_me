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
  return (
    <img
      src="/assets/FIA.svg"
      alt="FIA"
      width={size}
      height={size}
      className={cn('object-contain', className)}
    />
  );
};

export default FIAIcon;
