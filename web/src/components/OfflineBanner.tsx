import { useIsMutating } from '@tanstack/react-query';
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
    <div className="sg-banner" role="status">
      <span>{inFlight > 0 ? `OFFLINE — ${inFlight} CAPTURES QUEUED` : 'OFFLINE'}</span>
      <span className="ml-auto">RETRYING…</span>
    </div>
  );
}
