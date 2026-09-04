// 与 Wails 后端的桥接封装。
// 直接调用 window.go.main.App.<Method>（Wails 运行时注入），
// 不依赖构建期生成的 wailsjs 绑定，避免首次构建的先有鸡先有蛋问题。

export interface UsageWindow {
  kind: string;
  label: string;
  usedPercent: number;
  remainingPercent: number;
  resetsAt?: string;
  resetsIn?: string;
}

export interface KimiUsage {
  status: string; // ok | loading | missing | unauthorized | unavailable | empty
  plan?: string;
  session?: UsageWindow;
  weekly?: UsageWindow;
  error?: string;
  fetchedAt?: string;
}

export interface ProfileView {
  name: string;
  createdAt: string;
  updatedAt: string;
  hasCredential: boolean;
  isActive: boolean;
  freshness: string; // fresh | rotated | missing-live | missing-snap | ""
  freshnessNote: string;
  usage?: KimiUsage;
}

export interface CliView {
  id: string;
  name: string;
  livePath: string;
  livePathShort: string;
  liveExists: boolean;
  activeProfile: string;
  profiles: ProfileView[];
  envWarnings: string[];
  liveUsage?: KimiUsage;
}

export interface SwitchResult {
  from: string;
  to: string;
  noOp: boolean;
  warnings: string[];
}

declare global {
  interface Window {
    go: any;
    runtime: any;
  }
}

function call<T>(method: string, ...args: any[]): Promise<T> {
  const fn = window.go?.main?.App?.[method];
  if (typeof fn !== 'function') {
    return Promise.reject(new Error(`后端方法不可用: ${method}`));
  }
  return fn(...args) as Promise<T>;
}

export const api = {
  overview: () => call<CliView[]>('GetOverview'),
  capture: (cli: string, name: string) =>
    call<void>('CaptureProfile', cli, name),
  recapture: (cli: string, name: string) =>
    call<void>('RecaptureProfile', cli, name),
  switchTo: (cli: string, name: string) =>
    call<SwitchResult | null>('SwitchProfile', cli, name),
  remove: (cli: string, name: string) =>
    call<void>('DeleteProfile', cli, name),
  openStoreDir: () => call<void>('OpenStoreDir'),
  openLiveDir: (cli: string) => call<void>('OpenLiveDir', cli),
  openLiveFile: (cli: string) => call<void>('OpenLiveFile', cli),
  appInfo: () => call<Record<string, string>>('AppInfo'),
  refreshUsage: () => call<void>('RefreshUsage'),
};
