"use client";

import { useEffect, useState } from "react";
import { apiGet, ApiError } from "@/lib/api";
import type {
  BadgesResponse,
  Me,
  QuestsResponse,
  Streak,
  XPSummary,
} from "@/lib/types";
import { XpMeter } from "@/components/xp-meter";
import { StreakChip } from "@/components/streak-chip";
import { QuestCard } from "@/components/quest-card";
import { BadgeShelf } from "@/components/badge-shelf";

type Loaded = {
  me: Me;
  xp: XPSummary;
  streak: Streak;
  quests: QuestsResponse;
  badges: BadgesResponse;
};

export default function TodayPage() {
  const [data, setData] = useState<Loaded | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function load() {
      try {
        const [me, xp, streak, quests, badges] = await Promise.all([
          apiGet<Me>("/v1/me"),
          apiGet<XPSummary>("/v1/me/xp"),
          apiGet<Streak>("/v1/me/streak"),
          apiGet<QuestsResponse>("/v1/me/quests"),
          apiGet<BadgesResponse>("/v1/me/badges"),
        ]);
        if (!cancelled) setData({ me, xp, streak, quests, badges });
      } catch (e) {
        if (cancelled) return;
        if (e instanceof ApiError && e.status === 401) {
          setError("not-signed-in");
        } else {
          setError(e instanceof Error ? e.message : String(e));
        }
      }
    }

    load();
    return () => {
      cancelled = true;
    };
  }, []);

  if (error === "not-signed-in") {
    return (
      <main className="mx-auto max-w-4xl px-6 py-16">
        <h1 className="font-display text-4xl text-ink">Sign in to continue</h1>
        <p className="mt-4 text-ink-muted">
          You are not signed in. Use one of the links below.
        </p>
        <a
          className="mt-6 inline-block rounded-full bg-harbour px-6 py-3 text-white"
          href="http://localhost:8080/v1/auth/oidc/harbourline/start"
        >
          Sign in with OIDC
        </a>
        <a
          className="ml-3 mt-6 inline-block rounded-full border border-ink/20 px-6 py-3 text-ink"
          href="http://localhost:8080/v1/auth/saml/harbourline/start"
        >
          Sign in with SAML
        </a>
      </main>
    );
  }

  if (error) {
    return (
      <main className="mx-auto max-w-4xl px-6 py-16">
        <h1 className="font-display text-4xl text-vermilion">Something went wrong</h1>
        <pre className="mt-4 overflow-x-auto rounded bg-card p-4 text-sm">
          {error}
        </pre>
      </main>
    );
  }

  if (!data) {
    return (
      <main className="mx-auto max-w-4xl px-6 py-16">
        <p className="text-ink-muted">Loading…</p>
      </main>
    );
  }

  const firstName = data.me.full_name.split(" ")[0];

  return (
    <main className="mx-auto max-w-4xl px-6 py-12">
      <header className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <p className="text-sm uppercase tracking-widest text-ink-muted">
            Welcome back
          </p>
          <h1 className="mt-1 font-display text-4xl text-ink">
            {firstName}
          </h1>
        </div>
        <StreakChip streak={data.streak} />
      </header>

      <section className="mt-8">
        <XpMeter xp={data.xp} />
      </section>

      <section className="mt-8">
        <h2 className="font-display text-2xl text-ink">Today&apos;s quests</h2>
        {data.quests.active.length === 0 ? (
          <p className="mt-4 text-ink-muted">
            All quests complete for now. Come back tomorrow.
          </p>
        ) : (
          <div className="mt-4 grid gap-4">
            {data.quests.active.map((q) => (
              <QuestCard key={q.id} quest={q} />
            ))}
          </div>
        )}
      </section>

      <section className="mt-10">
        <BadgeShelf badges={data.badges.badges} />
      </section>
    </main>
  );
}