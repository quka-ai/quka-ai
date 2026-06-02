import type { PartialBlock } from '@blocknote/core';
import {
  BookOpenText,
  Check,
  Copy,
  FileText,
  Loader2,
  LogIn,
  LogOut,
  MessageSquare,
  Plus,
  RefreshCw,
  Save,
  SendHorizontal,
  Sparkles,
  Wand2
} from 'lucide-react';
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
  createChatSession,
  createKnowledge,
  getChatHistory,
  getUser,
  listChatSessions,
  listResources,
  listSpaces,
  loginWithAccessToken,
  loginWithPassword,
  normalizeHost,
  sendChatMessageAndWait,
  summarizePage
} from '@/lib/api';
import { readCurrentPage } from '@/lib/page';
import { clearSettings, defaultSettings, loadSettings, saveSettings } from '@/lib/storage';
import type { ChatMessageDetail, ChatSession, ExtensionSettings, PageSnapshot, QukaUser, Resource, UserSpace } from '@/lib/types';
import { cn, formatError } from '@/lib/utils';

type ActiveTab = 'summary' | 'memory' | 'chat';

const ROLE_USER = 1;
const ROLE_ASSISTANT = 2;
const ROLE_TOOL = 4;
const MESSAGE_PROGRESS_GENERATING = 3;

function messageText(item: ChatMessageDetail) {
  return item.meta.message?.text || '';
}

function formatSessionTitle(session: ChatSession) {
  if (session.title?.trim()) {
    return session.title.trim();
  }

  if (session.latest_access_time) {
    return new Date(session.latest_access_time * 1000).toLocaleString('zh-CN', {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit'
    });
  }

  return session.id;
}

function createLocalSession(sessionId: string, spaceId: string, userId = ''): ChatSession {
  const now = Math.floor(Date.now() / 1000);

  return {
    id: sessionId,
    space_id: spaceId,
    user_id: userId,
    title: '新会话',
    session_type: 1,
    status: 2,
    created_at: now,
    latest_access_time: now
  };
}

function createLocalMessage(spaceId: string, sessionId: string, userId: string, role: number, text: string, complete = 1): ChatMessageDetail {
  const now = Math.floor(Date.now() / 1000);

  return {
    meta: {
      message_id: `local-${role}-${Date.now()}`,
      sequence: now,
      send_time: now,
      role,
      user_id: userId,
      session_id: sessionId,
      space_id: spaceId,
      complete,
      message_type: 1,
      message: {
        text
      },
      attach: []
    },
    ext: null
  };
}

