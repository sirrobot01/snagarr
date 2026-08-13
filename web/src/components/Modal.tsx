import * as Dialog from '@radix-ui/react-dialog';
import { X } from 'lucide-react';
import type { ReactNode } from 'react';

interface ModalProps {
  open: boolean;
  onClose: () => void;
  title: string;
  description?: string;
  /** sm for a question, lg for the service forms. */
  size?: 'sm' | 'md' | 'lg';
  footer?: ReactNode;
  children: ReactNode;
}

/** The one overlay every settings surface uses. It centres itself on a large
    screen and becomes a bottom sheet on a phone, which is a CSS matter only. */
export function Modal({ open, onClose, title, description, size = 'md', footer, children }: ModalProps) {
  return (
    <Dialog.Root open={open} onOpenChange={(next) => !next && onClose()}>
      <Dialog.Portal>
        <Dialog.Overlay className="sg-scrim" />
        <Dialog.Content
          className="sg-modal"
          data-size={size}
          /* Radix points aria-describedby at its own Description. Without one
             it warns unless the attribute is dropped on purpose. */
          {...(description ? {} : { 'aria-describedby': undefined })}
        >
          <header className="sg-modal-head">
            <div className="min-w-0 flex-1">
              <Dialog.Title className="sg-modal-title">{title}</Dialog.Title>
              {description && (
                <Dialog.Description className="sg-modal-copy">{description}</Dialog.Description>
              )}
            </div>
            <Dialog.Close className="sg-modal-close" aria-label="Close">
              <X aria-hidden="true" size={18} />
            </Dialog.Close>
          </header>

          <div className="sg-modal-body">{children}</div>

          {footer && <footer className="sg-modal-foot">{footer}</footer>}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

interface ConfirmProps {
  open: boolean;
  title: string;
  body: string;
  confirmLabel: string;
  pending?: boolean;
  onConfirm: () => void;
  onClose: () => void;
}

/** Destructive answers stay inside the interface rather than in a browser
    confirm, which cannot be styled and reads as a different application. */
export function ConfirmDialog({
  open,
  title,
  body,
  confirmLabel,
  pending,
  onConfirm,
  onClose,
}: ConfirmProps) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      title={title}
      size="sm"
      footer={
        <>
          <button type="button" className="btn btn-secondary" onClick={onClose}>
            Cancel
          </button>
          <button
            type="button"
            className="btn btn-danger ml-auto"
            disabled={pending}
            onClick={onConfirm}
          >
            {pending ? 'Working…' : confirmLabel}
          </button>
        </>
      }
    >
      <p className="text-muted m-0 text-[13px]">{body}</p>
    </Modal>
  );
}
