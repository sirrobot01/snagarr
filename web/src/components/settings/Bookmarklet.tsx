import { useMutation } from '@tanstack/react-query';
import { useState } from 'react';
import { api } from '../../lib/api';
import { pushToast } from '../../lib/toast';
import { CopyField } from './fields';
import { errorText } from './states';

function build(publicUrl: string, token: string): string {
  const base = (publicUrl.trim() === '' ? window.location.origin : publicUrl).replace(/\/+$/, '');
  return (
    `javascript:(function(){fetch('${base}/api/v1/capture',{method:'POST',` +
    `headers:{'Content-Type':'application/json','Authorization':'Bearer ${token}'},` +
    `body:JSON.stringify({url:location.href,source:'bookmarklet'})})` +
    `.then(function(r){alert(r.ok?'Snagged':'Snagarr said '+r.status)})` +
    `.catch(function(e){alert('Snagarr unreachable: '+e)})})()`
  );
}

export function useBookmarklet(publicUrl: string, userId: number) {
  const [code, setCode] = useState<string | null>(null);

  const create = useMutation({
    mutationFn: () => api.createToken(userId, 'Bookmarklet'),
    onSuccess: (token) => setCode(build(publicUrl, token.token)),
    onError: (error) => pushToast(`TOKEN FAILED — ${errorText(error).toUpperCase()}`),
  });

  return { code, pending: create.isPending, generate: () => create.mutate() };
}

export function BookmarkletPanel({ code }: { code: string }) {
  return (
    <div className="flex flex-col gap-2 border-t border-line pt-4">
      <CopyField id="bookmarklet" label="Drag this to your bookmarks bar" value={code} />
      <p className="sg-k m-0">
        THIS LINK CARRIES A NEW TOKEN. IT IS READABLE ONCE — COPY IT NOW.
      </p>
    </div>
  );
}