export default function App() {
  const [settings, setSettings] = useState<ExtensionSettings>(defaultSettings);
  const [settingsLoaded, setSettingsLoaded] = useState(false);
  const [accountReady, setAccountReady] = useState(false);
  const [hostInput, setHostInput] = useState(defaultSettings.host);
  const [tokenInput, setTokenInput] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [user, setUser] = useState<QukaUser | null>(null);
  const [spaces, setSpaces] = useState<UserSpace[]>([]);
  const [resources, setResources] = useState<Resource[]>([]);
  const [chatSessions, setChatSessions] = useState<ChatSession[]>([]);
  const [chatMessages, setChatMessages] = useState<ChatMessageDetail[]>([]);
  const [chatInput, setChatInput] = useState('');
  const [snapshot, setSnapshot] = useState<PageSnapshot | null>(null);
  const [summary, setSummary] = useState('');
  const [memoryBlocks, setMemoryBlocks] = useState<PartialBlock[]>(emptyBlocks());
  const [tab, setTab] = useState<ActiveTab>('summary');
  const [busy, setBusy] = useState('');
  const [notice, setNotice] = useState('');
  const [error, setError] = useState('');

  const selectedSpace = useMemo(() => spaces.find(space => space.space_id === settings.selectedSpaceId), [settings.selectedSpaceId, spaces]);
  const selectedSession = useMemo(() => chatSessions.find(session => session.id === settings.selectedChatSessionId), [chatSessions, settings.selectedChatSessionId]);
  const isAuthed = Boolean(user && settings.token);
  const isCheckingAuth = !settingsLoaded || (Boolean(settings.token) && !accountReady && !user);

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
      let nextResources: Resource[] = [];
      let nextChatSessions: ChatSession[] = [];

      if (selectedSpaceId) {
        const [resourceList, sessionData] = await Promise.all([listResources(normalizedSettings, selectedSpaceId), listChatSessions(normalizedSettings, selectedSpaceId)]);
        nextResources = resourceList;
        nextChatSessions = sessionData.list;
      }

      const selectedResourceId =
        normalizedSettings.selectedResourceId && nextResources.some(item => item.id === normalizedSettings.selectedResourceId)
          ? normalizedSettings.selectedResourceId
          : nextResources[0]?.id || 'knowledge';
      const selectedChatSessionId =
        normalizedSettings.selectedChatSessionId && nextChatSessions.some(item => item.id === normalizedSettings.selectedChatSessionId)
          ? normalizedSettings.selectedChatSessionId
          : nextChatSessions[0]?.id || '';
      const nextSettings = {
        ...normalizedSettings,
        selectedSpaceId,
        selectedResourceId,
        selectedChatSessionId
      };

      setUser(nextUser);
      setSpaces(nextSpaces);
      setResources(nextResources);
      setChatSessions(nextChatSessions);

      if (selectedSpaceId && selectedChatSessionId) {
        const history = await getChatHistory(nextSettings, selectedSpaceId, selectedChatSessionId);
        setChatMessages(history.list || []);
      } else {
        setChatMessages([]);
      }

      await updateSettings(nextSettings);
    },
    [updateSettings]
  );

  useEffect(() => {
    let active = true;

    loadSettings()
      .then(async stored => {
        if (!active) {
          return;
        }

        setSettingsLoaded(true);
        setSettings(stored);
        setHostInput(stored.host);
        setTokenInput(stored.token);

        if (!stored.token) {
          setAccountReady(true);
          return;
        }

        setAccountReady(false);
        try {
          await loadAccountData(stored);
        } catch (err) {
          if (active) {
            setUser(null);
            setSpaces([]);
            setResources([]);
            setChatSessions([]);
            setChatMessages([]);
            setError(formatError(err));
          }
        } finally {
          if (active) {
            setAccountReady(true);
          }
        }
      })
      .catch(err => {
        if (active) {
          setSettingsLoaded(true);
          setAccountReady(true);
          setError(formatError(err));
        }
      });

    readCurrentPage()
      .then(page => {
        if (active) {
          setSnapshot(page);
        }
      })
      .catch(err => {
        if (active) {
          setNotice(formatError(err));
        }
      });

    return () => {
      active = false;
    };
  }, [loadAccountData]);

  async function loadChatSessionHistory(spaceId: string, sessionId: string, baseSettings = settings) {
    if (!spaceId || !sessionId) {
      setChatMessages([]);
      return;
    }

    const history = await getChatHistory(baseSettings, spaceId, sessionId);
    setChatMessages(history.list || []);
  }

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
      let nextResources: Resource[] = [];
      let nextChatSessions: ChatSession[] = [];

      if (spaceId) {
        const [resourceList, sessionData] = await Promise.all([listResources(settings, spaceId), listChatSessions(settings, spaceId)]);
        nextResources = resourceList;
        nextChatSessions = sessionData.list;
      }

      const selectedChatSessionId = nextChatSessions[0]?.id || '';
      const nextSettings = {
        ...settings,
        selectedSpaceId: spaceId,
        selectedResourceId: nextResources[0]?.id || 'knowledge',
        selectedChatSessionId
      };

      setResources(nextResources);
      setChatSessions(nextChatSessions);
      await updateSettings(nextSettings);
      await loadChatSessionHistory(spaceId, selectedChatSessionId, nextSettings);
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

  async function handleNewChatSession() {
    if (!settings.selectedSpaceId || !user) {
      return;
    }

    await run('session', async () => {
      const sessionId = await createChatSession(settings, settings.selectedSpaceId);
      const sessionData = await listChatSessions(settings, settings.selectedSpaceId);
      const fallback = createLocalSession(sessionId, settings.selectedSpaceId, user.user_id);
      const nextChatSessions = sessionData.list.some(item => item.id === sessionId) ? sessionData.list : [fallback, ...sessionData.list];
      const nextSettings = {
        ...settings,
        selectedChatSessionId: sessionId
      };

      setChatSessions(nextChatSessions);
      setChatMessages([]);
      await updateSettings(nextSettings);
      setTab('chat');
      setNotice('已新建会话');
    });
  }

  async function handleChatSessionChange(sessionId: string) {
    await run('history', async () => {
      const nextSettings = {
        ...settings,
        selectedChatSessionId: sessionId
      };

      await updateSettings(nextSettings);
      await loadChatSessionHistory(settings.selectedSpaceId, sessionId, nextSettings);
    });
  }

  async function handleRefreshChatHistory() {
    await run('history', async () => {
      await loadChatSessionHistory(settings.selectedSpaceId, settings.selectedChatSessionId);
      setNotice('已刷新会话');
    });
  }

  async function handleSendChat() {
    const text = chatInput.trim();

    if (!text || !settings.selectedSpaceId || !user) {
      return;
    }

    await run('chat', async () => {
      let activeSessionId = settings.selectedChatSessionId;
      let requestSettings = settings;

      if (!activeSessionId) {
        activeSessionId = await createChatSession(settings, settings.selectedSpaceId);
        const fallback = createLocalSession(activeSessionId, settings.selectedSpaceId, user.user_id);
        const nextSettings = {
          ...settings,
          selectedChatSessionId: activeSessionId
        };

        requestSettings = nextSettings;
        setChatSessions(prev => (prev.some(item => item.id === activeSessionId) ? prev : [fallback, ...prev]));
        await updateSettings(nextSettings);
      }

      const pending = createLocalMessage(settings.selectedSpaceId, activeSessionId, user.user_id, ROLE_ASSISTANT, '生成中...', MESSAGE_PROGRESS_GENERATING);

      setChatInput('');
      setChatMessages(prev => [...prev, createLocalMessage(settings.selectedSpaceId, activeSessionId, user.user_id, ROLE_USER, text), pending]);

      try {
        await sendChatMessageAndWait(
          requestSettings,
          settings.selectedSpaceId,
          activeSessionId,
          text,
          {
            enableKnowledge: true,
            enableSearch: false,
            enableThinking: false
          },
          history => setChatMessages(history.list || [])
        );

        await loadChatSessionHistory(settings.selectedSpaceId, activeSessionId, requestSettings);
        const sessionData = await listChatSessions(requestSettings, settings.selectedSpaceId);
        setChatSessions(sessionData.list.some(item => item.id === activeSessionId) ? sessionData.list : [createLocalSession(activeSessionId, settings.selectedSpaceId, user.user_id), ...sessionData.list]);
      } catch (err) {
        setChatMessages(prev => prev.filter(item => item.meta.message_id !== pending.meta.message_id));
        throw err;
      }
    });
  }

  async function handleLogout() {
    await clearSettings();
    setSettings(defaultSettings);
    setSettingsLoaded(true);
    setAccountReady(true);
    setHostInput(defaultSettings.host);
    setTokenInput('');
    setEmail('');
    setPassword('');
    setUser(null);
    setSpaces([]);
    setResources([]);
    setChatSessions([]);
    setChatMessages([]);
    setError('');
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
        {isAuthed ? (
          <Card>
            <CardHeader className="flex-row items-center justify-between pb-2">
              <CardTitle>工作区</CardTitle>
              <Button type="button" variant="outline" size="icon" title="刷新账号" disabled={busy === 'account'} onClick={handleRefreshAccount}>
                {busy === 'account' ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
              </Button>
            </CardHeader>
            <CardContent className="grid grid-cols-2 gap-2">
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
            </CardContent>
          </Card>
        ) : (
          <Card>
            <CardHeader className="pb-2">
              <CardTitle>连接</CardTitle>
            </CardHeader>
            <CardContent className="grid gap-3">
              {isCheckingAuth ? (
                <div className="flex h-20 items-center justify-center gap-2 text-sm text-muted-foreground">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  正在恢复登录态
                </div>
              ) : (
                <>
                  <div className="grid gap-2">
                    <Label htmlFor="host">HOST</Label>
                    <Input id="host" value={hostInput} placeholder="http://localhost:33033/api/v1" onChange={event => setHostInput(event.target.value)} />
                  </div>
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
                  {authControls}
                </>
              )}
            </CardContent>
          </Card>
        )}

        <section className="space-y-3">
          <Tabs
            value={tab}
            onValueChange={setTab}
            tabs={[
              { value: 'summary', label: '总结', icon: <Sparkles className="h-4 w-4" /> },
              { value: 'memory', label: '记忆', icon: <BookOpenText className="h-4 w-4" /> },
              { value: 'chat', label: '对话', icon: <MessageSquare className="h-4 w-4" /> }
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
          ) : tab === 'memory' ? (
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
          ) : (
            <div className="space-y-3">
              <div className="grid grid-cols-[1fr_auto_auto] gap-2">
                <Select
                  value={settings.selectedChatSessionId}
                  disabled={!isAuthed || chatSessions.length === 0 || busy === 'session'}
                  onChange={event => handleChatSessionChange(event.target.value)}
                >
                  {chatSessions.length === 0 && <option value="">新会话</option>}
                  {chatSessions.map(session => (
                    <option key={session.id} value={session.id}>
                      {formatSessionTitle(session)}
                    </option>
                  ))}
                </Select>
                <Button type="button" variant="outline" size="icon" title="新建会话" disabled={!isAuthed || !settings.selectedSpaceId || busy === 'session'} onClick={handleNewChatSession}>
                  {busy === 'session' ? <Loader2 className="h-4 w-4 animate-spin" /> : <Plus className="h-4 w-4" />}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="icon"
                  title="刷新消息"
                  disabled={!isAuthed || !settings.selectedChatSessionId || busy === 'history'}
                  onClick={handleRefreshChatHistory}
                >
                  {busy === 'history' ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
                </Button>
              </div>

              <div className="h-[285px] space-y-2 overflow-y-auto rounded-lg border bg-card p-3">
                {chatMessages.length === 0 ? (
                  <div className="flex h-full items-center justify-center text-sm text-muted-foreground">{selectedSession ? '暂无消息' : '暂无会话'}</div>
                ) : (
                  chatMessages.map(item => {
                    const isUser = item.meta.role === ROLE_USER;
                    const isTool = item.meta.role === ROLE_TOOL;
                    const text = messageText(item) || (item.meta.complete === MESSAGE_PROGRESS_GENERATING ? '生成中...' : '');

                    return (
                      <div key={item.meta.message_id} className={cn('flex', isUser ? 'justify-end' : 'justify-start')}>
                        <div
                          className={cn(
                            'max-w-[82%] whitespace-pre-wrap rounded-md px-3 py-2 text-sm leading-relaxed',
                            isUser ? 'bg-primary text-primary-foreground' : 'bg-muted text-foreground',
                            isTool && 'border border-border bg-background text-muted-foreground'
                          )}
                        >
                          {text}
                        </div>
                      </div>
                    );
                  })
                )}
              </div>

              <div className="grid grid-cols-[1fr_auto] gap-2">
                <Textarea
                  value={chatInput}
                  placeholder="输入消息"
                  className="min-h-[74px] resize-none"
                  disabled={!isAuthed || busy === 'chat'}
                  onChange={event => setChatInput(event.target.value)}
                  onKeyDown={event => {
                    if (event.key === 'Enter' && !event.shiftKey) {
                      event.preventDefault();
                      void handleSendChat();
                    }
                  }}
                />
                <Button
                  type="button"
                  size="icon"
                  title="发送"
                  disabled={!isAuthed || !settings.selectedSpaceId || !chatInput.trim() || busy === 'chat'}
                  onClick={handleSendChat}
                >
                  {busy === 'chat' ? <Loader2 className="h-4 w-4 animate-spin" /> : <SendHorizontal className="h-4 w-4" />}
                </Button>
              </div>
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
