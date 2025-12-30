import React, { useState, useCallback, useEffect } from 'react';
import { TFeedback } from 'librechat-data-provider';
import {
  Button,
  OGDialog,
  OGDialogContent,
  OGDialogTitle,
} from '@librechat/client';
import { useLocalize } from '~/hooks';
import { cn } from '~/utils';

interface FeedbackProps {
  handleFeedback: ({ feedback }: { feedback: TFeedback | undefined }) => void;
  feedback?: TFeedback;
  isLast?: boolean;
}

function FeedbackButtons({
  isLast,
  feedback,
  onFeedback,
}: {
  isLast: boolean;
  feedback?: TFeedback;
  onFeedback: (fb: TFeedback | undefined) => void;
}) {
  const localize = useLocalize();

  const handleThumbsUpClick = useCallback(
    (e: React.MouseEvent<HTMLButtonElement>) => {
      e.preventDefault();
      if (feedback?.rating === 'thumbsUp') {
        onFeedback(undefined);
      } else {
        onFeedback({ rating: 'thumbsUp', tag: undefined });
      }
    },
    [feedback, onFeedback],
  );

  const handleThumbsDownClick = useCallback(
    (e: React.MouseEvent<HTMLButtonElement>) => {
      e.preventDefault();
      if (feedback?.rating === 'thumbsDown') {
        onFeedback(undefined);
      } else {
        onFeedback({ rating: 'thumbsDown', tag: undefined });
      }
    },
    [feedback, onFeedback],
  );

  return (
    <>
      {feedback?.rating !== 'thumbsDown' && (
        <button
          className={buttonClasses(feedback?.rating === 'thumbsUp', isLast)}
          onClick={handleThumbsUpClick}
          type="button"
          title={localize('com_ui_feedback_positive')}
          aria-pressed={feedback?.rating === 'thumbsUp'}
        >
          <img 
            src="/assets/Vectorup.svg" 
            alt="Thumbs Up" 
            className="opacity-70 dark:brightness-0 dark:invert dark:opacity-70" 
            style={{ width: '19px', height: '19px' }}
          />
        </button>
      )}

      <button
        className={buttonClasses(feedback?.rating === 'thumbsDown', isLast)}
        onClick={handleThumbsDownClick}
        type="button"
        title={localize('com_ui_feedback_negative')}
        aria-pressed={feedback?.rating === 'thumbsDown'}
      >
        <img 
          src="/assets/Vector.svg" 
          alt="Thumbs Down" 
          className="opacity-70 dark:brightness-0 dark:invert dark:opacity-70" 
          style={{ width: '19px', height: '19px' }}
        />
      </button>
    </>
  );
}

function buttonClasses(isActive: boolean, isLast: boolean) {
  return cn(
    'hover-button rounded-lg p-1.5 text-text-secondary-alt transition-colors duration-200',
    'hover:text-text-primary hover:bg-surface-hover',
    'md:group-hover:visible md:group-focus-within:visible md:group-[.final-completion]:visible',
    !isLast && 'md:opacity-0 md:group-hover:opacity-100 md:group-focus-within:opacity-100',
    'focus-visible:ring-2 focus-visible:ring-black dark:focus-visible:ring-white focus-visible:outline-none',
    isActive && 'active text-text-primary bg-surface-hover',
  );
}

export default function Feedback({
  isLast = false,
  handleFeedback,
  feedback: initialFeedback,
}: FeedbackProps) {
  const localize = useLocalize();
  const [openDialog, setOpenDialog] = useState(false);
  const [feedback, setFeedback] = useState<TFeedback | undefined>(initialFeedback);

  useEffect(() => {
    setFeedback(initialFeedback);
  }, [initialFeedback]);

  const propagateMinimal = useCallback(
    (fb: TFeedback | undefined) => {
      setFeedback(fb);
      handleFeedback({ feedback: fb });
    },
    [handleFeedback],
  );

  const handleTextChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setFeedback((prev) => (prev ? { ...prev, text: e.target.value } : undefined));
  };

  const handleDialogSave = useCallback(() => {
    if (feedback?.tag?.key === 'other' && !feedback?.text?.trim()) {
      return;
    }
    propagateMinimal(feedback);
    setOpenDialog(false);
  }, [feedback, propagateMinimal]);

  const handleDialogClear = useCallback(() => {
    setFeedback(undefined);
    handleFeedback({ feedback: undefined });
    setOpenDialog(false);
  }, [handleFeedback]);

  const renderSingleFeedbackButton = () => {
    if (!feedback) return null;
    const isThumbsUp = feedback.rating === 'thumbsUp';
    const label = isThumbsUp
      ? localize('com_ui_feedback_positive')
      : localize('com_ui_feedback_negative');
    return (
      <button
        className={buttonClasses(true, isLast)}
        onClick={() => {
          propagateMinimal(undefined);
        }}
        type="button"
        title={label}
        aria-pressed="true"
      >
        <img 
          src={isThumbsUp ? "/assets/Vectorup.svg" : "/assets/Vector.svg"} 
          alt={label} 
          className="opacity-70 dark:brightness-0 dark:invert dark:opacity-70" 
          style={{ width: '19px', height: '19px' }}
        />
      </button>
    );
  };

  return (
    <>
      {feedback ? (
        renderSingleFeedbackButton()
      ) : (
        <FeedbackButtons
          isLast={isLast}
          feedback={feedback}
          onFeedback={propagateMinimal}
        />
      )}
      <OGDialog open={openDialog} onOpenChange={setOpenDialog}>
        <OGDialogContent className="w-11/12 max-w-lg">
          <OGDialogTitle className="text-token-text-primary text-lg font-semibold leading-6">
            {localize('com_ui_feedback_more_information')}
          </OGDialogTitle>
          <textarea
            className="w-full rounded-xl border border-border-light bg-transparent p-2 text-text-primary"
            value={feedback?.text || ''}
            onChange={handleTextChange}
            rows={4}
            placeholder={localize('com_ui_feedback_placeholder')}
            maxLength={500}
          />
          <div className="mt-4 flex items-end justify-end gap-2">
            <Button variant="destructive" onClick={handleDialogClear}>
              {localize('com_ui_delete')}
            </Button>
            <Button variant="submit" onClick={handleDialogSave} disabled={!feedback?.text?.trim()}>
              {localize('com_ui_save')}
            </Button>
          </div>
        </OGDialogContent>
      </OGDialog>
    </>
  );
}
