const EMPTY_SUGGESTIONS = ['看一下目前 git 狀態與最近的變更', '概覽這個專案的結構', '跑測試並整理失敗原因'];

function ChatView({ session, onBack, showBack = true, fullHeight = true, usePermModeDropdown = false, onJumpToSession, allSessions }) {
  const jumpToSession = typeof onJumpToSession === 'function' ? onJumpToSession : () => {};
  const agentType = session.agent_type || 'claude';
  // Claude / Cursor / Antigravity 皆支援 mode 切換（Codex 暫無對應概念）
  const showPermModeSelect = agentType !== 'codex' && agentType !== 'kiro';
  const showEffortSelect = agentType !== 'cursor';
  const {
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
  } = useChatSocket({ session, agentType, showPermModeSelect, showEffortSelect });

  const [input, setInput]         = useState('');
  const [lightboxSrc, setLightboxSrc] = useState(null);
  const [forwardModal, setForwardModal] = useState(null);
  const [forwardHints, setForwardHints] = useState({});
  // 長按選單的狀態放在 MessageActionsHost，這裡只拿開啟函式，避免長按時整串訊息重繪。
  const openMsgSheetRef = useRef(null);
  const handleLongPress = useCallback((payload) => {
    if (openMsgSheetRef.current) openMsgSheetRef.current(payload);
  }, []);
  const bindLongPress = useLongPress(handleLongPress);
  const [slashMenuItems, setSlashMenuItems] = useState([]);
  const [slashActiveIdx, setSlashActiveIdx] = useState(0);
  const [mentionOpen, setMentionOpen] = useState(false);
  const [mentionItems, setMentionItems] = useState([]);
  const [mentionActiveIdx, setMentionActiveIdx] = useState(0);
  const [mentionChips, setMentionChips] = useState([]);
  // 已上傳、尚未送出的附件：送出時才組成 `[附件] 路徑` 前綴，輸入框保持乾淨
  const [attachments, setAttachments] = useState([]);
  const [dragOver, setDragOver] = useState(false);
  const dragDepthRef = useRef(0);
  const bottomRef = useRef(null);
  const chatScrollRef = useRef(null);
  const chatNearBottomRef = useRef(true);
  const [showJumpLatest, setShowJumpLatest] = useState(false);
  const chatInputRef = useRef(null);
  const slashInputWrapRef = useRef(null);
  const composerWrapRef = useRef(null);
  const { collapsed: headerCollapsed, toggle: toggleHeader, setCollapsed: setHeaderCollapsed } = useChatHeaderCollapsed();

  const syncChatNearBottom = useCallback(() => {
    const el = chatScrollRef.current;
    if (!el) return;
    const gap = el.scrollHeight - el.scrollTop - el.clientHeight;
    chatNearBottomRef.current = gap < 80;
    setShowJumpLatest(gap >= 200);
  }, []);

  const jumpToLatest = () => {
    chatNearBottomRef.current = true;
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  /** 進入會話或執行結束後將游標放回輸入框（雙 rAF 以配合 React commit／行動裝置鍵盤） */
  const focusChatInput = useCallback(() => {
    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        const el = chatInputRef.current;
        if (!el || el.disabled) return;
        try {
          el.focus({ preventScroll: true });
        } catch (_) {
          el.focus();
        }
      });
    });
  }, []);

  // 捲到底
  useEffect(() => {
    // 使用者往上翻舊內容時不強制拉回；自己剛送出的訊息例外
    const last = messages[messages.length - 1];
    if (chatNearBottomRef.current || last?.role === 'user') {
      bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
    }
    let id2;
    const id1 = requestAnimationFrame(() => {
      id2 = requestAnimationFrame(() => {
        syncChatNearBottom();
      });
    });
    return () => {
      cancelAnimationFrame(id1);
      if (id2 != null) cancelAnimationFrame(id2);
    };
  }, [messages, syncChatNearBottom]);

  // 待授權／Shell 確認區塊出現時捲到底（此狀態常不經 messages 更新而單獨出現）
  useEffect(() => {
    const showPermPanel =
      (state === 'AWAITING_CONFIRM' && permTools.length > 0) ||
      (state === 'SHELL_AWAITING_APPROVAL' && shellPendingCmd) ||
      (state === 'AWAITING_SHELL_CONFIRM' && shellRequest);
    if (!showPermPanel) return;
    let id2;
    const id1 = requestAnimationFrame(() => {
      id2 = requestAnimationFrame(() => {
        bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
      });
    });
    return () => {
      cancelAnimationFrame(id1);
      if (id2 != null) cancelAnimationFrame(id2);
    };
  }, [state, permTools.length, shellPendingCmd, shellRequest]);

  // 離開頁面確認（任務進行中或等待授權時）
  useEffect(() => {
    const handler = (e) => {
      if (state === 'THINKING' || state === 'STREAMING' || state === 'AWAITING_CONFIRM' || state === 'SHELL_RUNNING' || state === 'SHELL_AWAITING_APPROVAL' || state === 'AWAITING_SHELL_CONFIRM' || state === 'SHELL_EXEC') {
        e.preventDefault();
        e.returnValue = '';
      }
    };
    window.addEventListener('beforeunload', handler);
    return () => window.removeEventListener('beforeunload', handler);
  }, [state]);

  // session 切換時重置輸入框草稿與 composer 選單（訊息／連線狀態由 useChatSocket 自行重置）
  useEffect(() => {
    let draft = '';
    try {
      draft = localStorage.getItem(draftInputStorageKey(session.id)) || '';
    } catch (_) {}
    setInput(draft);
    chatNearBottomRef.current = true;
    setShowJumpLatest(false);
    setSlashMenuItems([]);
    setSlashActiveIdx(0);
    setMentionOpen(false);
    setMentionItems([]);
    setMentionActiveIdx(0);
    setMentionChips([]);
    setAttachments((prev) => {
      prev.forEach((a) => a.previewUrl && URL.revokeObjectURL(a.previewUrl));
      return [];
    });
  }, [session.id]);

  const prevChatStateRef = useRef(null);
  // 歷史載入完成＝畫面就緒，將 focus 預設在輸入框
  useEffect(() => {
    if (!histLoaded) return;
    focusChatInput();
  }, [histLoaded, session.id, focusChatInput]);

  // 切換會話、歷史載入完成：直接跳到最底。平滑捲動在長歷史上會被後續排版（圖片、程式碼上色）撐高而停在半路，
  // 所以用瞬間跳轉，並在排版穩定後（下一幀、150ms）再補跳一次。
  useEffect(() => {
    if (!histLoaded) return;
    const jump = () => {
      const el = chatScrollRef.current;
      if (el) el.scrollTop = el.scrollHeight;
      chatNearBottomRef.current = true;
      setShowJumpLatest(false);
    };
    jump();
    const raf = requestAnimationFrame(jump);
    const timer = setTimeout(jump, 150);
    return () => {
      cancelAnimationFrame(raf);
      clearTimeout(timer);
    };
  }, [histLoaded, session.id]);

  // 狀態切回可輸入時（例如 THINKING／STREAMING 結束）自動 focus。
  // 只在閒置狀態 focus：執行中輸入框也可用（排隊），若每次狀態變化都 focus，手機鍵盤會反覆彈出。
  useEffect(() => {
    const prev = prevChatStateRef.current;
    prevChatStateRef.current = state;
    if (prev === null) return;
    if (prev === state) return;
    if (state !== 'IDLE' && state !== 'SHELL_IDLE') return;
    focusChatInput();
  }, [state, focusChatInput]);

  // code block 複製鍵、圖片點擊放大：dangerouslySetInnerHTML 插入的 DOM 沒有 React 事件，
  // 用外層 onClick 事件委派抓 .code-copy-btn / .chat-img（冒泡到這裡才處理，不影響一般點擊/選字）。
  const handleProseClick = (e) => {
    const img = e.target.closest ? e.target.closest('.chat-img') : null;
    if (img) {
      e.stopPropagation();
      setLightboxSrc(img.getAttribute('src') || '');
      return;
    }
    const btn = e.target.closest ? e.target.closest('.code-copy-btn') : null;
    if (!btn) return;
    e.stopPropagation();
    const b64 = btn.getAttribute('data-copy-b64') || '';
    let text = '';
    try {
      text = decodeURIComponent(escape(atob(b64)));
    } catch (_) {
      return;
    }
    if (!copyTextExecCommand(text)) return;
    btn.classList.add('copied');
    const iconCopy = btn.querySelector('.icon-copy');
    const iconCheck = btn.querySelector('.icon-check');
    if (iconCopy) iconCopy.style.display = 'none';
    if (iconCheck) iconCheck.style.display = '';
    clearTimeout(btn._copyResetTimer);
    btn._copyResetTimer = setTimeout(() => {
      btn.classList.remove('copied');
      if (iconCopy) iconCopy.style.display = '';
      if (iconCheck) iconCheck.style.display = 'none';
    }, 2000);
  };

  const closeComposerMenus = () => {
    setSlashMenuItems([]);
    setMentionOpen(false);
    setMentionItems([]);
    setMentionActiveIdx(0);
  };

  const syncComposerMenus = (value, cursor, mode) => {
    if (mode === 'shell') {
      closeComposerMenus();
      return;
    }
    if (String(value || '').startsWith('/')) {
      const q = String(value).toLowerCase();
      const filtered = SLASH_COMMANDS.filter(
        (c) => (!c.modes || c.modes.includes(mode)) && c.command.toLowerCase().startsWith(q)
      );
      setSlashMenuItems(filtered);
      setSlashActiveIdx(0);
      setMentionOpen(false);
      setMentionItems([]);
      return;
    }
    setSlashMenuItems([]);
    const hit = mentionQueryAtCursor(value, cursor);
    if (!hit) {
      setMentionOpen(false);
      setMentionItems([]);
      return;
    }
    const excludeIds = mentionChips.map((s) => s.id);
    const filtered = filterMentionSessions(allSessions, session.id, hit.query, excludeIds);
    setMentionOpen(true);
    setMentionItems(filtered);
    setMentionActiveIdx(0);
  };

  const handleMentionSelect = (s) => {
    const el = chatInputRef.current;
    const cursor = el ? el.selectionStart : input.length;
    const next = consumeMentionQuery(input, cursor);
    setInput(next.text);
    try {
      localStorage.setItem(draftInputStorageKey(session.id), next.text);
    } catch (_) {}
    setMentionChips((prev) => (prev.some((x) => x.id === s.id) ? prev : [...prev, s]));
    setMentionOpen(false);
    setMentionItems([]);
    requestAnimationFrame(() => {
      const ta = chatInputRef.current;
      if (!ta) return;
      ta.focus();
      ta.setSelectionRange(next.cursor, next.cursor);
    });
  };

  const handleMentionChipRemove = (id) => {
    setMentionChips((prev) => prev.filter((s) => s.id !== id));
  };

  // ── 訊息長按選單的動作（開關與複製由 MessageActionsHost 處理） ──
  const handleSheetForward = (p) => {
    setForwardModal({ messageKey: p.messageKey, messageContent: p.text });
  };
  const handleSend = (overrideText) => {
    const raw = overrideText !== undefined && overrideText !== null ? String(overrideText) : input;
    const trimmed = raw.trim();
    if (!trimmed && attachments.length === 0) return;
    if (trimmed === '/reset' || trimmed === '/clear') {
      if (state !== 'IDLE' && state !== 'SHELL_IDLE') return;
      if (!flushPendingModes()) return;
      send({ type: 'reset_context' });
      clearDraftInputForSession(session.id);
      setInput('');
      closeComposerMenus();
      return;
    }
    if (inputMode === 'shell') {
      if (state !== 'SHELL_IDLE' && state !== 'IDLE') return;
      if (!flushPendingModes()) return;
      send({ type: 'shell_exec', data: trimmed });
      clearDraftInputForSession(session.id);
      setInput('');
      closeComposerMenus();
      return;
    }
    const idle = state === 'IDLE' || state === 'SHELL_IDLE';
    if (!idle && !canQueue) return;
    // 排隊時不 flush 模式：set_mode 等會廣播 idle 狀態，執行中送會讓 UI 誤判完成。
    if (idle && !flushPendingModes()) return;
    const attachPrefix = attachments.map((a) => `[附件] ${a.path}
`).join('');
    const expanded = attachPrefix + expandMentionPrompt(trimmed, session, mentionChips);
    if (!send({ type: 'input', data: expanded })) return;
    clearDraftInputForSession(session.id);
    setInput('');
    clearAttachments();
    setMentionChips([]);
    closeComposerMenus();
  };

  const applySuggestion = (text) => {
    setInput(text);
    try { localStorage.setItem(draftInputStorageKey(session.id), text); } catch (_) {}
    focusChatInput();
  };

  const clearAttachments = () => {
    setAttachments((prev) => {
      prev.forEach((a) => a.previewUrl && URL.revokeObjectURL(a.previewUrl));
      return [];
    });
  };
  const removeAttachment = (id) => {
    setAttachments((prev) => {
      const hit = prev.find((a) => a.id === id);
      if (hit && hit.previewUrl) URL.revokeObjectURL(hit.previewUrl);
      return prev.filter((a) => a.id !== id);
    });
  };

  const handleSlashSelect = (command) => {
    setInput(command);
    closeComposerMenus();
    handleSend(command);
  };

  const handleKeyDown = (e) => {
    // 中文輸入法選字時的 Enter 是確認候選字，不能當成送出（keyCode 229 為舊版 WebView 的 IME 標記）
    if (e.nativeEvent.isComposing || e.keyCode === 229) return;
    const slashMenuOpenNow = slashMenuItems.length > 0;
    if (slashMenuOpenNow) {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        setSlashActiveIdx((i) => Math.min(i + 1, slashMenuItems.length - 1));
        return;
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        setSlashActiveIdx((i) => Math.max(i - 1, 0));
        return;
      }
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        const item = slashMenuItems[slashActiveIdx];
        if (item) handleSlashSelect(item.command);
        return;
      }
      if (e.key === 'Escape') {
        e.preventDefault();
        setSlashMenuItems([]);
        return;
      }
    }
    if (mentionOpen) {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        setMentionActiveIdx((i) => Math.min(i + 1, Math.max(mentionItems.length - 1, 0)));
        return;
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        setMentionActiveIdx((i) => Math.max(i - 1, 0));
        return;
      }
      if (e.key === 'Enter' && !e.shiftKey) {
        if (mentionItems.length > 0) {
          e.preventDefault();
          const item = mentionItems[mentionActiveIdx];
          if (item) handleMentionSelect(item);
          return;
        }
      }
      if (e.key === 'Escape') {
        e.preventDefault();
        setMentionOpen(false);
        setMentionItems([]);
        return;
      }
    }
    // 觸控裝置 Enter 換行，用送出鈕送出；桌面 Enter 送出、Shift+Enter 換行
    if (e.key === 'Enter' && !e.shiftKey && !window.matchMedia('(hover: none) and (pointer: coarse)').matches) {
      e.preventDefault();
      handleSend();
    }
  };

  const handleAllowOnce = () => {
    send({ type: 'allow_once', tools: permTools.map(t => t.tool_name) });
    setPermTools([]);
  };

  const handleDenyOnce = () => {
    send({ type: 'deny_once' });
    setPermTools([]);
  };

  /** 僅更新畫面選擇，送出訊息時才會送 set_mode */
  const handlePermModeDraftChange = (newMode) => {
    setMode(normalizePermMode(agentType, newMode));
  };
  /** 僅更新畫面選擇，送出訊息時才會送 set_model / set_effort */
  const handleModelDraftChange = (newModel) => setModelSel(newModel);
  const handleEffortDraftChange = (newEffort) => setEffortSel(newEffort);
  /** 授權面板「允許並記住」：須立即寫入後端並重試 */
  const handlePermModeCommitNow = (newMode) => {
    if (!commitPermMode(newMode)) return;
    setPermTools([]);
  };

  const handleInterrupt = () => send({ type: 'interrupt' });

  const fileInputRef = useRef(null);
  const [uploading, setUploading] = useState(false);
  // 上傳：存到 runtime workspace，成功後成為輸入框上方的附件 chip；送出時才把絕對路徑組進訊息（各家 CLI 通用）。
  // 迴紋針、貼上、拖放共用。
  const uploadFiles = async (files) => {
    if (!files.length || uploading) return;
    setUploading(true);
    const added = [];
    try {
      for (const f of files) {
        const fd = new FormData();
        fd.append('file', f);
        const res = await apiFetch(`/sessions/${session.id}/uploads`, { method: 'POST', body: fd });
        const data = await res.json().catch(() => ({}));
        if (!res.ok) {
          showToast(`${f.name}：${data.error || '上傳失敗'}`, { error: true, duration: 3000 });
          continue;
        }
        added.push({
          id: `${Date.now()}-${added.length}`,
          name: f.name,
          path: data.path,
          previewUrl: f.type.startsWith('image/') ? URL.createObjectURL(f) : '',
        });
      }
    } catch (_) {
      showToast('上傳失敗', { error: true });
    } finally {
      setUploading(false);
    }
    if (!added.length) return;
    setAttachments((prev) => [...prev, ...added]);
    focusChatInput();
  };
  const handleFilesPicked = (e) => {
    const files = Array.from(e.target.files || []);
    e.target.value = '';
    uploadFiles(files);
  };
  // 貼上剪貼簿的截圖／檔案；純文字貼上不攔。截圖常沒有副檔名（後端依副檔名比白名單），依 MIME 補上。
  const handlePaste = (e) => {
    if (inputMode === 'shell') return;
    const files = Array.from(e.clipboardData?.files || []);
    if (!files.length) return;
    e.preventDefault();
    uploadFiles(files.map((f) => {
      if (/\.[a-z0-9]+$/i.test(f.name)) return f;
      const ext = (f.type.split('/')[1] || '').replace('jpeg', 'jpg').replace('plain', 'txt');
      return new File([f], `paste.${ext}`, { type: f.type });
    }));
  };

  // 桌面拖放檔案：用計數器處理子元素 enter/leave 交錯造成的閃爍
  const hasFiles = (e) => Array.from(e.dataTransfer?.types || []).includes('Files');
  const handleDragEnter = (e) => {
    if (inputMode === 'shell' || !hasFiles(e)) return;
    e.preventDefault();
    dragDepthRef.current += 1;
    setDragOver(true);
  };
  const handleDragOver = (e) => {
    if (inputMode === 'shell' || !hasFiles(e)) return;
    e.preventDefault();
  };
  const handleDragLeave = (e) => {
    if (inputMode === 'shell' || !hasFiles(e)) return;
    dragDepthRef.current = Math.max(0, dragDepthRef.current - 1);
    if (dragDepthRef.current === 0) setDragOver(false);
  };
  const handleDrop = (e) => {
    if (inputMode === 'shell' || !hasFiles(e)) return;
    e.preventDefault();
    dragDepthRef.current = 0;
    setDragOver(false);
    uploadFiles(Array.from(e.dataTransfer.files || []));
  };

  const handleInputModeChange = (newMode) => {
    const busy = ['THINKING', 'STREAMING', 'AWAITING_CONFIRM', 'SHELL_RUNNING', 'SHELL_AWAITING_APPROVAL', 'SHELL_EXEC', 'AWAITING_SHELL_CONFIRM'].includes(state);
    if (busy) return;
    setInputMode(newMode);
    try {
      localStorage.setItem(inputModeStorageKey(session.id), newMode);
    } catch (_) {}
    const el = chatInputRef.current;
    syncComposerMenus(input, el ? el.selectionStart : input.length, newMode);
  };

  const handleShellApprove = () => {
    send({ type: 'shell_approve' });
    setShellPendingCmd(null);
  };

  const handleShellCancel = () => {
    send({ type: 'shell_cancel' });
    setShellPendingCmd(null);
  };

  const agentRunning = state === 'THINKING' || state === 'STREAMING';
  const taskRunning = agentRunning || state === 'SHELL_RUNNING' || state === 'SHELL_EXEC';
  // Agent 執行中／等授權時仍可送出：後端排入佇列，前一輪成功後依序執行。
  const canQueue = inputMode !== 'shell' && (agentRunning || state === 'AWAITING_CONFIRM');
  const isDisabled = !canQueue && (state === 'THINKING' || state === 'STREAMING' || state === 'AWAITING_CONFIRM' || state === 'SHELL_RUNNING' || state === 'SHELL_AWAITING_APPROVAL' || state === 'SHELL_EXEC' || state === 'AWAITING_SHELL_CONFIRM');
  const modeSwitchDisabled = ['THINKING', 'STREAMING', 'AWAITING_CONFIRM', 'SHELL_RUNNING', 'SHELL_AWAITING_APPROVAL', 'SHELL_EXEC', 'AWAITING_SHELL_CONFIRM'].includes(state);
  const coarsePointer = window.matchMedia('(hover: none) and (pointer: coarse)').matches;
  const slashMenuOpen = slashMenuItems.length > 0;
  const composerMenuOpen = slashMenuOpen || mentionOpen;

  useEffect(() => {
    if (isDisabled) closeComposerMenus();
  }, [isDisabled]);

  useEffect(() => {
    if (!composerMenuOpen) return;
    const onDocMouseDown = (e) => {
      const root = mentionOpen ? composerWrapRef.current : slashInputWrapRef.current;
      if (root && !root.contains(e.target)) {
        closeComposerMenus();
      }
    };
    document.addEventListener('mousedown', onDocMouseDown);
    return () => document.removeEventListener('mousedown', onDocMouseDown);
  }, [composerMenuOpen, mentionOpen]);

  /** 聊天輸入框：依內容動態增高；未達上限不出現卷軸 */
  useLayoutEffect(() => {
    const el = chatInputRef.current;
    if (!el || state === 'THINKING' || state === 'STREAMING' || state === 'SHELL_RUNNING' || state === 'SHELL_EXEC' || state === 'AWAITING_SHELL_CONFIRM') return;
    el.style.height = 'auto';
    const maxStr = getComputedStyle(el).maxHeight;
    const maxPx = parseFloat(maxStr);
    const cap = Number.isFinite(maxPx) && maxPx > 0 ? maxPx : 12 * 16;
    const needed = el.scrollHeight;
    el.style.height = `${Math.min(needed, cap)}px`;
    el.style.overflowY = needed > cap + 1 ? 'auto' : 'hidden';
  }, [input, state]);

  return (
    <div
      className={`flex flex-col ${fullHeight ? 'h-app' : 'h-full'}`}
      onDragEnter={handleDragEnter}
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
    >
      <ChatSessionHeader
        session={session}
        showBack={showBack}
        onBack={onBack}
        agentType={agentType}
        state={state}
        activityHint={activityHint}
        sessionModel={sessionModel}
        quota={quota}
        quotaRefreshing={quotaRefreshing}
        onQuotaRefresh={handleQuotaRefresh}
        headerCollapsed={headerCollapsed}
        onToggleHeader={toggleHeader}
        onExpandHeader={() => setHeaderCollapsed(false)}
        showPermModeSelect={showPermModeSelect}
        inputMode={inputMode}
        usePermModeDropdown={usePermModeDropdown}
        mode={mode}
        modeSwitchDisabled={modeSwitchDisabled}
        onPermModeChange={handlePermModeDraftChange}
        showEffortSelect={showEffortSelect}
        modelSel={modelSel}
        effortSel={effortSel}
        onModelChange={handleModelDraftChange}
        onEffortChange={handleEffortDraftChange}
      />

      {/* 訊息列表 */}
      <div className="relative flex-1 min-h-0 flex flex-col">
      <div
        ref={chatScrollRef}
        onScroll={syncChatNearBottom}
        className="flex-1 min-h-0 overflow-y-auto app-scroll px-4 py-[18px] sm:px-8 sm:py-7 flex flex-col gap-5"
      >
        {histLoaded && messages.length === 0 && (
          <div className="mt-16 flex flex-col items-center gap-4 text-center">
            <div className="text-sm text-[oklch(0.65_0.01_264)]">輸入指令開始對話</div>
            {session.work_dir ? (
              <div className="max-w-full truncate ra-mono text-xs text-[oklch(0.5_0.01_264)]" title={session.work_dir}>
                {workDirGroupShortLabel(session.work_dir)}{session.git_branch ? ` · ${session.git_branch}` : ''}
              </div>
            ) : null}
            {inputMode !== 'shell' && (
              <div className="flex flex-wrap justify-center gap-2">
                {EMPTY_SUGGESTIONS.map((t) => (
                  <button key={t} type="button" onClick={() => applySuggestion(t)}
                    className="rounded-full border border-[oklch(0.3_0.02_264)] bg-[oklch(0.19_0.02_264)] px-3 py-1.5 text-xs text-[oklch(0.8_0.01_264)] hover:border-violet-500/50 hover:text-violet-300 transition-colors">
                    {t}
                  </button>
                ))}
              </div>
            )}
          </div>
        )}
        {messages.map((m, i) => {
          const msgKey = m.id != null ? String(m.id) : `idx-${i}`;
          const forwardBody =
            m.role === 'claude' && String(m.resultText || '').trim() !== ''
              ? m.resultText
              : (m.content || '').trim();
          const canForwardShellOrAgent =
            (m.role === 'claude' || m.role === 'shell') && !m.streaming && !!String(forwardBody || '').trim();
          const showStreamingTail =
            (state === 'THINKING' || state === 'STREAMING') &&
            m.role === 'claude' &&
            i === messages.length - 1;
          const timeLabel = formatMessageTime(m.createdAt);
          // 長按選單的文字；思考中（thinking）或尚無內容的訊息不啟用。
          const lpText = m.thinking ? '' : String(forwardBody || '').trim();
          return (
          <div
            key={m.id != null ? `m-${m.id}` : i}
            className={`flex w-full min-w-0 ${m.role === 'user' ? 'justify-end' : 'justify-start'} ${m.role === 'claude' || m.role === 'shell' ? 'group' : ''}`}
          >
            <div
              className={`flex flex-col min-w-0 ${lpText ? 'msg-lp' : ''} ${m.role === 'user' ? 'items-end max-w-[60%]' : m.role === 'shell' ? 'items-start max-w-[85%]' : 'items-start max-w-[78%]'}`}
              {...(lpText ? bindLongPress({
                text: lpText,
                role: m.role,
                canForward: canForwardShellOrAgent,
                messageKey: msgKey,
                createdAt: m.createdAt,
              }) : {})}
            >
            {m.role === 'user' ? (
              <div className="bubble-user text-white px-4 py-3 text-sm w-fit max-w-full min-w-0">
                <div className="whitespace-pre-wrap break-words leading-relaxed">{m.content}</div>
              </div>
            ) : m.role === 'shell' ? (
              <div className="bubble-shell px-4 py-3 text-sm w-fit max-w-full min-w-0">
                <div className="flex items-start gap-2 mb-1.5 min-w-0">
                  <span className="inline-flex items-center gap-1 font-mono text-amber-500/80 text-xs">
                    <span>&gt;_</span>
                    <span>{shellType || 'shell'}</span>
                  </span>
                </div>
                <ShellOutput content={m.content || ''} exitCode={m.exitCode} streaming={m.streaming} />
                {forwardHints[msgKey] ? (
                  <div className="mt-2 pt-2 border-t border-gray-700/80 text-[11px] text-gray-500">
                    已轉發到「{forwardHints[msgKey].label}」
                    <button type="button" className="ml-1 text-violet-400 hover:text-violet-300 font-medium" onClick={() => jumpToSession(forwardHints[msgKey].session)}>前往查看 →</button>
                  </div>
                ) : null}
              </div>
            ) : (
              /* Claude — 設計 1a 文件流：頭像列 + 無邊框正文 */
              <div className="bubble-claude w-fit max-w-full min-w-0 flex flex-col gap-3">
                <div className="flex items-center gap-2 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className={`inline-flex items-center justify-center w-[22px] h-[22px] rounded-md shrink-0 ${getAgentBadgeClass(agentType)}`} aria-hidden>
                      <AgentBadgeIcon agentType={agentType} />
                    </span>
                    <span className="text-[13px] font-bold text-[oklch(0.85_0.01_264)]">{AGENT_LABEL[agentType] || agentType || 'Claude'}</span>
                  </div>
                </div>
                {m.thinking ? (
                  <div className="flex flex-row items-start gap-1.5">
                    <span className="text-violet-400/60 text-xs mt-0.5 shrink-0 select-none">💭</span>
                    <span className="text-[oklch(0.55_0.01_264)] text-sm italic leading-relaxed line-clamp-3 min-w-0">{m.content}</span>
                  </div>
                ) : (m.html || showStreamingTail) ? (
                  <div className="flex flex-row items-start gap-1">
                    {m.html && (
                      <div
                        className="prose text-[oklch(0.85_0.01_264)] text-sm leading-[1.8] flex-1 min-w-0"
                        dangerouslySetInnerHTML={{ __html: m.html }}
                        onClick={handleProseClick}
                      />
                    )}
                    {showStreamingTail && (
                      <span className="streaming-tail shrink-0 select-none" aria-hidden="true">...</span>
                    )}
                  </div>
                ) : (
                  <div className="flex gap-1 py-1"><span className="dot"/><span className="dot"/><span className="dot"/></div>
                )}
                {forwardHints[msgKey] ? (
                  <div className="pt-1 text-[11px] text-[oklch(0.5_0.01_264)]">
                    已轉發到「{forwardHints[msgKey].label}」
                    <button type="button" className="ml-1 text-violet-400 hover:text-violet-300 font-medium" onClick={() => jumpToSession(forwardHints[msgKey].session)}>前往查看 →</button>
                  </div>
                ) : null}
              </div>
            )}
            {(timeLabel || m.role === 'claude' || m.role === 'shell') ? (
              <div className="mt-0.5 -mb-1 flex items-center gap-1 px-0.5">
                {timeLabel ? (
                  <span className="ra-msg-time text-[10px] leading-none text-[oklch(0.48_0.01_264)] tabular-nums select-none" title={String(m.createdAt || '')}>
                    {timeLabel}
                  </span>
                ) : null}
                {(m.role === 'claude' || m.role === 'shell') ? (
                  <div className="flex items-center gap-0.5">
                    <MessageCopyButton text={forwardBody} className="p-1 rounded-md text-gray-400 hover:text-gray-200" />
                    <button
                      type="button"
                      onClick={(e) => { e.stopPropagation(); setForwardModal({ messageKey: msgKey, messageContent: forwardBody }); }}
                      disabled={!canForwardShellOrAgent}
                      title="轉發到其他會話"
                      aria-label="轉發到其他會話"
                      className="inline-flex items-center justify-center min-w-[32px] min-h-[32px] rounded-md text-gray-400 hover:text-cyan-400 disabled:opacity-30"
                    >
                      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="w-3.5 h-3.5" aria-hidden="true"><path d="m15 14 5-5-5-5" /><path d="M4 20v-7a4 4 0 0 1 4-4h12" /></svg>
                    </button>
                  </div>
                ) : null}
              </div>
            ) : null}
            </div>
          </div>
          );
        })}

        {/* Shell 指令批准對話框 */}
        {state === 'SHELL_AWAITING_APPROVAL' && shellPendingCmd && (
          <div className="rounded-xl border border-amber-700/60 bg-amber-950/30 px-4 py-3 text-sm mr-8">
            <div className="text-amber-400 font-semibold mb-2">⚠️ 即將執行 Shell 指令</div>
            <div className="text-gray-400 text-xs mb-1">Shell：{shellPendingCmd.shell_type || shellType}</div>
            {shellPendingCmd.work_dir && (
              <div className="text-gray-400 text-xs mb-2">目錄：<span className="font-mono text-gray-300">{shellPendingCmd.work_dir}</span></div>
            )}
            <pre className="bg-gray-900/60 rounded px-3 py-2 text-xs font-mono text-gray-200 mb-3 whitespace-pre-wrap break-words">{shellPendingCmd.command}</pre>
            <div className="flex gap-2">
              <button onClick={handleShellApprove}
                className="px-3 py-1.5 bg-amber-700 hover:bg-amber-600 text-white rounded-lg text-xs font-medium">
                確認執行
              </button>
              <button onClick={handleShellCancel}
                className="px-3 py-1.5 bg-gray-700 hover:bg-gray-600 text-gray-300 rounded-lg text-xs">
                取消
              </button>
            </div>
          </div>
        )}

        {/* AI 授權確認 */}
        {state === 'AWAITING_CONFIRM' && permTools.length > 0 && (
          <div className="rounded-xl border border-yellow-700 bg-yellow-950/40 px-4 py-3 text-sm mr-8">
            <div className="text-yellow-400 font-semibold mb-2">需要授權</div>
            {permTools.map((t, i) => <PermToolDetail key={i} tool={t} />)}
            <div className="flex gap-2 mt-3">
              <button onClick={handleAllowOnce}
                className="px-3 py-1.5 bg-yellow-700 hover:bg-yellow-600 text-white rounded-lg text-xs">
                允許此操作
              </button>
              {/* 切到 acceptEdits 只對編輯類工具有效；Bash 等會再被拒一次，所以只在全是編輯工具時顯示 */}
              {permTools.every((t) => EDIT_TOOL_NAMES.has(t.tool_name)) && (
                <button onClick={() => handlePermModeCommitNow('acceptEdits')}
                  className="px-3 py-1.5 bg-orange-700 hover:bg-orange-600 text-white rounded-lg text-xs">
                  允許並自動允許編輯
                </button>
              )}
              <button onClick={handleDenyOnce}
                className="px-3 py-1.5 bg-red-900 hover:bg-red-800 text-white rounded-lg text-xs">
                拒絕
              </button>
            </div>
          </div>
        )}

        {state === 'AWAITING_SHELL_CONFIRM' && shellRequest && (
          <div className="rounded-xl border border-orange-700/90 bg-orange-950/35 px-4 py-3 text-sm mr-8">
            <div className="text-orange-300 font-semibold mb-2">允許執行 Shell 指令？</div>
            <div className="text-gray-200 text-xs font-mono break-words mb-1 whitespace-pre-wrap">{shellRequest.line}</div>
            <div className="text-gray-500 text-[10px] font-mono mb-2">
              指令名稱：<span className="text-orange-200/90">{shellRequest.command}</span>
              {shellRequest.workDirKey ? (
                <span className="block truncate mt-0.5" title={shellRequest.workDirKey}>目錄：{shellRequest.workDirKey}</span>
              ) : null}
            </div>
            <div className="flex flex-wrap gap-2 mt-2">
              <button type="button" onClick={() => { send({ type: 'shell_allow_once' }); }}
                className="px-3 py-1.5 bg-orange-800 hover:bg-orange-700 text-white rounded-lg text-xs">
                允許一次
              </button>
              <button type="button" onClick={() => { send({ type: 'shell_allow_remember_workdir' }); }}
                className="px-3 py-1.5 bg-amber-900 hover:bg-amber-800 text-amber-100 rounded-lg text-xs">
                允許並記住此目錄
              </button>
              <button type="button" onClick={() => { send({ type: 'shell_deny' }); }}
                className="px-3 py-1.5 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-xs">
                拒絕
              </button>
            </div>
          </div>
        )}

        <div ref={bottomRef} />
      </div>
      {showJumpLatest && (
        <button
          type="button"
          onClick={jumpToLatest}
          aria-label="跳到最新訊息"
          title="跳到最新訊息"
          className="absolute bottom-3 right-4 flex h-9 w-9 items-center justify-center rounded-full border border-[oklch(0.34_0.03_264)] bg-[oklch(0.22_0.02_264)] text-[oklch(0.85_0.01_264)] shadow-lg hover:bg-[oklch(0.27_0.03_264)] transition-colors"
        >
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="w-4 h-4" aria-hidden="true">
            <path d="M12 5v14" />
            <path d="m19 12-7 7-7-7" />
          </svg>
        </button>
      )}
      </div>

      {/* 輸入區：水平內距略小於訊息列表，讓輸入框可用寬度較大 */}
      <div
        className={`shrink-0 px-4 sm:px-7 py-4 border-t transition-colors duration-200 ${inputMode === 'shell' ? 'bg-[oklch(0.15_0.02_264)] border-amber-900/40' : 'bg-[oklch(0.15_0.02_264)] border-[oklch(0.26_0.02_264)]'}`}
        style={{ paddingBottom: 'calc(0.75rem + env(safe-area-inset-bottom))' }}
      >
        {taskRunning && !canQueue ? (
          <button onClick={handleInterrupt}
            className="w-full py-2 bg-red-900/60 hover:bg-red-800 text-red-300 rounded-lg text-sm">
            中斷
          </button>
        ) : (
          <div className="w-full relative" ref={composerWrapRef}>
            <QueuedMessages
              items={queue}
              paused={queuePaused}
              canResume={queue.length > 0 && (queuePaused || state === 'IDLE' || state === 'SHELL_IDLE')}
              onRemove={(id) => send({ type: 'queue_remove', id })}
              onResume={() => send({ type: 'queue_resume' })}
            />
            {inputMode !== 'shell' && (
              <MentionChips items={mentionChips} onRemove={handleMentionChipRemove} />
            )}
            {mentionOpen && !slashMenuOpen && (
              <MentionMenu
                items={mentionItems}
                activeIndex={mentionActiveIdx}
                onSelect={handleMentionSelect}
              />
            )}
            <AttachmentChips items={attachments} onRemove={removeAttachment} />
            <div className={(inputMode === 'shell' ? 'ra-cmd-bar shell' : 'ra-cmd-bar') + ' w-full' + (dragOver ? ' ring-2 ring-violet-500/70' : '')}>
              <ModeToggleBtn
                value={inputMode}
                onChange={handleInputModeChange}
                disabled={modeSwitchDisabled}
                agentLabel={AGENT_LABEL[agentType] || 'Claude'}
              />
              {inputMode !== 'shell' && (
                <>
                  <input
                    ref={fileInputRef}
                    type="file"
                    multiple
                    accept="image/*,.pdf,.txt,.md,.log,.json,.csv"
                    className="hidden"
                    onChange={handleFilesPicked}
                  />
                  <button type="button" onClick={() => fileInputRef.current?.click()}
                    disabled={uploading || isDisabled}
                    aria-label="附加圖片或檔案"
                    title="附加圖片或檔案（上限 8 MB）"
                    className="shrink-0 flex items-center justify-center w-8 h-8 rounded-[8px] text-[oklch(0.75_0.01_264)] hover:bg-[oklch(0.22_0.02_264)] hover:text-violet-300 disabled:opacity-40 transition-colors">
                    {uploading ? (
                      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" className="w-4 h-4 animate-spin" aria-hidden="true">
                        <path d="M21 12a9 9 0 1 1-6.22-8.56" />
                      </svg>
                    ) : (
                      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="w-4 h-4" aria-hidden="true">
                        <path d="m21.44 11.05-9.19 9.19a6 6 0 0 1-8.49-8.49l8.57-8.57A4 4 0 1 1 18 8.84l-8.59 8.57a2 2 0 0 1-2.83-2.83l8.49-8.48" />
                      </svg>
                    )}
                  </button>
                </>
              )}
              <div className="relative flex w-full min-w-0 order-first items-end" ref={slashInputWrapRef}>
                {slashMenuOpen && (
                  <SlashCommandMenu
                    items={slashMenuItems}
                    activeIndex={slashActiveIdx}
                    onSelect={handleSlashSelect}
                  />
                )}
                <textarea
                  ref={chatInputRef}
                  data-chat-input
                  value={input}
                  onChange={(e) => {
                    const value = e.target.value;
                    setInput(value);
                    try {
                      localStorage.setItem(draftInputStorageKey(session.id), value);
                    } catch (_) {}
                    syncComposerMenus(value, e.target.selectionStart, inputMode);
                  }}
                  onSelect={(e) => syncComposerMenus(e.target.value, e.target.selectionStart, inputMode)}
                  onKeyDown={handleKeyDown}
                  onPaste={handlePaste}
                  disabled={isDisabled}
                  placeholder={inputMode === 'shell' ? `輸入 ${shellType || 'Shell'} 指令…` : canQueue ? '執行中…送出會排入佇列' : '輸入指令… @ 標記 session'}
                  rows={1}
                  className={[
                    'flex-1 min-w-0 w-full resize-none overflow-hidden border-0 bg-transparent px-1 py-1.5 text-[13.5px] leading-relaxed placeholder-[oklch(0.5_0.01_264)] focus:outline-none disabled:opacity-40 min-h-[2rem] max-h-[min(40vh,12rem)] box-border',
                    inputMode === 'shell' ? 'text-amber-50 font-mono placeholder-amber-900/60' : 'text-[oklch(0.9_0.01_264)]',
                  ].join(' ')}
                />
              </div>
              {/* 把中斷／送出推到按鈕列右側 */}
              <div className="flex-1" aria-hidden="true" />
              {agentRunning && (
                <button type="button" onClick={handleInterrupt}
                  aria-label="中斷"
                  title="中斷"
                  className="shrink-0 flex items-center justify-center w-8 h-8 rounded-[8px] bg-red-900/70 hover:bg-red-800 text-red-200 text-sm">
                  <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" className="w-3.5 h-3.5" aria-hidden="true"><rect x="5" y="5" width="14" height="14" rx="2" /></svg>
                </button>
              )}
              <button type="button" onClick={() => handleSend()} disabled={isDisabled || (!input.trim() && attachments.length === 0)}
                aria-label={canQueue ? '排入佇列' : '送出'}
                title={(canQueue ? '排入佇列' : '送出') + (coarsePointer ? '' : '（Enter）')}
                className={`shrink-0 flex items-center justify-center w-8 h-8 rounded-[8px] ${inputMode === 'shell' ? 'bg-amber-700 hover:bg-amber-600' : 'bg-[oklch(0.62_0.19_275)] hover:brightness-110'} disabled:opacity-30 text-white transition-colors text-sm`}>
                <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" className="w-4 h-4" aria-hidden="true"><path d="M12 19V5" /><path d="m5 12 7-7 7 7" /></svg>
              </button>
            </div>
          </div>
        )}
      </div>

      <ForwardModal
        payload={forwardModal}
        onClose={() => setForwardModal(null)}
        currentSessionId={session.id}
        allSessions={allSessions}
        usePermModeDropdown={usePermModeDropdown}
        onForwarded={({ messageKey, targetSession, jump }) => {
          setForwardHints((prev) => ({
            ...prev,
            [messageKey]: {
              label: `${targetSession.agent_type || 'claude'} / ${workDirGroupShortLabel(targetSession.work_dir)}`,
              session: targetSession,
            },
          }));
          if (jump) jumpToSession(targetSession);
        }}
      />

      <MessageActionsHost
        openRef={openMsgSheetRef}
        onForward={handleSheetForward}
      />

      {lightboxSrc && (
        <ChatImageLightbox src={lightboxSrc} onClose={() => setLightboxSrc(null)} />
      )}
    </div>
  );
}

