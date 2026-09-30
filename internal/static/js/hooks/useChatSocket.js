/**
 * ChatView 的 WebSocket 生命週期 + 串流解析 + 訊息／設定狀態。
 * 從 ChatView.js 抽出（原本塞在元件裡的 330 行 init effect）。
 */
function useChatSocket({ session, agentType, showPermModeSelect, showEffortSelect }) {
  const [messages, setMessages]   = useState([]);
  const [state, setState]         = useState('IDLE');
  const [permTools, setPermTools] = useState([]);
  const [mode, setMode]           = useState(() => normalizePermMode(agentType, session.permission_mode || 'default'));
  const [modelSel, setModelSel]   = useState(() => session.model || '');
  const [effortSel, setEffortSel] = useState(() => session.effort || '');
  const [histLoaded, setHistLoaded] = useState(false);
  const [inputMode, setInputMode] = useState(() => readInputModeForSession(session.id));
  const [shellType, setShellType] = useState('bash');
  const [shellPendingCmd, setShellPendingCmd] = useState(null);
  const [shellRequest, setShellRequest] = useState(null);
  const [quota, setQuota] = useState(null);
  const [quotaRefreshing, setQuotaRefreshing] = useState(false);
  const [sessionModel, setSessionModel] = useState(null);
  const [activityHint, setActivityHint] = useState('');
  /** 執行中先送出、等待前一輪完成的訊息（後端持久化，sync / queue_update 同步） */
  const [queue, setQueue] = useState([]);
  const [queuePaused, setQueuePaused] = useState(false);

  const shellBuf = useRef([]);
  const shellRenderPending = useRef(false);
  const wsRef = useRef(null);
  const streamBuf = useRef('');
  const thinkingMode = useRef(false);
  const connGenRef = useRef(0);
  const activeSessionRef = useRef(session.id);
  const reconnectTimerRef = useRef(null);
  /** 後端已套用的權限／輸入模式（送出時若與畫面選擇不同才送 set_mode／set_input_mode） */
  const serverPermAppliedRef = useRef(null);
  const serverInputAppliedRef = useRef(null);
  const serverModelAppliedRef = useRef(null);
  const serverEffortAppliedRef = useRef(null);

  // session 切換時重置訊息／連線相關狀態，避免殘留舊會話上下文
  useEffect(() => {
    activeSessionRef.current = session.id;
    streamBuf.current = '';
    thinkingMode.current = false;
    shellBuf.current = [];
    shellRenderPending.current = false;
    setMessages([]);
    setHistLoaded(false);
    setState('IDLE');
    setPermTools([]);
    setShellRequest(null);
    setQueue([]);
    setQueuePaused(false);
    const im =
      session.input_mode === 'shell' || session.input_mode === 'agent'
        ? session.input_mode
        : readInputModeForSession(session.id);
    const pm = normalizePermMode(agentType, session.permission_mode || 'default');
    serverInputAppliedRef.current = im;
    serverPermAppliedRef.current = pm;
    serverModelAppliedRef.current = session.model || '';
    serverEffortAppliedRef.current = session.effort || '';
    setInputMode(im);
    setMode(pm);
    setModelSel(session.model || '');
    setEffortSel(session.effort || '');
    setShellPendingCmd(null);
  }, [session.id, session.permission_mode, session.model, session.effort, agentType, session.input_mode]);

  // 載入歷史 + 建立 WS
  useEffect(() => {
    connGenRef.current += 1;
    const effectGen = connGenRef.current;
    let cancelled = false;

    const isCurrent = () =>
      !cancelled && effectGen === connGenRef.current && activeSessionRef.current === session.id;

    const clearReconnectTimer = () => {
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }
    };

    const init = async () => {
      // 載入歷史紀錄
      let hist = [];
      try {
        const res = await apiFetch(`/sessions/${session.id}/messages`);
        if (res.ok) {
          const data = await res.json();
          if (Array.isArray(data)) hist = data;
          else console.warn('[messages] unexpected payload', data);
        } else {
          console.warn('[messages] load failed', res.status);
        }
      } catch (err) {
        console.warn('[messages] load error', err);
      }
      if (isCurrent()) {
        setMessages(hist.map(mapMessageRow));
        const lastClaude = [...hist].reverse().find((m) => m.role === 'claude' && m.status === 'pending');
        streamBuf.current = lastClaude ? (lastClaude.content || '') : '';
        setHistLoaded(true);
      }

      // 建立 WebSocket
      connectWS();
    };

    const connectWS = () => {
      if (!isCurrent()) return;
      clearReconnectTimer();
      const ws = new WebSocket(wsURL(session.id));
      ws._sessionId = session.id;
      ws._gen = effectGen;
      wsRef.current = ws;
      let everConnected = false;

      // 使用者正在看這個 session：標記已讀，讓列表未讀點消失。失敗不影響聊天功能，靜默忽略。
      const markRead = () => {
        apiFetch(`/sessions/${session.id}`, {
          method: 'PATCH',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ mark_read: true }),
        }).catch(() => {});
      };

      ws.onopen = () => {
        everConnected = true;
        markRead();
      };

      ws.onmessage = (evt) => {
        if (!isCurrent() || wsRef.current !== ws) return;
        const msg = JSON.parse(evt.data);

        if (msg.type === 'sync') {
          setState(msg.value);
          if (msg.input_mode) {
            setInputMode(msg.input_mode);
            serverInputAppliedRef.current = msg.input_mode;
          }
          if (msg.shell_type) setShellType(msg.shell_type);
          if (msg.shell_pending) setShellPendingCmd(msg.shell_pending);
          if (msg.messages) {
            const rows = Array.isArray(msg.messages) ? msg.messages : JSON.parse(msg.messages);
            const mapped = rows.map(mapMessageRow);
            const lastClaude = [...rows].reverse().find((m) => m.role === 'claude' && (m.status || 'done') === 'pending');
            streamBuf.current = lastClaude ? (lastClaude.content || '') : '';
            setMessages((prev) => {
              const byId = new Map();
              for (const m of prev) {
                if (m.id != null && m.createdAt) byId.set(m.id, m.createdAt);
              }
              return mapped.map((m) => (
                m.createdAt || m.id == null || !byId.has(m.id)
                  ? m
                  : { ...m, createdAt: byId.get(m.id) }
              ));
            });
          }
          if (msg.quota) setQuota(msg.quota);
          if (msg.model) setSessionModel(msg.model);
          setQueue(msg.queue || []);
          setQueuePaused(!!msg.queue_paused);
          return;
        }

        if (msg.type === 'queue_update') {
          setQueue(msg.queue || []);
          setQueuePaused(!!msg.queue_paused);
          return;
        }

        if (msg.type === 'quota_update' && msg.quota) {
          setQuota(msg.quota);
          setQuotaRefreshing(false);
          return;
        }

        if (msg.type === 'model_update' && msg.model) {
          setSessionModel(msg.model);
          return;
        }

        if (msg.type === 'status') {
          setState(msg.value);
          if (msg.value !== 'AWAITING_SHELL_CONFIRM') {
            setShellRequest(null);
          }
          if (msg.value === 'THINKING') {
            streamBuf.current = '';
            thinkingMode.current = false;
            setActivityHint('');
            setMessages((prev) => {
              const last = prev[prev.length - 1];
              if (last && last.role === 'claude' && last.streaming && last.status === 'pending') {
                return prev;
              }
              return [...prev, { role: 'claude', content: '', html: null, streaming: true, status: 'pending', createdAt: new Date().toISOString() }];
            });
          }
          if (msg.value === 'SHELL_RUNNING' || msg.value === 'SHELL_EXEC') {
            shellBuf.current = [];
            setShellPendingCmd(null);
            setMessages((prev) => {
              const last = prev[prev.length - 1];
              if (last && last.role === 'shell' && last.streaming) return prev;
              return [...prev, { role: 'shell', content: '', html: null, streaming: true, status: 'pending', createdAt: new Date().toISOString() }];
            });
          }
          if (msg.value === 'SHELL_IDLE') {
            setShellPendingCmd(null);
            shellBuf.current = [];
            setMessages((prev) => prev.map((m, i) =>
              i === prev.length - 1 && m.role === 'shell' && m.streaming
                ? { ...m, streaming: false, status: 'done' }
                : m
            ));
          }
          if (msg.value === 'IDLE' || msg.value === 'AWAITING_CONFIRM') {
            // 使用者全程留在這個 session：一輪對話結束才會推進 last_active，須補標已讀，
            // 否則只在連線建立當下標過一次，中途 agent 的新回覆會讓列表誤判成未讀。
            markRead();
            setMessages((prev) => {
              const updated = prev.map((m, i) =>
                i === prev.length - 1 ? { ...m, streaming: false, status: m.status === 'pending' ? 'done' : m.status } : m
              );
              const last = updated[updated.length - 1];
              // 移除因中斷而從未收到任何內容的暫存 claude 泡泡（無 DB id、無內容）
              if (last && last.role === 'claude' && last.id == null && !last.content && !last.html) {
                return updated.slice(0, -1);
              }
              return updated;
            });
          }
        }

        if (msg.type === 'reset') {
          setMessages([]);
          streamBuf.current = '';
        }

        if (msg.type === 'user_message') {
          setMessages(prev => [...prev, { role: 'user', content: msg.content, createdAt: msg.created_at || new Date().toISOString() }]);
        }

        if (msg.type === 'activity') {
          if (msg.content) setActivityHint(msg.content);
          return;
        }

        if (msg.type === 'thinking') {
          thinkingMode.current = true;
          const text = msg.content || '';
          setMessages((prev) => prev.map((m, i) =>
            i === prev.length - 1 ? { ...m, content: text, html: null, thinking: true, streaming: true, status: 'pending' } : m
          ));
        }

        if (msg.type === 'delta') {
          if (thinkingMode.current) {
            thinkingMode.current = false;
            streamBuf.current = '';
          }
          streamBuf.current += msg.content;
          const html = parseMarkdown(streamBuf.current);
          setMessages((prev) => prev.map((m, i) =>
            i === prev.length - 1 ? { ...m, content: streamBuf.current, html, thinking: false, streaming: true, status: 'pending' } : m
          ));
        }

        if (msg.type === 'permission_request') {
          setPermTools(msg.tools || []);
        }

        if (msg.type === 'message_result_text') {
          const rid = msg.id;
          const rt = msg.content != null ? String(msg.content) : '';
          setMessages((prev) => {
            let idx = prev.findIndex((m) => m.id != null && rid != null && m.id === rid);
            if (idx < 0) {
              for (let i = prev.length - 1; i >= 0; i--) {
                if (prev[i].role === 'claude') {
                  idx = i;
                  break;
                }
              }
            }
            if (idx < 0) return prev;
            const copy = [...prev];
            const prevContent = (copy[idx].content || '').trim();
            copy[idx] = {
              ...copy[idx],
              resultText: rt,
              id: copy[idx].id != null ? copy[idx].id : rid,
              ...(prevContent ? {} : {
                content: rt,
                html: rt ? parseMarkdown(rt) : null,
                thinking: false,
                streaming: false,
              }),
            };
            return copy;
          });
        }

        if (msg.type === 'input_mode_changed') {
          setInputMode(msg.value);
          serverInputAppliedRef.current = msg.value;
          if (msg.shell_type) setShellType(msg.shell_type);
        }

        if (msg.type === 'shell_approval_request') {
          setShellPendingCmd({ command: msg.content, work_dir: msg.work_dir, shell_type: msg.shell_type });
        }

        if (msg.type === 'shell_approval_cancelled') {
          setShellPendingCmd(null);
        }

        if (msg.type === 'shell_command_request') {
          setShellRequest({
            command: msg.command || '',
            line: msg.line || '',
            workDirKey: msg.work_dir_key || '',
          });
        }

        if (msg.type === 'shell_delta') {
          if (!msg.content) return;
          shellBuf.current.push({ stream: msg.stream || 'stdout', text: msg.content });
          if (!shellRenderPending.current) {
            shellRenderPending.current = true;
            requestAnimationFrame(() => {
              shellRenderPending.current = false;
              const rendered = shellBuf.current.map(s => s.stream === 'stderr' ? '[stderr] ' + s.text : s.text).join('');
              setMessages(prev => prev.map((m, i) =>
                i === prev.length - 1 && m.role === 'shell' && m.streaming
                  ? { ...m, content: rendered }
                  : m
              ));
            });
          }
        }

        if (msg.type === 'shell_done') {
          const code = msg.exit_code;
          setMessages(prev => prev.map((m, i) =>
            i === prev.length - 1 && m.role === 'shell' && m.streaming
              ? { ...m, streaming: false, status: 'done', exitCode: code, id: msg.id != null ? msg.id : m.id }
              : m
          ));
          shellBuf.current = [];
        }

        if (msg.type === 'shell_error') {
          shellBuf.current.push({ stream: 'stderr', text: '[error] ' + (msg.content || '') + '\n' });
          const rendered = shellBuf.current.map(s => s.stream === 'stderr' ? '[stderr] ' + s.text : s.text).join('');
          setMessages(prev => prev.map((m, i) =>
            i === prev.length - 1 && m.role === 'shell' && m.streaming
              ? { ...m, content: rendered }
              : m
          ));
        }

        if (msg.type === 'shell_result') {
          setMessages((prev) => {
            const id = msg.id;
            const content = msg.content != null ? String(msg.content) : '';
            let found = false;
            const next = prev.map((m) => {
              if (found) return m;
              if (id != null && m.id === id) {
                found = true;
                return { ...m, role: 'shell', content, html: null, streaming: false, status: 'done' };
              }
              return m;
            });
            if (found) return next;
            for (let i = next.length - 1; i >= 0; i--) {
              if (next[i].role === 'shell' && next[i].status === 'pending') {
                const copy = [...next];
                copy[i] = { ...copy[i], id: id != null ? id : copy[i].id, content, html: null, streaming: false, status: 'done' };
                return copy;
              }
            }
            return [...next, { role: 'shell', id: id != null ? id : null, content, html: null, streaming: false, status: 'done' }];
          });
        }
      };

      ws.onerror = (evt) => {
        console.error('[ws] 連線錯誤', evt);
      };

      ws.onclose = (evt) => {
        console.warn('[ws] 連線關閉', evt.code, evt.reason);
        if (!isCurrent() || wsRef.current !== ws) return;
        const scheduleReconnect = () => {
          reconnectTimerRef.current = setTimeout(() => {
            reconnectTimerRef.current = null;
            connectWS();
          }, 2000);
        };
        if (!everConnected && !isTelegram) {
          apiFetch('/sessions').then((res) => {
            if (!isCurrent() || wsRef.current !== ws) return;
            if (!res.ok) return;
            try {
              if (!localStorage.getItem(WEB_SESSION_STORAGE_KEY)) return;
            } catch (_) { return; }
            scheduleReconnect();
          }).catch(() => {});
          return;
        }
        if (!isTelegram) {
          try {
            if (!localStorage.getItem(WEB_SESSION_STORAGE_KEY)) return;
          } catch (_) { return; }
        }
        scheduleReconnect();
      };
    };

    init();

    return () => {
      cancelled = true;
      clearReconnectTimer();
      if (wsRef.current && wsRef.current._gen === effectGen) {
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, [session.id]);

  const send = useCallback((obj) => {
    const ws = wsRef.current;
    if (!ws || ws.readyState !== 1) return false;
    if (ws._sessionId !== activeSessionRef.current || ws._sessionId !== session.id) return false;
    ws.send(JSON.stringify(obj));
    return true;
  }, [session.id]);

  const flushPendingModes = useCallback(() => {
    const wantPerm = normalizePermMode(agentType, mode);
    const wantInput = inputMode === 'shell' || inputMode === 'agent' ? inputMode : 'agent';
    if (showPermModeSelect && wantPerm !== serverPermAppliedRef.current) {
      if (!send({ type: 'set_mode', mode: wantPerm })) return false;
      serverPermAppliedRef.current = wantPerm;
    }
    if (wantInput !== serverInputAppliedRef.current) {
      if (!send({ type: 'set_input_mode', mode: wantInput })) return false;
      serverInputAppliedRef.current = wantInput;
    }
    if (modelSel !== serverModelAppliedRef.current) {
      if (!send({ type: 'set_model', model: modelSel })) return false;
      serverModelAppliedRef.current = modelSel;
    }
    if (showEffortSelect && effortSel !== serverEffortAppliedRef.current) {
      if (!send({ type: 'set_effort', effort: effortSel })) return false;
      serverEffortAppliedRef.current = effortSel;
    }
    return true;
  }, [send, agentType, mode, inputMode, showPermModeSelect, modelSel, effortSel, showEffortSelect]);

  const handleQuotaRefresh = useCallback(() => {
    if (quotaRefreshing) return;
    if (agentType === 'antigravity') return;
    setQuotaRefreshing(true);
    if (!send({ type: 'refresh_quota' })) {
      setQuotaRefreshing(false);
      return;
    }
    setTimeout(() => setQuotaRefreshing(false), 10000);
  }, [quotaRefreshing, agentType, send]);

  /** 授權面板「允許並記住」：立即寫入後端並更新已套用 ref，不等下次送出才 flush */
  const commitPermMode = useCallback((newMode) => {
    const n = normalizePermMode(agentType, newMode);
    setMode(n);
    if (!send({ type: 'set_mode', mode: n })) return false;
    serverPermAppliedRef.current = n;
    return true;
  }, [agentType, send]);

  return {
    messages,
    state,
    permTools, setPermTools,
    mode, setMode,
    modelSel, setModelSel,
    effortSel, setEffortSel,
    histLoaded,
    inputMode, setInputMode,
    shellType,
    shellPendingCmd, setShellPendingCmd,
    shellRequest,
    quota, quotaRefreshing,
    sessionModel,
    activityHint,
    queue, queuePaused,
    send,
    flushPendingModes,
    handleQuotaRefresh,
    commitPermMode,
  };
}
