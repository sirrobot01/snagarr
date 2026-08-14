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

/* One row per server the account reaches, named rather than addressed. Picking
   one fills the token only: the URL is the operator's own answer to "which
   address can this Snagarr reach?", and plex.tv cannot answer that for them. */
export function PlexServerPicker({
  servers,
  token,
  onPicked,
}: {
  servers: PlexServer[];
  token: string;
  onPicked: (token: string) => void;
}) {
  // plex.tv repeats a server once per shared account, so it is deduplicated on
  // the identifier the machine actually keeps.
  const unique = servers.filter(
    (server, index) =>
      servers.findIndex((other) => other.client_identifier === server.client_identifier) === index,
  );

  if (unique.length === 0) {
    return <p className="text-muted m-0 text-[13px]">This account reaches no Plex server.</p>;
  }

  return (
    <div className="sg-plex-servers">
      {unique.map((server) => (
        <button
          key={server.client_identifier}
          type="button"
          className="sg-plex-server-pick"
          aria-label={`Use the token for ${server.name}`}
          onClick={() => onPicked(token)}
        >
          <Server aria-hidden="true" size={16} />
          <span className="sg-plex-server-name">{server.name}</span>
          <span className="sg-k">
            {server.connections.some((connection) => connection.reachable)
              ? 'Answers'
              : 'No answer'}
          </span>
        </button>
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

/** Trades a plex.tv PIN for a token, then lets the user confirm which of their
    servers it is for. The caller stores the token; the address stays theirs. */
export function PlexSignIn({ onPicked }: { onPicked: (token: string) => void }) {
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
      void api.plexPinCheck(pinId).then(
        async (check) => {
          if (!live || !check.token) return;
          window.clearInterval(timer);
          try {
            const { servers } = await api.plexServers(check.token);
            if (live) setPhase({ step: 'picking', token: check.token, servers });
          } catch (error) {
            if (live) setPhase({ step: 'failed', message: reason(error) });
          }
        },
        (error: unknown) => {
          // A dropped poll is not an answer: the code is still pending on
          // plex.tv, so the wait continues until it expires. Only a definitive
          // refusal ends the sign-in early.
          const fatal =
            error instanceof ApiError && (error.status === 401 || error.status === 404 || error.status === 410);
          if (!fatal) return;
          window.clearInterval(timer);
          if (live) setPhase({ step: 'failed', message: reason(error) });
        },
      );
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
          <CheckCircle2 aria-hidden="true" size={15} /> Signed in · Choose a server to take its
          token
        </p>
        <PlexServerPicker
          servers={phase.servers}
          token={phase.token}
          onPicked={(token) => {
            onPicked(token);
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