const EDIT_TOOL_NAMES = new Set(['Edit', 'Write', 'MultiEdit', 'NotebookEdit']);

/** 授權面板的單一工具：Bash 顯示完整指令，編輯類顯示檔名與內容，其餘退回格式化 JSON。長內容可捲動。 */
function PermToolDetail({ tool }) {
  const input = tool.tool_input && typeof tool.tool_input === 'object' ? tool.tool_input : null;
  const preCls = 'mt-1 max-h-40 overflow-auto app-scroll rounded bg-gray-900/60 px-2.5 py-2 text-xs font-mono text-gray-200 whitespace-pre-wrap break-words';
  let body = null;
  if (input && typeof input.command === 'string') {
    body = <pre className={preCls}>{input.command}</pre>;
  } else if (input && typeof input.file_path === 'string') {
    const text = input.new_string ?? input.content ?? '';
    body = (
      <>
        <div className="mt-1 text-xs font-mono text-gray-400 break-all">{input.file_path}</div>
        {text ? <pre className={preCls}>{String(text)}</pre> : null}
      </>
    );
  } else if (input) {
    body = <pre className={preCls}>{JSON.stringify(input, null, 2)}</pre>;
  }
  return (
    <div className="mb-2">
      <div className="text-gray-300 text-xs font-mono font-semibold">{tool.tool_name}</div>
      {body}
    </div>
  );
}

