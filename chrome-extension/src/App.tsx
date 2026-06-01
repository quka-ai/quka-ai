import type { PartialBlock } from '@blocknote/core';
import { BookOpenText, Check, Copy, FileText, Loader2, LogIn, LogOut, RefreshCw, Save, Sparkles, Wand2 } from 'lucide-react';
import { useCallback, useEffect, useMemo, useState } from 'react';

import { MemoryEditor } from '@/components/editor/MemoryEditor';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select } from '@/components/ui/select';
import { Tabs } from '@/components/ui/tabs';
import { Textarea } from '@/components/ui/textarea';
import { blocksText, emptyBlocks, pageToBlocks, summaryToBlocks } from '@/lib/blocks';
import {
  createKnowledge,
  getUser,
  listResources,
  listSpaces,
  loginWithAccessToken,
  loginWithPassword,
  normalizeHost,
  summarizePage
} from '@/lib/api';
import { readCurrentPage } from '@/lib/page';
import { clearSettings, defaultSettings, loadSettings, saveSettings } from '@/lib/storage';
import type { ExtensionSettings, PageSnapshot, QukaUser, Resource, UserSpace } from '@/lib/types';
import { formatError } from '@/lib/utils';

type ActiveTab = 'summary' | 'memory';

export default function App() {
  const [settings, setSettings] = useState<ExtensionSettings>(defaultSettings);
  const [hostInput, setHostInput] = useState(defaultSettings.host);
  const [tokenInput, setTokenInput] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [user, setUser] = useState<QukaUser | null>(null);
  const [spaces, setSpaces] = useState<UserSpace[]>([]);
  const [resources, setResources] = useState<Resource[]>([]);
  const [snapshot, setSnapshot] = useState<PageSnapshot | null>(null);
  const [summary, setSummary] = useState('');
  const [memoryBlocks, setMemoryBlocks] = useState<PartialBlock[]>(emptyBlocks());
  const [tab, setTab] = useState<ActiveTab>('summary');
  const [busy, setBusy] = useState('');
  const [notice, setNotice] = useState('');
  const [error, setError] = useState('');

  const selectedSpace = useMemo(() => spaces.find(space => space.space_id === settings.selectedSpaceId), [settings.selectedSpaceId, spaces]);
  const isAuthed = Boolean(user && settings.token);

  const updateSettings = useCallback(async (next: ExtensionSettings) => {
    setSettings(next);
    setHostInput(next.host);
    setTokenInput(next.token);
    await saveSettings(next);
  }, []);

  const loadAccountData = useCallback(
    async (baseSettings: ExtensionSettings, keepSpace = true) => {
      const normalizedSettings = {
        ...baseSettings,
        host: normalizeHost(baseSettings.host)
      };
      const [nextUser, nextSpaces] = await Promise.all([getUser(normalizedSettings), listSpaces(normalizedSettings)]);
      const selectedSpaceId =
        keepSpace && nextSpaces.some(space => space.space_id === normalizedSettings.selectedSpaceId) ? normalizedSettings.selectedSpaceId : nextSpaces[0]?.space_id || '';
      const nextResources = selectedSpaceId ? await listResources(normalizedSettings, selectedSpaceId) : [];
      const selectedResourceId =
        normalizedSettings.selectedResourceId && nextResources.some(item => item.id === normalizedSettings.selectedResourceId)
          ? normalizedSettings.selectedResourceId
          : nextResources[0]?.id || 'knowledge';
      const nextSettings = {
        ...normalizedSettings,
        selectedSpaceId,
        selectedResourceId
      };

      setUser(nextUser);
      setSpaces(nextSpaces);
      setResources(nextResources);
      await updateSettings(nextSettings);
    },
    [updateSettings]
  );

  useEffect(() => {
    loadSettings()
      .then(async stored => {
        setSettings(stored);
        setHostInput(stored.host);
        setTokenInput(stored.token);

        if (stored.token) {
          await loadAccountData(stored);
        }
      })
      .catch(err => setError(formatError(err)));

    readCurrentPage()
      .then(page => setSnapshot(page))
      .catch(err => setNotice(formatError(err)));
  }, [loadAccountData]);

  async function handleAccessTokenLogin() {
    await run('login', async () => {
      const nextSettings = {
        ...settings,
        host: normalizeHost(hostInput),
        authType: 'access' as const,
        token: tokenInput.trim()
      };
      const nextUser = await loginWithAccessToken(nextSettings, nextSettings.token);

      setUser(nextUser);
      await loadAccountData(nextSettings, false);
      setNotice('已登录');
    });
  }

  async function handlePasswordLogin() {
    await run('login', async () => {
      const baseSettings = {
        ...settings,
        host: normalizeHost(hostInput),
        authType: 'login' as const,
        token: ''
      };
      const login = await loginWithPassword(baseSettings, email.trim(), password);
      const nextSettings = {
        ...baseSettings,
        token: login.token
      };

      setUser(login.meta);
      setPassword('');
      setTokenInput(login.token);
      await loadAccountData(nextSettings, false);
      setNotice('已登录');
    });
  }

  async function handleRefreshPage() {
    await run('page', async () => {
      const page = await readCurrentPage();
      setSnapshot(page);
      setNotice('已读取当前页');
    });
  }

  async function handleRefreshAccount() {
    await run('account', async () => {
      await loadAccountData({
        ...settings,
        host: normalizeHost(hostInput)
      });
      setNotice('已刷新');
    });
  }

  async function handleSpaceChange(spaceId: string) {
    await run('space', async () => {
      const nextResources = spaceId ? await listResources(settings, spaceId) : [];
      const nextSettings = {
        ...settings,
        selectedSpaceId: spaceId,
        selectedResourceId: nextResources[0]?.id || 'knowledge'
      };

      setResources(nextResources);
      await updateSettings(nextSettings);
    });
  }

  async function handleSummarize() {
    if (!snapshot) {
      return;
    }

    await run('summary', async () => {
      const result = await summarizePage(settings, settings.selectedSpaceId, snapshot);
      setSummary(result.text);
      setTab('summary');
      setNotice('总结完成');
    });
  }

  async function handleSaveMemory(blocks = memoryBlocks) {
    await run('memory', async () => {
      if (!blocksText(blocks)) {
        throw new Error('记忆内容为空');
      }

      const id = await createKnowledge(settings, settings.selectedSpaceId, settings.selectedResourceId || 'knowledge', blocks);
      setNotice(`已记录: ${id}`);
      setMemoryBlocks(emptyBlocks());
    });
  }

  async function handleSaveSummary() {
    if (!snapshot || !summary.trim()) {
      return;
    }

    await handleSaveMemory(summaryToBlocks(snapshot, summary));
  }

  async function handleLogout() {
    await clearSettings();
    setSettings(defaultSettings);
    setHostInput(defaultSettings.host);
    setTokenInput('');
    setUser(null);
    setSpaces([]);
    setResources([]);
    setNotice('已退出');
  }

  async function handleCopySummary() {
    if (!summary) {
      return;
    }

    await navigator.clipboard.writeText(summary);
    setNotice('已复制');
  }

  async function run(scope: string, task: () => Promise<void>) {
    setBusy(scope);
    setError('');
    setNotice('');

    try {
      await task();
    } catch (err) {
      setError(formatError(err));
    } finally {
      setBusy('');
    }
  }

  const authControls =
    settings.authType === 'access' ? (
      <div className="grid gap-2">
        <Label htmlFor="token">Access Token</Label>
        <Input id="token" type="password" value={tokenInput} onChange={event => setTokenInput(event.target.value)} />
        <Button type="button" className="gap-2" disabled={busy === 'login' || !tokenInput.trim()} onClick={handleAccessTokenLogin}>
          {busy === 'login' ? <Loader2 className="h-4 w-4 animate-spin" /> : <LogIn className="h-4 w-4" />}
          登录
        </Button>
      </div>
    ) : (
      <div className="grid gap-2">
        <Label htmlFor="email">邮箱</Label>
        <Input id="email" type="email" value={email} onChange={event => setEmail(event.target.value)} />
        <Label htmlFor="password">密码</Label>
        <Input id="password" type="password" value={password} onChange={event => setPassword(event.target.value)} />
        <Button type="button" className="gap-2" disabled={busy === 'login' || !email.trim() || !password} onClick={handlePasswordLogin}>
          {busy === 'login' ? <Loader2 className="h-4 w-4 animate-spin" /> : <LogIn className="h-4 w-4" />}
          登录
        </Button>
      </div>
    );

  return (
    <main className="flex h-[640px] w-[430px] flex-col bg-background text-foreground">
      <header className="flex items-center justify-between border-b px-4 py-3">
        <div>
          <div className="text-sm font-semibold">Quka Quick Memory</div>
          <div className="mt-1 text-xs text-muted-foreground">{selectedSpace?.title || selectedSpace?.space_id || 'No space selected'}</div>
        </div>
        <div className="flex items-center gap-2">
          <Badge variant={isAuthed ? 'default' : 'secondary'}>{isAuthed ? user?.user_name || '已登录' : '未登录'}</Badge>
          {isAuthed && (
            <Button type="button" variant="ghost" size="icon" title="退出" onClick={handleLogout}>
              <LogOut className="h-4 w-4" />
            </Button>
          )}
        </div>
      </header>

      <div className="flex-1 space-y-3 overflow-auto p-3">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle>连接</CardTitle>
          </CardHeader>
          <CardContent className="grid gap-3">
            <div className="grid gap-2">
              <Label htmlFor="host">HOST</Label>
              <Input id="host" value={hostInput} placeholder="http://localhost:33033/api/v1" onChange={event => setHostInput(event.target.value)} />
            </div>
            <div className="grid grid-cols-[120px_1fr] gap-2">
              <Select
                value={settings.authType}
                onChange={event =>
                  updateSettings({
                    ...settings,
                    authType: event.target.value as ExtensionSettings['authType']
                  })
                }
              >
                <option value="access">Access Token</option>
                <option value="login">邮箱密码</option>
              </Select>
              {isAuthed ? (
                <Button type="button" variant="outline" className="gap-2" disabled={busy === 'account'} onClick={handleRefreshAccount}>
                  {busy === 'account' ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
                  刷新账号
                </Button>
              ) : (
                authControls
              )}
            </div>
            {isAuthed && (
              <div className="grid grid-cols-2 gap-2">
                <div className="grid gap-2">
                  <Label htmlFor="space">空间</Label>
                  <Select id="space" value={settings.selectedSpaceId} onChange={event => handleSpaceChange(event.target.value)}>
                    {spaces.map(space => (
                      <option key={space.space_id} value={space.space_id}>
                        {space.title || space.space_id}
                      </option>
                    ))}
                  </Select>
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="resource">资源</Label>
                  <Select
                    id="resource"
                    value={settings.selectedResourceId}
                    onChange={event =>
                      updateSettings({
                        ...settings,
                        selectedResourceId: event.target.value
                      })
                    }
                  >
                    {resources.length === 0 && <option value="knowledge">knowledge</option>}
                    {resources.map(resource => (
                      <option key={resource.id} value={resource.id}>
                        {resource.title || resource.id}
                      </option>
                    ))}
                  </Select>
                </div>
              </div>
            )}
          </CardContent>
        </Card>

        <section className="space-y-3">
          <Tabs
            value={tab}
            onValueChange={setTab}
            tabs={[
              { value: 'summary', label: '总结', icon: <Sparkles className="h-4 w-4" /> },
              { value: 'memory', label: '记忆', icon: <BookOpenText className="h-4 w-4" /> }
            ]}
          />

          {tab === 'summary' ? (
            <div className="space-y-3">
              <div className="rounded-lg border bg-card p-3">
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <div className="truncate text-sm font-medium">{snapshot?.title || 'Current page'}</div>
                    <div className="mt-1 truncate text-xs text-muted-foreground">{snapshot?.url || ''}</div>
                  </div>
                  <Button type="button" variant="outline" size="icon" title="读取当前页" disabled={busy === 'page'} onClick={handleRefreshPage}>
                    {busy === 'page' ? <Loader2 className="h-4 w-4 animate-spin" /> : <FileText className="h-4 w-4" />}
                  </Button>
                </div>
                <div className="mt-2 flex gap-2 text-xs text-muted-foreground">
                  <span>{snapshot?.selection ? `选中 ${snapshot.selection.length}` : '无选中'}</span>
                  <span>{snapshot?.text ? `正文 ${snapshot.text.length}` : '正文 0'}</span>
                </div>
              </div>

              <div className="flex gap-2">
                <Button type="button" className="flex-1 gap-2" disabled={!isAuthed || !settings.selectedSpaceId || !snapshot || busy === 'summary'} onClick={handleSummarize}>
                  {busy === 'summary' ? <Loader2 className="h-4 w-4 animate-spin" /> : <Wand2 className="h-4 w-4" />}
                  总结当前页
                </Button>
                <Button type="button" variant="outline" size="icon" title="复制总结" disabled={!summary} onClick={handleCopySummary}>
                  <Copy className="h-4 w-4" />
                </Button>
                <Button type="button" variant="outline" size="icon" title="保存总结" disabled={!isAuthed || !summary || busy === 'memory'} onClick={handleSaveSummary}>
                  {busy === 'memory' ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
                </Button>
              </div>

              <Textarea readOnly value={summary} placeholder="总结结果" className="min-h-[190px] resize-none" />
            </div>
          ) : (
            <div className="space-y-3">
              <div className="flex gap-2">
                <Button
                  type="button"
                  variant="outline"
                  className="flex-1 gap-2"
                  disabled={!snapshot}
                  onClick={() => snapshot && setMemoryBlocks(pageToBlocks(snapshot, snapshot.selection ? 'selection' : 'page'))}
                >
                  <FileText className="h-4 w-4" />
                  载入页面
                </Button>
                <Button type="button" variant="outline" size="icon" title="清空" onClick={() => setMemoryBlocks(emptyBlocks())}>
                  <RefreshCw className="h-4 w-4" />
                </Button>
              </div>

              <MemoryEditor value={memoryBlocks} onChange={setMemoryBlocks} />

              <Button type="button" className="w-full gap-2" disabled={!isAuthed || !settings.selectedSpaceId || busy === 'memory'} onClick={() => handleSaveMemory()}>
                {busy === 'memory' ? <Loader2 className="h-4 w-4 animate-spin" /> : <Check className="h-4 w-4" />}
                记录新记忆
              </Button>
            </div>
          )}
        </section>
      </div>

      {(notice || error) && (
        <footer className="border-t px-4 py-2 text-xs">
          {notice && <div className="text-emerald-700">{notice}</div>}
          {error && <div className="text-destructive">{error}</div>}
        </footer>
      )}
    </main>
  );
}
