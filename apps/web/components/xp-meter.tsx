import type { XPSummary } from "@/lib/types";

export function XpMeter({ xp }: { xp: XPSummary }) {
  return (
    <div className="rounded-[var(--radius-card)] bg-card p-6 shadow-[var(--shadow-paper)]">
      <div className="flex items-baseline justify-between">
        <div>
          <p className="text-sm uppercase tracking-widest text-ink-muted">Level</p>
          <p className="font-display text-5xl leading-none text-ink">{xp.level}</p>
        </div>
        <div className="text-right">
          <p className="font-mono text-sm text-ink-muted">
            {xp.total_xp.toLocaleString()} XP
          </p>
          <p className="text-xs text-ink-muted">
            {xp.xp_into_level.toLocaleString()} / {xp.xp_for_next_level.toLocaleString()} to next
          </p>
        </div>
      </div>
      <div
        className="mt-4 h-2 overflow-hidden rounded-full bg-paper"
        role="progressbar"
        aria-valuenow={xp.progress_pct}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-label={`Level progress: ${xp.progress_pct}%`}
      >
        <div
          className="h-full bg-marigold transition-all duration-[var(--duration-base)] ease-[var(--ease-paper)]"
          style={{ width: `${xp.progress_pct}%` }}
        />
      </div>
    </div>
  );
}