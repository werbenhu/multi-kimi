import {ref} from 'vue';

export type Lang = 'zh' | 'en';

// 界面文案字典。插值用 {name} 占位。
const messages = {
  loggedIn: {zh: '已登录', en: 'Logged in'},
  loggedOut: {zh: '未登录', en: 'Logged out'},
  openStoreTitle: {zh: '打开 ~/.multi-kimi', en: 'Open ~/.multi-kimi'},
  storeDir: {zh: '存储目录', en: 'Data folder'},
  refresh: {zh: '刷新', en: 'Refresh'},
  refreshing: {zh: '刷新中', en: 'Refreshing'},
  saveAccount: {zh: '保存账号', en: 'Save account'},
  switchMayFail: {zh: '切换可能不生效', en: 'Switch may not take effect'},
  emptyHasLive: {
    zh: '还没有账号，点击右上角「保存账号」开始保存。',
    en: 'No accounts yet — click "Save account" in the top right to save the current login.',
  },
  emptyNoLiveA: {
    zh: '还没有账号，且未检测到 live 凭据。请先在终端登录一次',
    en: 'No accounts yet and no live credential detected. Log in once in the terminal with',
  },
  emptyNoLiveB: {zh: '，再回到这里保存。', en: ', then come back here to save it.'},
  savedAccounts: {zh: '已保存账号', en: 'Saved accounts'},
  profilesMeta: {zh: '{count} 个 · 切换后请重启 {name}', en: '{count} · restart {name} after switching'},
  missingSnapshot: {zh: '快照缺失', en: 'Snapshot missing'},
  currentLabel: {zh: '当前', en: 'Current'},
  updatedAt: {zh: '更新于 {time}', en: 'Updated {time}'},
  rotatedNote: {
    zh: 'live 已是其他账号的凭据（如需覆盖快照请重新捕获）',
    en: 'Live credential now belongs to another account (re-capture to overwrite this snapshot)',
  },
  missingLiveNote: {zh: '未登录或已登出', en: 'Not logged in or already logged out'},
  switchLabel: {zh: '切换', en: 'Switch'},
  recapture: {zh: '重新捕获', en: 'Re-capture'},
  recaptureTip: {zh: '用当前 live 凭据刷新 {name}', en: 'Refresh {name} with the current live credential'},
  deleteAccountTip: {zh: '删除账号', en: 'Delete account'},
  deleteAria: {zh: '删除 {name}', en: 'Delete {name}'},
  captureTitle: {zh: '保存当前凭据', en: 'Save current credential'},
  nameLabel: {zh: '名称', en: 'Name'},
  namePlaceholder: {zh: '例如：work / personal / 账号2', en: 'e.g. work / personal / account2'},
  cancel: {zh: '取消', en: 'Cancel'},
  save: {zh: '保存', en: 'Save'},
  saving: {zh: '保存中…', en: 'Saving…'},
  working: {zh: '执行中…', en: 'Working…'},
  ok: {zh: '确定', en: 'OK'},
  loadFailed: {zh: '加载失败: {msg}', en: 'Failed to load: {msg}'},
  capturedToast: {zh: '已保存账号「{name}」', en: 'Account "{name}" saved'},
  notePrefix: {zh: '注意：{msg}', en: 'Note: {msg}'},
  alreadyActive: {zh: '「{name}」已是当前账号', en: '"{name}" is already active'},
  switchedTo: {zh: '已切换到「{name}」', en: 'Switched to "{name}"'},
  recaptureModalTitle: {zh: '重新捕获「{name}」', en: 'Re-capture "{name}"'},
  recaptureModalBody: {
    zh: '将用当前 live 凭据（{path}）覆盖该账号的快照。此操作不可撤销，确定继续？',
    en: 'The snapshot will be overwritten with the current live credential ({path}). This cannot be undone. Continue?',
  },
  overwriteSnapshot: {zh: '覆盖快照', en: 'Overwrite'},
  snapshotUpdated: {zh: '「{name}」快照已更新', en: 'Snapshot of "{name}" updated'},
  deleteModalTitle: {zh: '删除账号「{name}」', en: 'Delete account "{name}"'},
  deleteActiveBody: {
    zh: '该账号正在使用中。删除后 active 指针将被清除，但 live 凭据文件不受影响。确定删除？',
    en: 'This account is currently active. Deleting clears the active pointer; the live credential file is not affected. Delete anyway?',
  },
  deleteBody: {
    zh: '只删除本工具保存的快照，不会影响 live 凭据文件。确定删除？',
    en: 'Only the snapshot saved by this tool is deleted; the live credential file is not affected. Delete?',
  },
  deleteLabel: {zh: '删除', en: 'Delete'},
  deletedToast: {zh: '已删除「{name}」', en: '"{name}" deleted'},
  // UsageMeters
  fiveHour: {zh: '5小时', en: '5-hour'},
  weekly: {zh: '周用量', en: 'Weekly'},
  usageLoading: {zh: '用量加载中…', en: 'Loading usage…'},
  usageUnauthorized: {zh: '凭据失效，请重新登录后保存', en: 'Credential expired — log in again and re-save'},
  usageEmpty: {zh: '未获取到用量', en: 'No usage data'},
  usageMissing: {zh: '无凭据，无法查询用量', en: 'No credential, cannot query usage'},
  usageFailed: {zh: '用量获取失败', en: 'Failed to fetch usage'},
} as const;

export type MessageKey = keyof typeof messages;

const stored = typeof localStorage !== 'undefined' ? localStorage.getItem('multi-kimi-lang') : null;
export const lang = ref<Lang>(stored === 'en' ? 'en' : 'zh');

export function toggleLang() {
  lang.value = lang.value === 'zh' ? 'en' : 'zh';
  try {
    localStorage.setItem('multi-kimi-lang', lang.value);
  } catch {
    // 隐私模式下写不进去就算了，保持当次生效
  }
}

export function t(key: MessageKey, vars?: Record<string, string | number>): string {
  let s: string = messages[key][lang.value];
  if (vars) {
    for (const [k, v] of Object.entries(vars)) {
      s = s.split(`{${k}}`).join(String(v));
    }
  }
  return s;
}

// 列表项之间的连接符，中文用全角分号。
export function listSep(): string {
  return lang.value === 'zh' ? '；' : '; ';
}