/** 已上傳、待送出的附件：圖片顯示縮圖，其他檔案顯示檔名；× 移除（只是不帶進這次訊息，不刪伺服器上的檔）。 */
function AttachmentChips({ items, onRemove }) {
  if (!Array.isArray(items) || items.length === 0) return null;
  return (
    <div className="mb-2 flex flex-wrap gap-2" aria-label="待送出的附件">
      {items.map((a) => (
        <div key={a.id} className="relative flex items-center gap-2 rounded-lg border border-[oklch(0.3_0.02_264)] bg-[oklch(0.19_0.02_264)] p-1 pr-6 max-w-[14rem]" title={a.name}>
          {a.previewUrl ? (
            <img src={a.previewUrl} alt="" className="h-10 w-10 shrink-0 rounded-md object-cover" />
          ) : (
            <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md bg-[oklch(0.24_0.02_264)] text-[10px] font-semibold uppercase text-[oklch(0.7_0.01_264)]">
              {(a.name.split('.').pop() || 'file').slice(0, 4)}
            </span>
          )}
          <span className="min-w-0 truncate text-xs text-[oklch(0.8_0.01_264)]">{a.name}</span>
          <button type="button" onClick={() => onRemove(a.id)} aria-label={`移除附件 ${a.name}`}
            className="absolute right-1 top-1 flex h-5 w-5 items-center justify-center rounded-full text-[oklch(0.6_0.01_264)] hover:bg-[oklch(0.28_0.02_264)] hover:text-red-300">
            ×
          </button>
        </div>
      ))}
    </div>
  );
}

