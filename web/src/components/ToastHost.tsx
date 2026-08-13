import * as Toast from '@radix-ui/react-toast';
import { useSyncExternalStore } from 'react';
import { dismissToast, getToasts, subscribeToasts } from '../lib/toast';

export function ToastHost() {
  const toasts = useSyncExternalStore(subscribeToasts, getToasts, getToasts);

  return (
    <Toast.Provider duration={6000} swipeDirection="right">
      {toasts.map((toast) => (
        <Toast.Root
          key={toast.id}
          className="sg-toast"
          duration={6000}
          onOpenChange={(open) => {
            if (!open) dismissToast(toast.id);
          }}
        >
          <Toast.Title className="sg-toast-text">{toast.text}</Toast.Title>
          {toast.action && (
            <Toast.Action asChild altText={toast.action.label}>
              <button
                type="button"
                className="btn btn-primary ml-auto text-[11px] tracking-[0.08em]"
                onClick={() => {
                  toast.action?.run();
                  dismissToast(toast.id);
                }}
              >
                {toast.action.label}
              </button>
            </Toast.Action>
          )}
        </Toast.Root>
      ))}
      <Toast.Viewport className="fixed bottom-4 left-4 right-4 z-[60] m-0 flex list-none flex-col gap-2 p-0 outline-none md:left-auto md:right-5 md:w-[380px]" />
    </Toast.Provider>
  );
}
