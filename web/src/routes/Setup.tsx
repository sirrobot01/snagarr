import { useState } from 'react';
import { useLocation } from 'wouter';
import { useSettingsDraft } from '../components/settings/draft';
import { ErrorState, Loading } from '../components/settings/states';
import { SetupCard } from '../components/setup/SetupCard';
import { StepArr } from '../components/setup/StepArr';
import { StepDone } from '../components/setup/StepDone';
import { StepLibrary } from '../components/setup/StepLibrary';
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
    copy: 'Take the household token, then start snagging.',
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
        <Loading label="LOADING SETTINGS…" />
      </SetupCard>
    );
  }

  return (
    <SetupCard
      step={step}
      title={page.title}
      copy={page.copy}
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
            {save.isPending ? 'SAVING…' : 'CONTINUE'}
          </button>
        </>
      }
    >
      {step === 0 && <StepTmdb settings={settings.data} draft={draft} />}
      {step === 1 && <StepLibrary settings={settings.data} draft={draft} />}
      {step === 2 && <StepArr settings={settings.data} draft={draft} />}
      {step === 3 && <StepDone settings={settings.data} />}
    </SetupCard>
  );
}
