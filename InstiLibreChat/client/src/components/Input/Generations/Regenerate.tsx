import type { TGenButtonProps } from '~/common';
import { useLocalize } from '~/hooks';
import Button from './Button';

export default function Regenerate({ onClick }: TGenButtonProps) {
  const localize = useLocalize();

  return (
    <Button onClick={onClick}>
      <img 
        src="/assets/repeat.svg" 
        alt="Regenerate" 
        className="h-3 w-3 flex-shrink-0 dark:invert" 
      />
      {localize('com_ui_regenerate')}
    </Button>
  );
}
