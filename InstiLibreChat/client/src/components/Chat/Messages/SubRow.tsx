import { cn } from '~/utils';

type TSubRowProps = {
  children: React.ReactNode;
  classes?: string;
  subclasses?: string;
  onClick?: () => void;
  isUser?: boolean;
};

export default function SubRow({ children, classes = '', onClick, isUser = false }: TSubRowProps) {
  return (
    <div
      className={cn('mt-1 flex gap-3 empty:hidden lg:flex', classes)}
      style={{
        justifyContent: isUser ? 'flex-end' : 'flex-start'
      }}
      onClick={onClick}
    >
      {children}
    </div>
  );
}
