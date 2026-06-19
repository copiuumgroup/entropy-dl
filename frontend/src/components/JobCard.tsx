import { useState, memo } from 'react';
import { motion } from 'framer-motion';
import type { JobCardProps } from '../types';
import { STATUS_COLORS, STATUS_LABELS } from '../lib/utils';
import { LinearProgress } from './LinearProgress';
import { Ripple } from './Ripple';

const JobCard = memo(function JobCard({ job, onRetry, onRemove, onOpenFolder }: JobCardProps) {
  const [copied, setCopied] = useState(false);

  const statusColor = STATUS_COLORS[job.status] || 'var(--md-outline)';
  const statusLabel = STATUS_LABELS[job.status] || job.status.toUpperCase();
  const progress = Math.max(0, Math.min(100, job.progress || 0));
  const isTerminal = job.status === 'failed' || job.status === 'canceled';
  const isActive = job.status === 'downloading' || job.status === 'processing' || job.status === 'searching';

  let progressColor: string | undefined;
  if (job.status === 'downloading') progressColor = 'var(--md-status-downloading)';
  else if (job.status === 'processing') progressColor = 'var(--md-status-processing)';
  else if (job.status === 'done') progressColor = 'var(--md-status-done)';
  else if (job.status === 'failed' || job.status === 'canceled') progressColor = 'var(--md-status-failed)';
  else if (job.status === 'searching') progressColor = 'var(--md-status-downloading)';

  const handleCopy = async (): Promise<void> => {
    try {
      await navigator.clipboard.writeText(job.output_file);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // clipboard may fail in some contexts
    }
  };

  const filename = job.output_file
    ? job.output_file.split(/[/\\]/).pop() || null
    : null;

  return (
    <motion.div
      className="queue-item"
      data-testid={`queue-item-${job.id}`}
      style={{ animation: 'm3-fade-in-up 0.3s cubic-bezier(0.05, 0.7, 0.1, 1.0) both' }}
      whileTap={{ scale: 0.995 }}
      transition={{ type: 'spring', stiffness: 500, damping: 30 }}
    >
      {job.thumbnail ? (
        <img className="thumb" src={job.thumbnail} alt="" loading="lazy" />
      ) : (
        <div className="thumb-placeholder">
          <span className="search-icon" style={{ fontSize: 'var(--md-sys-typescale-size-xl)', opacity: 0.4 }}>music_note</span>
        </div>
      )}

      <div className="body">
        <div className="queue-line-1">
          <span
            className="status-dot"
            style={{
              width: 'var(--size-status-dot)',
              height: 'var(--size-status-dot)',
              borderRadius: '50%',
              background: statusColor,
              flexShrink: 0,
            }}
            aria-hidden="true"
          />
          <span
            className="queue-status-badge"
            style={{ color: statusColor, borderColor: statusColor }}
            data-testid={`queue-status-${job.id}`}
          >
            {statusLabel}
          </span>
          {job.options?.media_type && (
            <span
              className="queue-type-badge"
              data-media={job.options.media_type}
              title={`Detected as ${job.options.media_type} by smart routing`}
            >
              {job.options.media_type.toUpperCase()}
            </span>
          )}
          <span className="queue-title" title={job.title || job.url}>
            {job.title || job.url}
          </span>
        </div>

        {job.uploader && <span className="queue-uploader">{job.uploader}</span>}

        {/* Animated progress bar */}
        {isActive || job.status === 'done' || isTerminal ? (
          <div className="progress-shell" data-testid={`queue-progress-${job.id}`}>
            <LinearProgress
              value={job.status === 'done' ? 100 : progress}
              determinate={job.status === 'downloading' || job.status === 'done'}
              color={progressColor}
            />
          </div>
        ) : (
          <div className="progress-shell" data-testid={`queue-progress-${job.id}`}>
            <div className="progress-fill" style={{ width: '0%' }} />
          </div>
        )}

        <div className="queue-line-3">
          {job.status === 'downloading' && (
            <>
              {Math.floor(progress)}% {job.speed || ''} ETA {job.eta || ''}
            </>
          )}
          {job.status === 'processing' && 'Encoding…'}
          {job.status === 'done' && (
            <span className="queue-done-file" title={filename || undefined}>{filename}</span>
          )}
          {job.status === 'failed' && (
            <span className="err">{job.error}</span>
          )}
          {job.status === 'queued' && 'Waiting'}
          {job.status === 'canceled' && (
            <span className="err">{job.error || 'Canceled'}</span>
          )}
          {job.status === 'searching' && 'Searching…'}
        </div>
      </div>

      <div className="actions" role="group" aria-label="Job actions">
        {job.status === 'done' && onOpenFolder && (
          <button
            className="btn icon"
            onClick={() => onOpenFolder(job.id)}
            title="Open folder"
            aria-label="Open folder containing this file"
          >
            <span className="search-icon">folder_open</span>
            <Ripple />
          </button>
        )}
        {job.status === 'done' && job.output_file && (
          <button
            className="btn icon"
            onClick={handleCopy}
            title={copied ? 'Copied' : 'Copy file path'}
            aria-label="Copy file path"
          >
            <span className="search-icon">{copied ? 'check' : 'content_copy'}</span>
            <Ripple />
          </button>
        )}
        {isTerminal && (
          <button
            className="btn icon"
            onClick={() => onRetry(job.id)}
            title="Retry download"
            aria-label="Retry download"
          >
            <span className="search-icon">refresh</span>
            <Ripple />
          </button>
        )}
        <button
          className="btn icon"
          onClick={() => onRemove(job.id)}
          title="Remove"
          aria-label="Remove job"
          style={{ color: 'var(--md-error)' }}
        >
          <span className="search-icon">close</span>
          <Ripple />
        </button>
      </div>
    </motion.div>
  );
});

export default JobCard;