/** 排隊中的訊息：每則可移除；暫停（前一輪失敗／中斷／拒絕授權）時需手動繼續。 */
function QueuedMessages({ items, paused, canResume, onRemove, onResume }) {
  if (!Array.isArray(items) || items.length === 0) return null;
  return (
    <div className="mb-2 rounded-lg border border-[oklch(0.3_0.02_264)] bg-[oklch(0.18_0.02_264)] px-2 py-1.5 text-xs" aria-label="排隊中的訊息">
      <div className="flex items-center justify-between mb-1 text-[oklch(0.65_0.01_264)]">
        <span>{paused ? `⏸ 佇列已暫停（${items.length}）` : `佇列（${items.length}）· 完成後依序執行`}</span>
        {canResume && (
          <button type="button" onClick={onResume}
            className="px-2 py-0.5 rounded bg-[oklch(0.62_0.19_275)] hover:brightness-110 text-white">
            繼續
          </button>
        )}
      </div>
      <ol className="space-y-0.5">
        {items.map((q, i) => (
          <li key={q.id} className="flex items-center gap-1.5 text-[oklch(0.85_0.01_264)]">
            <span className="shrink-0 font-mono text-[10px] text-[oklch(0.55_0.01_264)]">{i + 1}.</span>
            <span className="min-w-0 flex-1 truncate" title={q.content}>{q.content}</span>
            <button type="button" onClick={() => onRemove(q.id)}
              aria-label="移除這則排隊訊息"
              className="shrink-0 px-1 text-[oklch(0.6_0.01_264)] hover:text-red-300">
              ×
            </button>
          </li>
        ))}
      </ol>
    </div>
  );
}

