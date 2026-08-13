import { useEffect, useState } from 'react';
import { CheckCircle2, ExternalLink, LoaderCircle, Server, X } from 'lucide-react';
import { ApiError, api } from '../../lib/api';
import type { PlexServer } from '../../lib/types';

type Phase =
  | { step: 'idle' }
  | { step: 'waiting'; pinId: number; code: string; until: number }
  | { step: 'picking'; token: string; servers: PlexServer[] }
  | { step: 'failed'; message: string };

const POLL_MS = 2000;

export function PlexServerPicker({
  servers,
  token,
  onPicked,
}: {
  servers: PlexServer[];
  token: string;
  onPicked: (url: string, token: string) => void;
}) {
  if (servers.length === 0) {
    return <p className="text-muted m-0 text-[13px]">This account reaches no Plex server.</p>;
  }

  return (
    <div className="sg-plex-servers">
      {servers.map((server) => (
        <section className="sg-plex-server" key={server.client_identifier}>
          <div className="sg-plex-server-head">
            <Server aria-hidden="true" size={16} />
            <strong>{server.name}</strong>
            <span className="text-muted">
              {server.connections.length} endpoint{server.connections.length === 1 ? '' : 's'}
            </span>
          </div>
          <div className="sg-plex-endpoints">
            {server.connections.length === 0 ? (
              <p className="text-muted m-0 text-[12px]">No address was returned for this server.</p>
            ) : (
              server.connections.map((connection, index) => {
                const route = connection.local ? 'Local' : connection.relay ? 'Relay' : 'Remote';
                // The list arrives best-first, so the first address that
                // answered is the one to take.
                const best = connection.reachable && index === 0;
                return (
                  <button
                    key={`${connection.uri}-${index}`}
                    type="button"
                    className="sg-plex-endpoint"
                    data-best={best ? '1' : undefined}
                    aria-label={`Use ${server.name} at ${connection.uri}`}
                    onClick={() => onPicked(connection.uri, token)}
                  >
                    <span className="sg-plex-route" data-route={route.toLowerCase()}>
                      {route}
                    </span>
                    <span className="sg-plex-uri">{connection.uri}</span>
                    <span className="sg-plex-reach" data-reach={connection.reachable ? 'ok' : 'no'}>
                      {best ? 'Recommended' : connection.reachable ? 'Answers' : 'No answer'}
                    </span>
                  </button>
                );
              })
            )}
          </div>
        </section>
      ))}
    </div>
  );
}

function reason(error: unknown): string {
  if (error instanceof ApiError && error.status === 404) {
    return 'this server does not offer Plex sign-in yet — paste a token instead';
  }
  if (error instanceof ApiError && error.status === 410) return 'the sign-in code expired';
  return error instanceof Error ? error.message : 'plex.tv is unreachable';
}

/** Trades a plex.tv PIN for a token, then lets the user pick one of their
    servers. The caller stores the resulting URL and token in the service. */
export function PlexSignIn({ onPicked }: { onPicked: (url: string, token: string) => void }) {
  const [phase, setPhase] = useState<Phase>({ step: 'idle' });
  const [starting, setStarting] = useState(false);

  async function start() {
    setStarting(true);
    try {
      const pin = await api.plexPin();
      window.open(pin.auth_url, 'snagarr-plex', 'width=620,height=740');
      setPhase({
        step: 'waiting',
        pinId: pin.id,
        code: pin.code,
        until: Date.parse(pin.expires_at),
      });
    } catch (error) {
      setPhase({ step: 'failed', message: reason(error) });
    } finally {
      setStarting(false);
    }
  }

  useEffect(() => {
    if (phase.step !== 'waiting') return;
    const { pinId, until } = phase;
    let live = true;

    const timer = window.setInterval(() => {
      if (Number.isFinite(until) && Date.now() >= until) {
        window.clearInterval(timer);
        setPhase({ step: 'failed', message: 'the sign-in code expired' });
        return;
      }
      void api
        .plexPinCheck(pinId)
        .then(async (check) => {
          if (!live || !check.token) return;
          window.clearInterval(timer);
          const { servers } = await api.plexServers(check.token);
          if (live) setPhase({ step: 'picking', token: check.token, servers });
        })
        .catch((error: unknown) => {
          window.clearInterval(timer);
          if (live) setPhase({ step: 'failed', message: reason(error) });
        });
    }, POLL_MS);

    return () => {
      live = false;
      window.clearInterval(timer);
    };
  }, [phase]);

  if (phase.step === 'waiting') {
    return (
      <Panel>
        <p className="sg-k m-0 flex items-center gap-2">
          <LoaderCircle className="animate-spin" aria-hidden="true" size={14} />
          Waiting for Plex · Code {phase.code}. Finish in the window that opened.
        </p>
        <button
          type="button"
          className="btn btn-ghost min-h-[44px] self-start"
          onClick={() => setPhase({ step: 'idle' })}
        >
          <X aria-hidden="true" size={15} />
          Cancel
        </button>
      </Panel>
    );
  }

  if (phase.step === 'picking') {
    return (
      <Panel>
        <p className="sg-k m-0 flex items-center gap-2">
          <CheckCircle2 aria-hidden="true" size={15} /> Signed in · Choose a server
        </p>
        <PlexServerPicker
          servers={phase.servers}
          token={phase.token}
          onPicked={(url, token) => {
            onPicked(url, token);
            setPhase({ step: 'idle' });
          }}
        />
        <button
          type="button"
          className="btn btn-ghost min-h-[44px] self-start"
          onClick={() => setPhase({ step: 'idle' })}
        >
          Cancel
        </button>
      </Panel>
    );
  }

  return (
    <Panel>
      <button
        type="button"
        className="btn btn-secondary min-h-[44px] self-start"
        style={{ fontSize: 12 }}
        disabled={starting}
        onClick={() => void start()}
      >
        <ExternalLink aria-hidden="true" size={16} />
        {starting ? 'Opening Plex…' : 'Sign in with Plex'}
      </button>
      {phase.step === 'failed' && <p className="sg-k sg-error m-0">{phase.message}</p>}
    </Panel>
  );
}

function Panel({ children }: { children: React.ReactNode }) {
  return <div className="flex flex-col gap-2 border-t border-line pt-3">{children}</div>;
}
