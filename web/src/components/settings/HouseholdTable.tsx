import { useMutation, useQueryClient } from '@tanstack/react-query';
import { KeyRound, Server, Trash2 } from 'lucide-react';
import { useState } from 'react';
import { api } from '../../lib/api';
import { keys } from '../../lib/queries';
import { pushToast } from '../../lib/toast';
import type { HouseholdUser } from '../../lib/types';
import { ConfirmDialog } from '../Modal';
import { errorText } from './states';

interface Props {
  users: HouseholdUser[];
  meId: number;
  onInspect: (user: HouseholdUser) => void;
  onTokens: (user: HouseholdUser) => void;
}

export function HouseholdTable({ users, meId, onInspect, onTokens }: Props) {
  const client = useQueryClient();
  const [removing, setRemoving] = useState<HouseholdUser | null>(null);

  const remove = useMutation({
    mutationFn: (userId: number) => api.deleteUser(userId),
    onSuccess: () => {
      pushToast('Member removed');
      setRemoving(null);
      void client.invalidateQueries({ queryKey: keys.users });
    },
    onError: (error) => pushToast(`Remove failed — ${errorText(error)}`),
  });

  return (
    <>
      <div className="sg-table-wrap">
        <table className="table sg-table-stack">
          <thead>
            <tr>
              <th>Username</th>
              <th>Role</th>
              <th>Tokens</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {users.map((user) => (
              <tr key={user.id}>
                <td data-label="Username">
                  <span className="block font-heading font-extrabold">@{user.username}</span>
                  {user.id === meId && <span className="sg-k">You</span>}
                </td>
                <td data-label="Role">
                  <span className={`sg-b ${user.role === 'admin' ? 'sg-lib' : 'sg-new'}`}>
                    {user.role}
                  </span>
                </td>
                <td data-label="Tokens" className="text-muted whitespace-nowrap">
                  {user.token_count} live
                  {user.telegram_user_id !== null && (
                    <span className="sg-k ml-2">
                      telegram {String(user.telegram_user_id).slice(0, 4)}…
                    </span>
                  )}
                </td>
                <td className="sg-row-actions">
                  <button
                    type="button"
                    className="btn btn-ghost min-h-[44px]"
                    onClick={() => onInspect(user)}
                  >
                    <Server aria-hidden="true" size={15} />
                    Services
                  </button>
                  <button
                    type="button"
                    className="btn btn-ghost min-h-[44px]"
                    onClick={() => onTokens(user)}
                  >
                    <KeyRound aria-hidden="true" size={15} />
                    Tokens
                  </button>
                  {user.id !== meId && (
                    <button
                      type="button"
                      className="btn btn-ghost min-h-[44px]"
                      disabled={remove.isPending}
                      onClick={() => setRemoving(user)}
                    >
                      <Trash2 aria-hidden="true" size={15} />
                      Remove
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <ConfirmDialog
        open={removing !== null}
        title={`Remove ${removing?.username ?? 'member'}?`}
        body="Their captures stay in the list without an owner. Their services and tokens go."
        confirmLabel="Remove"
        pending={remove.isPending}
        onClose={() => setRemoving(null)}
        onConfirm={() => removing && remove.mutate(removing.id)}
      />
    </>
  );
}
