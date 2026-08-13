import { useIsMutating } from '@tanstack/react-query';
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
  const inFlight = useIsMutating();

  if (online && !unreachable) return null;

  return (
    <div className="sg-banner" role="status" aria-live="polite">
      <WifiOff aria-hidden="true" />
      <span>
        {inFlight > 0
          ? `${online ? 'Server unreachable' : 'Offline'} — ${inFlight} capture${inFlight === 1 ? '' : 's'} queued`
          : online
            ? 'Snagarr is temporarily unreachable'
            : 'You’re offline'}
      </span>
      <span className="sg-banner-retry">
        <LoaderCircle aria-hidden="true" />
        Retrying
      </span>
    </div>
  );
}
