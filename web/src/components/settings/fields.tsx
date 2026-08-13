import { pushToast } from '../../lib/toast';

const CONTROL = { minHeight: 44 };

interface TextFieldProps {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  locked?: boolean;
  placeholder?: string;
  inputMode?: 'text' | 'numeric' | 'url';
}

export function TextField({
  id,
  label,
  value,
  onChange,
  locked,
  placeholder,
  inputMode,
}: TextFieldProps) {
  return (
    <div className="field">
      <label htmlFor={id}>{locked ? `${label} · LOCKED` : label}</label>
      <input
        id={id}
        className="input"
        style={CONTROL}
        value={value}
        placeholder={placeholder}
        inputMode={inputMode}
        readOnly={locked}
        autoComplete="off"
        autoCapitalize="off"
        spellCheck={false}
        onChange={(event) => onChange(event.target.value)}
      />
    </div>
  );
}

interface SelectFieldProps {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: { value: string; label: string }[];
  placeholder: string;
  locked?: boolean;
}

export function SelectField({
  id,
  label,
  value,
  onChange,
  options,
  placeholder,
  locked,
}: SelectFieldProps) {
  return (
    <div className="field">
      <label htmlFor={id}>{locked ? `${label} · LOCKED` : label}</label>
      <select
        id={id}
        className="input"
        style={CONTROL}
        value={value}
        disabled={locked}
        onChange={(event) => onChange(event.target.value)}
      >
        <option value="">{placeholder}</option>
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </div>
  );
}

interface SegProps<T extends string> {
  name: string;
  value: T | '';
  options: { value: T; label: string }[];
  onChange: (value: T) => void;
  locked?: boolean;
}

export function Seg<T extends string>({ name, value, options, onChange, locked }: SegProps<T>) {
  return (
    <div className="seg self-start">
      {options.map((option) => (
        <label key={option.value} className="seg-opt" style={CONTROL}>
          <input
            type="radio"
            name={name}
            value={option.value}
            checked={value === option.value}
            disabled={locked}
            onChange={() => onChange(option.value)}
          />
          {option.label}
        </label>
      ))}
    </div>
  );
}

export function CopyField({ id, label, value }: { id: string; label: string; value: string }) {
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      <div className="flex items-start gap-2">
        <textarea
          id={id}
          className="input flex-1 text-[12px]"
          style={{ fontFamily: 'ui-monospace, monospace' }}
          readOnly
          value={value}
          onFocus={(event) => event.currentTarget.select()}
        />
        <button
          type="button"
          className="btn btn-secondary min-h-[44px]"
          onClick={() => copyToClipboard(value)}
        >
          Copy
        </button>
      </div>
    </div>
  );
}

function copyToClipboard(value: string) {
  navigator.clipboard.writeText(value).then(
    () => pushToast('COPIED TO CLIPBOARD'),
    () => pushToast('COPY FAILED — SELECT THE TEXT AND COPY IT'),
  );
}
