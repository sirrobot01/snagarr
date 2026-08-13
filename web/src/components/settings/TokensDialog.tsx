import { AlertTriangle, KeyRound, Plus } from 'lucide-react';
import { useState } from 'react';
import { relativeTime, shortDate } from '../../lib/format';
import { useCreateToken, useRevokeToken, useTokens } from '../../lib/queries';
import { pushToast } from '../../lib/toast';
import type { HouseholdUser, Token } from '../../lib/types';
import { ConfirmDialog, Modal } from '../Modal';
import { CopyField, TextField } from './fields';
import { ErrorState, Loading } from './states';

/** Every client authenticates with its own bearer token. This is where they are
    issued, listed and revoked; the secret itself is readable once only. */
export function TokensDialog({ user, onClose }: { user: HouseholdUser; onClose: () => void }) {
  const tokens = useTokens(user.id);
  const create = useCreateToken(user.id);
  const revoke = useRevokeToken(user.id);
  const [name, setName] = useState('');
  const [secret, setSecret] = useState<string | null>(null);
  const [revoking, setRevoking] = useState<Token | null>(null);
  const [revokingAll, setRevokingAll] = useState(false);

  const live = (tokens.data?.tokens ?? []).filter((token) => !token.revoked);

  /* A lost phone is the case for this: every client the member holds stops at
     once, and the member signs in again to get a new browser session. */
  async function revokeAll() {
    const count = live.length;
    for (const token of live) {
      try {
        await revoke.mutateAsync(token.id);
      } catch {
        return;
      }
    }
    pushToast(`Revoked ${count} token${count === 1 ? '' : 's'}`);
    setRevokingAll(false);
  }

  function issue() {
    create.mutate(name.trim() === '' ? 'New token' : name.trim(), {
      onSuccess: (created) => {
        setSecret(created.token);
        setName('');
        pushToast(`Issued ${created.name}`);
      },
    });
  }

  return (
    <>
      <Modal
        open
        onClose={onClose}
        title={`Tokens · ${user.username}`}
        description="One token per client. Revoke a single one without disturbing the others."
        size="lg"
        footer={
          <>
            {live.length > 1 && (
              <button
                type="button"
                className="btn btn-danger"
                disabled={revoke.isPending}
                onClick={() => setRevokingAll(true)}
              >
                <KeyRound aria-hidden="true" size={16} />
                Revoke all
              </button>
            )}
            <button type="button" className="btn btn-secondary ml-auto" onClick={onClose}>
              Done
            </button>
          </>
        }
      >
        {secret && (
          <div className="sg-reveal">
            <CopyField id="new-token" label="New token" value={secret} />
            <p className="sg-k m-0 flex items-center gap-2">
              <AlertTriangle aria-hidden="true" size={14} />
              Copy it now. Snagarr stores only a hash, so this is the last time it is readable.
            </p>
          </div>
        )}

        <form
          className="sg-token-form"
          onSubmit={(event) => {
            event.preventDefault();
            if (!create.isPending) issue();
          }}
        >
          <TextField
            id="token-name"
            label="Token name"
            value={name}
            placeholder="iPhone Shortcut"
            description="Name it after the client that holds it, so you know what a revoke breaks."
            onChange={setName}
          />
          <button
            type="submit"
            className="btn btn-primary min-h-[44px]"
            disabled={create.isPending}
          >
            <Plus aria-hidden="true" size={16} />
            {create.isPending ? 'Issuing…' : 'Issue token'}
          </button>
        </form>

        {tokens.isError ? (
          <ErrorState error={tokens.error} onRetry={() => void tokens.refetch()} />
        ) : !tokens.data ? (
          <Loading label="Loading tokens…" />
        ) : live.length === 0 ? (
          <p className="text-muted m-0 text-[13px]">
            {user.username} holds no token yet. Signing in through a browser issues one
            automatically.
          </p>
        ) : (
          <div className="sg-table-wrap">
            <table className="table sg-table-stack">
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Prefix</th>
                  <th>Issued</th>
                  <th>Last used</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {live.map((token) => (
                  <tr key={token.id}>
                    <td data-label="Name" className="font-heading font-extrabold">
                      {token.name}
                    </td>
                    <td data-label="Prefix" className="text-muted break-all">
                      <code>{token.prefix}…</code>
                    </td>
                    <td data-label="Issued" className="text-muted whitespace-nowrap">
                      {shortDate(token.created_at)}
                    </td>
                    <td data-label="Last used" className="text-muted whitespace-nowrap">
                      {relativeTime(token.last_used_at)}
                    </td>
                    <td className="text-right">
                      <button
                        type="button"
                        className="btn btn-ghost min-h-[44px]"
                        disabled={revoke.isPending}
                        onClick={() => setRevoking(token)}
                      >
                        <KeyRound aria-hidden="true" size={15} />
                        Revoke
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Modal>

      <ConfirmDialog
        open={revokingAll}
        title={`Revoke every token for ${user.username}?`}
        body={`All ${live.length} tokens stop working, including the browser session ${user.username} is signed in with.`}
        confirmLabel="Revoke all"
        pending={revoke.isPending}
        onClose={() => setRevokingAll(false)}
        onConfirm={() => void revokeAll()}
      />

      <ConfirmDialog
        open={revoking !== null}
        title={`Revoke ${revoking?.name ?? 'token'}?`}
        body="Whatever holds this token stops working straight away. Other tokens keep working."
        confirmLabel="Revoke"
        pending={revoke.isPending}
        onClose={() => setRevoking(null)}
        onConfirm={() => {
          if (!revoking) return;
          revoke.mutate(revoking.id, {
            onSuccess: () => {
              pushToast(`Revoked ${revoking.name}`);
              setRevoking(null);
            },
          });
        }}
      />
    </>
  );
}
