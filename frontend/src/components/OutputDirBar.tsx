import { useState, useEffect } from 'react';
import { fetchSettings, saveOutputDir } from '../lib/api';
import type { OutputDirBarProps } from '../types';

export default function OutputDirBar({ onToast }: OutputDirBarProps) {
  const [localValue, setLocalValue] = useState('');
  const [savedValue, setSavedValue] = useState('');
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    fetchSettings().then((data) => {
      setLocalValue(data.output_dir || '');
      setSavedValue(data.output_dir || '');
    });
  }, []);

  const isDirty = localValue.trim() !== savedValue && localValue.trim() !== '';

  const handleSave = async (): Promise<void> => {
    if (!isDirty || saving) return;
    setSaving(true);
    try {
      await saveOutputDir({ output_dir: localValue.trim() });
      setSavedValue(localValue.trim());
      onToast(`Output → ${localValue.trim()}`);
    } catch {
      onToast('Failed to save output directory', true);
    }
    setSaving(false);
  };

  return (
    <div className="outdir-bar" data-testid="output-dir-bar">
      <span className="outdir-label">Output</span>
      <input
        className="outdir-input"
        value={localValue}
        onChange={(e) => setLocalValue(e.target.value)}
        placeholder="/path/to/downloads"
        spellCheck={false}
        aria-label="Output directory path"
        data-testid="output-dir-input"
      />
      <button
        className={`btn ghost${isDirty ? ' outdir-dirty' : ''}`}
        disabled={!isDirty || saving}
        onClick={handleSave}
        aria-label={isDirty ? 'Apply changes' : 'Saved'}
        data-testid="output-dir-apply"
      >
        {saving ? '...' : isDirty ? 'Apply' : 'OK'}
      </button>
    </div>
  );
}
