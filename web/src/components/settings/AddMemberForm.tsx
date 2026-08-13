import { useMutation, useQueryClient } from '@tanstack/react-query';
import { UserPlus, X } from 'lucide-react';
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
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [role, setRole] = useState<Role>('member');
  const [telegram, setTelegram] = useState('');
  const valid = username.trim() !== '' && password !== '';

  const create = useMutation({
    mutationFn: () => {
      const id = Number(telegram.trim());
      const body = {
        username: username.trim(),
        password,
        role,
        telegram_user_id: telegram.trim() !== '' && Number.isFinite(id) ? id : undefined,
      };
      return api.createUser(body);
    },
    onSuccess: (user) => {
      void client.invalidateQueries({ queryKey: keys.users });
      pushToast(`Added ${user.username}`);
      onDone();
    },
    onError: (error) => pushToast(`Add failed — ${errorText(error)}`),
  });

  return (
    <form
      className="flex flex-col gap-3 border-t border-line pt-4"
      onSubmit={(event) => {
        event.preventDefault();
        if (valid) create.mutate();
      }}
    >
      <div className="sg-section-heading">
        <UserPlus aria-hidden="true" size={18} />
        <div>
          <h5 className="m-0">Add a household member</h5>
          <p className="text-muted m-0 text-[12px]">
            Give them sign-in details now. They can connect their own services later.
          </p>
        </div>
      </div>
      <div className="sg-field-grid">
        <TextField
          id="member-username"
          label="Username"
          value={username}
          autoComplete="username"
          description="Used to sign in. Keep it short and easy to remember."
          required
          onChange={setUsername}
        />
        <TextField
          id="member-password"
          label="Password"
          value={password}
          type="password"
          autoComplete="new-password"
          required
          onChange={setPassword}
        />
        <TextField
          id="member-telegram"
          label="Telegram user ID"
          value={telegram}
          inputMode="numeric"
          description="Optional. Links captures from your Telegram bot to this person."
          onChange={setTelegram}
        />
      </div>
      <div className="field">
        <label>Account role</label>
        <Seg name="member-role" value={role} options={ROLES} onChange={setRole} />
      </div>
      <div className="flex items-center gap-2">
        <button
          type="submit"
          className="btn btn-primary min-h-[44px]"
          disabled={create.isPending || !valid}
        >
          <UserPlus aria-hidden="true" size={16} />
          {create.isPending ? 'Adding…' : 'Add member'}
        </button>
        <button type="button" className="btn btn-ghost min-h-[44px]" onClick={onDone}>
          <X aria-hidden="true" size={16} />
          Cancel
        </button>
      </div>
    </form>
  );
}
