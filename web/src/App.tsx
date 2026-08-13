import { Suspense, lazy, useEffect, useSyncExternalStore } from 'react';
import { Route, Switch, useLocation } from 'wouter';

import { Footer } from './components/Footer';
import { Nav } from './components/Nav';
import { OfflineBanner } from './components/OfflineBanner';
import { ToastHost } from './components/ToastHost';
import { TokenGate } from './components/TokenGate';
import { NetworkError } from './lib/api';
import { getToken, subscribeToken } from './lib/auth';
import { relativeTime } from './lib/format';
import { isAdmin, useMe, useStatus } from './lib/queries';
import { SEARCH_ID, Snag } from './routes/Snag';

const List = lazy(() => import('./routes/List'));
const Settings = lazy(() => import('./routes/Settings'));
const Setup = lazy(() => import('./routes/Setup'));

function Loading() {
  return (
    <p className="sg-k sg-pad py-6" role="status">
      LOADING…
    </p>
  );
}

export function App() {
  const token = useSyncExternalStore(subscribeToken, getToken, getToken);
  const [location, navigate] = useLocation();

  /* `/` returns to the capture box from anywhere, unless a field has focus. */
  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (event.key !== '/' || event.metaKey || event.ctrlKey || event.altKey) return;
      const target = event.target as HTMLElement | null;
      if (target && /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName)) return;
      if (target?.isContentEditable) return;

      event.preventDefault();
      if (location !== '/') navigate('/');
      requestAnimationFrame(() => document.getElementById(SEARCH_ID)?.focus());
    }
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [location, navigate]);

  if (!token) return <TokenGate rejected={false} />;

  return <Shell />;
}

function Shell() {
  const me = useMe();
  const status = useStatus();
  const admin = isAdmin(me.data);
  const unreachable = me.error instanceof NetworkError || status.error instanceof NetworkError;

  const counts = status.data?.counts;
  const footer =
    counts && counts.needs_review > 0
      ? `${counts.needs_review} IN NEEDS REVIEW`
      : status.data?.sync?.collection_at
        ? `SNAGGED COLLECTION SYNCED ${relativeTime(status.data.sync.collection_at)}`
        : 'SNAGGED COLLECTION NOT SYNCED YET';

  return (
    <div className="sg-shell">
      <OfflineBanner unreachable={unreachable} />
      <Nav me={me.data} admin={admin} />

      <main className="sg-col flex-1">
        <Suspense fallback={<Loading />}>
          <Switch>
            <Route path="/" component={Snag} />
            <Route path="/list" component={List} />
            {admin && <Route path="/settings" component={Settings} />}
            <Route path="/setup" component={Setup} />
            <Route>
              <div className="sg-pad py-8">
                <h3>Not found</h3>
                <p className="text-muted text-[13px]">That screen does not exist.</p>
              </div>
            </Route>
          </Switch>
        </Suspense>
      </main>

      <Footer context={footer} />
      <ToastHost />
    </div>
  );
}