/** 聊天截圖燈箱：鎖背景捲動，支援雙指縮放／單指拖曳／雙擊切換縮放。 */
function ChatImageLightbox({ src, onClose }) {
  const [{ scale, x, y }, setTransform] = useState({ scale: 1, x: 0, y: 0 });
  const transformRef = useRef({ scale: 1, x: 0, y: 0 });
  const pinchRef = useRef(null);
  const panRef = useRef(null);
  const lastTapRef = useRef(0);

  const applyTransform = useCallback((next) => {
    const scale = Math.min(4, Math.max(1, next.scale));
    const x = scale === 1 ? 0 : next.x;
    const y = scale === 1 ? 0 : next.y;
    const t = { scale, x, y };
    transformRef.current = t;
    setTransform(t);
  }, []);

  useEffect(() => {
    const prevOverflow = document.body.style.overflow;
    const prevTouchAction = document.body.style.touchAction;
    document.body.style.overflow = 'hidden';
    document.body.style.touchAction = 'none';
    const onKey = (e) => { if (e.key === 'Escape') onClose(); };
    // 非 passive，才能擋掉 iOS/Telegram WebView 把 touchmove 傳給底下聊天室。
    const blockScroll = (e) => { e.preventDefault(); };
    document.addEventListener('keydown', onKey);
    document.addEventListener('touchmove', blockScroll, { passive: false });
    return () => {
      document.body.style.overflow = prevOverflow;
      document.body.style.touchAction = prevTouchAction;
      document.removeEventListener('keydown', onKey);
      document.removeEventListener('touchmove', blockScroll);
    };
  }, [onClose]);

  const touchDistance = (a, b) => Math.hypot(a.clientX - b.clientX, a.clientY - b.clientY);

  const onTouchStart = (e) => {
    if (e.touches.length === 2) {
      panRef.current = null;
      pinchRef.current = {
        dist: touchDistance(e.touches[0], e.touches[1]),
        scale: transformRef.current.scale,
        x: transformRef.current.x,
        y: transformRef.current.y,
      };
      return;
    }
    if (e.touches.length === 1) {
      pinchRef.current = null;
      const now = Date.now();
      if (now - lastTapRef.current < 280) {
        lastTapRef.current = 0;
        const cur = transformRef.current;
        if (cur.scale > 1.05) {
          applyTransform({ scale: 1, x: 0, y: 0 });
        } else {
          applyTransform({ scale: 2.5, x: 0, y: 0 });
        }
        return;
      }
      lastTapRef.current = now;
      if (transformRef.current.scale > 1) {
        panRef.current = {
          x: e.touches[0].clientX,
          y: e.touches[0].clientY,
          ox: transformRef.current.x,
          oy: transformRef.current.y,
        };
      }
    }
  };

  const onTouchMove = (e) => {
    e.preventDefault();
    e.stopPropagation();
    if (e.touches.length === 2 && pinchRef.current) {
      const dist = touchDistance(e.touches[0], e.touches[1]);
      const ratio = dist / Math.max(1, pinchRef.current.dist);
      applyTransform({
        scale: pinchRef.current.scale * ratio,
        x: pinchRef.current.x,
        y: pinchRef.current.y,
      });
      return;
    }
    if (e.touches.length === 1 && panRef.current) {
      const dx = e.touches[0].clientX - panRef.current.x;
      const dy = e.touches[0].clientY - panRef.current.y;
      applyTransform({
        scale: transformRef.current.scale,
        x: panRef.current.ox + dx,
        y: panRef.current.oy + dy,
      });
    }
  };

  const onTouchEnd = (e) => {
    if (e.touches.length < 2) pinchRef.current = null;
    if (e.touches.length === 0) panRef.current = null;
    if (transformRef.current.scale <= 1.05) {
      applyTransform({ scale: 1, x: 0, y: 0 });
    }
  };

  const onWheel = (e) => {
    e.preventDefault();
    const cur = transformRef.current;
    const next = cur.scale * (e.deltaY < 0 ? 1.1 : 0.9);
    applyTransform({ scale: next, x: cur.x, y: cur.y });
  };

  return (
    <div
      className="fixed inset-0 z-[200] flex items-center justify-center bg-black/90 overscroll-none"
      style={{ touchAction: 'none' }}
      onClick={onClose}
      onTouchStart={onTouchStart}
      onTouchMove={onTouchMove}
      onTouchEnd={onTouchEnd}
      onWheel={onWheel}
      role="presentation"
    >
      <img
        src={src}
        alt=""
        draggable={false}
        className="max-w-[95vw] max-h-[95vh] object-contain rounded-lg select-none"
        style={{
          transform: `translate3d(${x}px, ${y}px, 0) scale(${scale})`,
          transformOrigin: 'center center',
          touchAction: 'none',
          cursor: scale > 1 ? 'grab' : 'zoom-in',
        }}
        onClick={(e) => e.stopPropagation()}
      />
      <div className="pointer-events-none absolute bottom-6 left-0 right-0 text-center text-[11px] text-white/55 px-4">
        雙指縮放 · 放大後可拖曳 · 雙擊切換 · 點空白關閉
      </div>
    </div>
  );
}

