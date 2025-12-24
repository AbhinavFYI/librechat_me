import { TMessage } from 'librechat-data-provider';

const Container = ({ children, message }: { children: React.ReactNode; message?: TMessage }) => (
  <div
    className="text-message flex min-h-[20px] flex-col gap-3 overflow-visible [.text-message+&]:mt-5"
    style={{
      alignItems: message?.isCreatedByUser ? 'flex-end' : 'flex-start'
    }}
    dir="auto"
  >
    {children}
  </div>
);

export default Container;
