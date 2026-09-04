<template>
  <div class="usage">
    <template v-if="usage.status === 'ok'">
      <div v-if="usage.session" class="usage-row">
        <span class="usage-label">{{ t('fiveHour') }}</span>
        <span class="usage-bar" aria-hidden="true">
          <i :class="tone(usage.session.usedPercent)" :style="{ width: barWidth(usage.session.usedPercent) }"></i>
        </span>
        <span class="usage-pct" :class="tone(usage.session.usedPercent)">{{ fmtPct(usage.session.usedPercent) }}</span>
        <span class="usage-reset">{{ usage.session.resetsIn || '—' }}</span>
      </div>
      <div v-if="usage.weekly" class="usage-row">
        <span class="usage-label">{{ t('weekly') }}</span>
        <span class="usage-bar" aria-hidden="true">
          <i :class="tone(usage.weekly.usedPercent)" :style="{ width: barWidth(usage.weekly.usedPercent) }"></i>
        </span>
        <span class="usage-pct" :class="tone(usage.weekly.usedPercent)">{{ fmtPct(usage.weekly.usedPercent) }}</span>
        <span class="usage-reset">{{ usage.weekly.resetsIn || '—' }}</span>
      </div>
    </template>
    <div v-else-if="usage.status === 'loading'" class="usage-note muted">{{ t('usageLoading') }}</div>
    <div v-else-if="usage.status === 'unauthorized'" class="usage-note bad">{{ usage.error || t('usageUnauthorized') }}</div>
    <div v-else-if="usage.status === 'empty'" class="usage-note muted">{{ usage.error || t('usageEmpty') }}</div>
    <div v-else-if="usage.status === 'missing'" class="usage-note muted">{{ usage.error || t('usageMissing') }}</div>
    <div v-else-if="usage.status" class="usage-note bad">{{ usage.error || t('usageFailed') }}</div>
  </div>
</template>

<script setup lang="ts">
import type {KimiUsage} from './api';
import {t} from './i18n';

defineProps<{
  usage: KimiUsage;
}>();

function tone(pct: number) {
  if (pct >= 90) return 'bad';
  if (pct >= 70) return 'warn';
  return 'ok';
}

function fmtPct(pct: number) {
  const r = Math.round(pct * 10) / 10;
  return `${Number.isInteger(r) ? String(r) : r.toFixed(1)}%`;
}

function barWidth(pct: number) {
  const n = Math.max(0, Math.min(100, pct));
  return n + '%';
}
</script>
