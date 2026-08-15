import { useState } from 'react';
import { useLocation } from 'wouter';
import { useSettingsDraft } from '../components/settings/draft';
import { ErrorState, Loading } from '../components/settings/states';
import { SetupCard } from '../components/setup/SetupCard';
import { StepDone } from '../components/setup/StepDone';
import { StepServices } from '../components/setup/StepServices';
import { StepTmdb } from '../components/setup/StepTmdb';
import { useSaveSettings, useSettings } from '../lib/queries';

const STEPS = [
  {
    title: 'Connect TMDB',
    copy: 'Snagarr needs one key to turn what you type into a real title.',
  },
  {
    title: 'Point at your library',
    copy: 'Snagarr reads your media server so it always knows what you already own.',
  },
  {
    title: 'Send to Radarr and Sonarr',
    copy: 'Snagged titles go here when you send them. Both are optional.',
  },
  {
    title: 'You are set up',
    copy: 'Review your connections, then start building the household list.',
  },
];

export default function Setup() {
  const [, navigate] = useLocation();
  const [step, setStep] = useState(0);
  const settings = useSettings(true);
  const draft = useSettingsDraft();
  const save = useSaveSettings();

  const page = STEPS[step];
  const last = step === STEPS.length - 1;
  const copy =
    step === 0 && settings.data?.tmdb.builtin_key
      ? 'Snagarr ships with a shared TMDB key, so this step is optional. Add your own key to use it instead.'
      : page.copy;

  /* Only step 1 writes settings. The service steps save themselves card by
     card, because a service has to exist before it can be tested. */
  async function advance(persist: boolean) {
    if (persist && draft.dirty) {
      try {
        await save.mutateAsync(draft.patch);
      } catch {
        return;
      }
      draft.reset();
    }
    if (last) {
      navigate('/');
      return;
    }
    setStep(step + 1);
  }

  if (settings.isError) {
    return (
      <SetupCard step={step} title={page.title} copy={page.copy}>
        <ErrorState error={settings.error} onRetry={() => void settings.refetch()} />
      </SetupCard>
    );
  }
  if (!settings.data) {
    return (
      <SetupCard step={step} title={page.title} copy={page.copy}>
        <Loading label="Loading settings…" />
      </SetupCard>
    );
  }

  return (
    <SetupCard
      step={step}
      title={page.title}
      copy={copy}
      footer={
        <>
          <button
            type="button"
            className="btn btn-ghost min-h-[44px]"
            disabled={step === 0}
            onClick={() => setStep(step - 1)}
          >
            Back
          </button>
          {!last && (
            <button
              type="button"
              className="btn btn-secondary ml-auto min-h-[44px]"
              onClick={() => void advance(false)}
            >
              Skip for now
            </button>
          )}
          <button
            type="button"
            className={`btn btn-primary min-h-[44px] ${last ? 'ml-auto' : ''}`}
            disabled={save.isPending}
            onClick={() => void advance(true)}
          >
            {save.isPending ? 'Saving…' : 'Continue'}
          </button>
        </>
      }
    >
      {step === 0 && <StepTmdb settings={settings.data} draft={draft} />}
      {step === 1 && (
        <StepServices
          kinds={['plex', 'emby', 'jellyfin']}
          empty="Add your media server, then sign in or paste a token. Plex can sign you in."
        />
      )}
      {step === 2 && (
        <StepServices
          kinds={['radarr', 'sonarr']}
          empty="Test each service to load its quality profiles and root folders. Skip this step if you send everything through Overseerr."
        />
      )}
      {step === 3 && <StepDone />}
    </SetupCard>
  );
}
