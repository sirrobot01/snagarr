import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { api } from '../../lib/api';
import { keys } from '../../lib/queries';
import { pushToast } from '../../lib/toast';
import type { Role } from '../../lib/types';
import { Seg, TextField } from './fields';
import { errorText } from './states';

const ROLES: { value: Role; label: string }[] = [
  { value: 'member', label: 'Member' },
  { value: 'admin', label: 'Admin' },
];

export function AddMemberForm({ onDone }: { onDone: () => void }) {
  const client = useQueryClient();
  const [name, setName] = useState('');
  const [role, setRole] = useState<Role>('member');
  const [telegram, setTelegram] = useState('');

  const create = useMutation({
    mutationFn: () => {
      const id = Number(telegram.trim());
      return api.createUser({
        display_name: name.trim(),
        role,
        telegram_user_id: telegram.trim() !== '' && Number.isFinite(id) ? id : undefined,
      });
    },
    onSuccess: (user) => {
      void client.invalidateQueries({ queryKey: keys.users });
      pushToast(`ADDED — ${user.display_name.toUpperCase()}`);
      onDone();
    },
    onError: (error) => pushToast(`ADD FAILED — ${errorText(error).toUpperCase()}`),
  });

  return (
    <form
      className="flex flex-col gap-3 border-t border-line pt-4"
      onSubmit={(event) => {
        event.preventDefault();
        if (name.trim() !== '') create.mutate();
      }}
    >
      <p className="sg-k m-0">NEW HOUSEHOLD MEMBER</p>
      <TextField id="member-name" label="Display name" value={name} onChange={setName} />
      <Seg name="member-role" value={role} options={ROLES} onChange={setRole} />
      <TextField
        id="member-telegram"
        label="Telegram user ID (optional)"
        value={telegram}
        inputMode="numeric"
        onChange={setTelegram}
      />
      <div className="flex items-center gap-2">
        <button
          type="submit"
          className="btn btn-primary min-h-[44px]"
          disabled={create.isPending || name.trim() === ''}
        >
          {create.isPending ? 'ADDING…' : 'Add member'}
        </button>
        <button type="button" className="btn btn-ghost min-h-[44px]" onClick={onDone}>
          Cancel
        </button>
      </div>
    </form>
  );
}
