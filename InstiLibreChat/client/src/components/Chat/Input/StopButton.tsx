import { TooltipAnchor } from '@librechat/client';
import { useLocalize } from '~/hooks';
import { cn } from '~/utils';

export default function StopButton({ stop, setShowStopButton }) {
  const localize = useLocalize();

  return (
    <TooltipAnchor
      description={localize('com_nav_stop_generating')}
      render={
        <button
          type="button"
          className={cn(
            'rounded-full bg-blue-600 p-1.5 text-white outline-offset-4 transition-all duration-200 hover:bg-blue-700 disabled:cursor-not-allowed disabled:bg-blue-400 disabled:opacity-50',
          )}
          aria-label={localize('com_nav_stop_generating')}
          onClick={(e) => {
            setShowStopButton(false);
            stop(e);
          }}
        >
          <svg
            width="24"
            height="24"
            viewBox="0 0 24 24"
            fill="none"
            xmlns="http://www.w3.org/2000/svg"
            className="icon-lg text-white"
          >
            <rect x="7" y="7" width="10" height="10" rx="1.25" fill="currentColor"></rect>
          </svg>
        </button>
      }
    ></TooltipAnchor>
  );
}
