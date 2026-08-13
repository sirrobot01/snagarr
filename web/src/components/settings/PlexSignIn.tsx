import { useEffect, useState } from 'react';
import { ApiError, api } from '../../lib/api';
import type { PlexServer } from '../../lib/types';

type Phase =
  | { step: 'idle' }
  | { step: 'waiting'; pinId: number; code: string; until: number }
  | { step: 'picking'; token: string; servers: PlexServer[] }
  | { step: 'failed'; message: string };

const POLL_MS = 2000;

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
        <p className="sg-k m-0">
          WAITING FOR PLEX — CODE {phase.code}. FINISH IN THE WINDOW THAT OPENED.
        </p>
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

  if (phase.step === 'picking') {
    return (
      <Panel>
        <p className="sg-k m-0">SIGNED IN — CHOOSE A SERVER</p>
        {phase.servers.length === 0 && (
          <p className="text-muted m-0 text-[13px]">This account reaches no Plex server.</p>
        )}
        {phase.servers.map((server) => (
          <button
            key={server.client_identifier}
            type="button"
            className="btn btn-secondary min-h-[44px] self-start"
            disabled={server.connections.length === 0}
            onClick={() => {
              onPicked(server.connections[0].uri, phase.token);
              setPhase({ step: 'idle' });
            }}
          >
            {server.name}
            {server.connections[0] ? ` · ${server.connections[0].uri}` : ' · no address'}
          </button>
        ))}
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
        {starting ? 'OPENING PLEX…' : 'Sign in with Plex'}
      </button>
      {phase.step === 'failed' && <p className="sg-k m-0">{phase.message.toUpperCase()}</p>}
    </Panel>
  );
}

function Panel({ children }: { children: React.ReactNode }) {
  return <div className="flex flex-col gap-2 border-t border-line pt-3">{children}</div>;
}
