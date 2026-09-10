<template>
  <div class="shell">
    <!-- CLI 面板 -->
    <main class="panels">
      <nav v-if="clis.length > 1" class="tabs">
        <button
          v-for="cli in clis"
          :key="cli.id"
          class="tab"
          :class="{ active: activeTab === cli.id }"
          @click="activeTab = cli.id"
        >
          {{ cli.name }}
          <span class="tab-dot" :class="{ online: cli.liveExists }" aria-hidden="true"></span>
        </button>
      </nav>

      <section v-for="cli in clis" :key="cli.id" v-show="activeTab === cli.id" class="cli-card">
        <!-- 卡片头部 -->
        <div class="cli-head">
          <div class="cli-title-row">
            <h2>{{ cli.name }}</h2>
            <span class="login-status" :class="cli.liveExists ? 'ok' : 'bad'">
              <span class="status-dot" aria-hidden="true"></span>
              {{ t(cli.liveExists ? 'loggedIn' : 'loggedOut') }}
            </span>
            <button class="lang-btn" @click="toggleLang" title="中文 / English">
              {{ lang === 'zh' ? 'EN' : '中' }}
            </button>
          </div>
          <div class="cli-status">
            <button class="btn quiet" @click="openStore" :title="t('openStoreTitle')">
              <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/></svg>
              {{ t('storeDir') }}
            </button>
            <button class="btn icon-btn" @click="refresh" :disabled="loading" :class="{ spinning: loading }" :aria-label="t(loading ? 'refreshing' : 'refresh')" :title="t('refresh')">
              <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M21 12a9 9 0 1 1-9-9c2.52 0 4.93 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/></svg>
            </button>
            <button class="btn primary" @click="openCapture(cli)" :disabled="!cli.liveExists">
              {{ t('saveAccount') }}
            </button>
          </div>
        </div>

        <!-- 环境变量绕过警告 -->
        <div v-for="(w, i) in cli.envWarnings" :key="i" class="alert warn">
          <strong>{{ t('switchMayFail') }}</strong>
          <div>{{ w }}</div>
        </div>

        <!-- 首次使用提示 -->
        <div v-if="cli.profiles.length === 0" class="empty">
          <template v-if="cli.liveExists">
            {{ t('emptyHasLive') }}
          </template>
          <template v-else>
            {{ t('emptyNoLiveA') }}<code>kimi</code>{{ t('emptyNoLiveB') }}
          </template>
        </div>

        <!-- Profile 列表 -->
        <div v-else class="profiles-block">
          <div class="section-head">
            <h3>{{ t('savedAccounts') }}</h3>
          </div>
          <ul class="profiles">
          <li v-for="p in cli.profiles" :key="p.name" class="profile" :class="{ active: p.isActive }">
            <div class="profile-main">
              <div class="profile-name">
                <span class="name">{{ p.name }}</span>
                <span v-if="!p.hasCredential" class="chip bad">{{ t('missingSnapshot') }}</span>
                <span v-if="p.isActive" class="active-label">{{ t('currentLabel') }}</span>
                <span class="name-meta">
                  <span v-if="p.usage?.plan && p.usage.status === 'ok'" class="plan-chip">{{ p.usage.plan }}</span>
                  <span>{{ t('updatedAt', { time: p.updatedAt || '—' }) }}</span>
                </span>
              </div>
              <div v-if="p.isActive && (p.freshness === 'rotated' || p.freshness === 'missing-live')"
                   class="profile-meta">
                <span class="note" :class="p.freshness === 'rotated' ? 'rotated' : 'stale'">
                  ● {{ t(p.freshness === 'rotated' ? 'rotatedNote' : 'missingLiveNote') }}
                </span>
              </div>
              <div class="profile-body">
                <UsageMeters v-if="p.usage" :usage="p.usage" />
                <div class="profile-actions">
                  <button v-if="!p.isActive" class="btn primary sm" :disabled="busy" @click="doSwitch(cli, p)">
                    {{ t('switchLabel') }}
                  </button>
                  <button v-else class="btn quiet sm" :disabled="busy"
                          :title="t('recaptureTip', { name: p.name })"
                          @click="doRecapture(cli, p)">
                    {{ t('recapture') }}
                  </button>
                  <button class="btn icon-btn danger sm" :disabled="busy" @click="confirmDelete(cli, p)" :title="t('deleteAccountTip')" :aria-label="t('deleteAria', { name: p.name })">
                    <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 7h16M9 7V4.75h6V7m3 0-1 13H7L6 7m4 4v5m4-5v5"/></svg>
                  </button>
                </div>
              </div>
            </div>
          </li>
          </ul>
        </div>
      </section>
    </main>

    <!-- 保存弹窗 -->
    <div v-if="modal.capture" class="overlay" @click.self="closeModal">
      <div class="modal">
        <h3>{{ t('captureTitle') }}</h3>
        <label>{{ t('nameLabel') }}</label>
        <input v-model.trim="modal.captureName" ref="captureNameInput" maxlength="40"
               :placeholder="t('namePlaceholder')"
               @keyup.enter="submitCapture" />
        <div class="modal-actions">
          <button class="btn ghost" @click="closeModal">{{ t('cancel') }}</button>
          <button class="btn primary" :disabled="busy || !modal.captureName" @click="submitCapture">
            {{ busy ? t('saving') : t('save') }}
          </button>
        </div>
      </div>
    </div>

    <!-- 确认弹窗 -->
    <div v-if="modal.confirm" class="overlay" @click.self="closeModal">
      <div class="modal">
        <h3>{{ modal.confirm.title }}</h3>
        <p class="modal-hint">{{ modal.confirm.body }}</p>
        <div class="modal-actions">
          <button class="btn ghost" @click="closeModal">{{ t('cancel') }}</button>
          <button class="btn danger" :disabled="busy" @click="modal.confirm.ok">
            {{ busy ? t('working') : modal.confirm.okText || t('ok') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Toast -->
    <div class="toasts">
      <div v-for="t in toasts" :key="t.id" class="toast" :class="t.kind" @click="dismiss(t.id)">
        {{ t.text }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import {onBeforeUnmount, onMounted, reactive, ref, nextTick, watch} from 'vue';
import {api, CliView, ProfileView, SwitchResult} from './api';
import UsageMeters from './UsageMeters.vue';
import {lang, toggleLang, t, listSep} from './i18n';

const clis = ref<CliView[]>([]);
const loading = ref(false);
const busy = ref(false);
const activeTab = ref('');

watch(clis, (val) => {
  if (val.length && !val.find(c => c.id === activeTab.value)) {
    activeTab.value = val[0].id;
  }
}, { immediate: true });

interface Toast { id: number; kind: 'ok' | 'err' | 'info'; text: string }
const toasts = ref<Toast[]>([]);
let toastSeq = 1;

function toast(kind: Toast['kind'], text: string) {
  const id = toastSeq++;
  toasts.value.push({id, kind, text});
  setTimeout(() => dismiss(id), 5200);
}
function dismiss(id: number) {
  toasts.value = toasts.value.filter(t => t.id !== id);
}

const captureNameInput = ref<HTMLInputElement | null>(null);

const modal = reactive<{
  capture: CliView | null;
  captureName: string;
  confirm: { title: string; body: string; okText?: string; ok: () => void } | null;
}>({
  capture: null, captureName: '',
  confirm: null,
});

function closeModal() {
  if (busy.value) return;
  modal.capture = null;
  modal.confirm = null;
}

function applyOverview(overview: CliView[]) {
  clis.value = overview.sort((a, b) => {
    if (a.id === 'kimi') return -1;
    if (b.id === 'kimi') return 1;
    return 0;
  });
}

async function loadOverview() {
  const overview = await api.overview();
  applyOverview(overview);
}

async function refresh() {
  loading.value = true;
  try {
    await loadOverview();
    await api.refreshUsage();
    await loadOverview();
  } catch (e: any) {
    toast('err', t('loadFailed', { msg: e?.message || e }));
  } finally {
    loading.value = false;
  }
}

let cancelOverviewListener: (() => void) | null = null;
let cancelUsageListener: (() => void) | null = null;

onMounted(() => {
  refresh();
  const eventsOn = window.runtime?.EventsOn;
  if (typeof eventsOn === 'function') {
    cancelOverviewListener = eventsOn('overview-changed', () => { loadOverview().catch(() => {}); });
    cancelUsageListener = eventsOn('usage-updated', () => { loadOverview().catch(() => {}); });
  }
});

onBeforeUnmount(() => {
  cancelOverviewListener?.();
  cancelUsageListener?.();
});

async function openStore() {
  try { await api.openStoreDir(); } catch (e: any) { toast('err', String(e?.message || e)); }
}

function openCapture(cli: CliView) {
  modal.capture = cli;
  modal.captureName = '';
  nextTick(() => captureNameInput.value?.focus());
}
async function submitCapture() {
  if (!modal.capture || !modal.captureName) return;
  busy.value = true;
  try {
    await api.capture(modal.capture.id, modal.captureName);
    toast('ok', t('capturedToast', { name: modal.captureName }));
    modal.capture = null;
    await refresh();
  } catch (e: any) {
    toast('err', String(e?.message || e));
  } finally {
    busy.value = false;
  }
}

async function doSwitch(cli: CliView, p: ProfileView) {
  busy.value = true;
  try {
    const res: SwitchResult | null = await api.switchTo(cli.id, p.name);
    if (res) {
      // 切换成功不再提示；仅当出现告警（回写失败等）时提醒
      if (res.noOp) {
        toast('info', t('alreadyActive', { name: p.name }));
      } else if (res.warnings?.length) {
        toast('err', t('notePrefix', { msg: res.warnings.join(listSep()) }));
      }
    }
    await refresh();
  } catch (e: any) {
    toast('err', String(e?.message || e));
    await refresh();
  } finally {
    busy.value = false;
  }
}

function doRecapture(cli: CliView, p: ProfileView) {
  modal.confirm = {
    title: t('recaptureModalTitle', { name: p.name }),
    body: t('recaptureModalBody', { path: cli.livePath }),
    okText: t('overwriteSnapshot'),
    ok: async () => {
      busy.value = true;
      try {
        await api.recapture(cli.id, p.name);
        toast('ok', t('snapshotUpdated', { name: p.name }));
        modal.confirm = null;
        await refresh();
      } catch (e: any) {
        toast('err', String(e?.message || e));
      } finally {
        busy.value = false;
      }
    },
  };
}

function confirmDelete(cli: CliView, p: ProfileView) {
  modal.confirm = {
    title: t('deleteModalTitle', { name: p.name }),
    body: p.isActive ? t('deleteActiveBody') : t('deleteBody'),
    okText: t('deleteLabel'),
    ok: async () => {
      busy.value = true;
      try {
        await api.remove(cli.id, p.name);
        toast('ok', t('deletedToast', { name: p.name }));
        modal.confirm = null;
        await refresh();
      } catch (e: any) {
        toast('err', String(e?.message || e));
      } finally {
        busy.value = false;
      }
    },
  };
}
</script>
