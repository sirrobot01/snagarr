import { LoaderCircle, WifiOff } from 'lucide-react';
import { useSyncExternalStore } from 'react';

function subscribe(onChange: () => void) {
  window.addEventListener('online', onChange);
  window.addEventListener('offline', onChange);
  return () => {
    window.removeEventListener('online', onChange);
    window.removeEventListener('offline', onChange);
  };
}

export function useOnline() {
  return useSyncExternalStore(
    subscribe,
    () => navigator.onLine,
    () => true,
  );
}

export function OfflineBanner({ unreachable }: { unreachable: boolean }) {
  const online = useOnline();

  if (online && !unreachable) return null;

  // Nothing is queued while disconnected — a failed snag rolls back with its
  // own toast — so the banner promises no more than what actually happens.
  return (
    <div className="sg-banner" role="status" aria-live="polite">
      <WifiOff aria-hidden="true" />
      <span>
        {online
          ? 'Snagarr is temporarily unreachable — changes will not save'
          : 'You’re offline — changes will not save'}
      </span>
      <span className="sg-banner-retry">
        <LoaderCircle aria-hidden="true" />
        Retrying
      </span>
    </div>
  );
}
