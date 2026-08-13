import { useMutation } from '@tanstack/react-query';
import { AlertTriangle, Bookmark } from 'lucide-react';
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
    onError: (error) => pushToast(`Token failed — ${errorText(error)}`),
  });

  return { code, pending: create.isPending, generate: () => create.mutate() };
}

export function BookmarkletPanel({ code }: { code: string }) {
  return (
    <div className="flex flex-col gap-2 border-t border-line pt-4">
      <div className="sg-section-heading">
        <Bookmark aria-hidden="true" size={18} />
        <div>
          <h5 className="m-0">Browser bookmarklet</h5>
          <p className="text-muted m-0 text-[12px]">
            Drag the generated link to your bookmarks bar to snag the current page.
          </p>
        </div>
      </div>
      <CopyField id="bookmarklet" label="Drag this to your bookmarks bar" value={code} />
      <p className="sg-k m-0 flex items-center gap-2">
        <AlertTriangle aria-hidden="true" size={14} />
        This private link contains a new token and is shown only once.
      </p>
    </div>
  );
}
