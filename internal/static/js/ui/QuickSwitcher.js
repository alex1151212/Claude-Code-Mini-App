/** Ctrl/Cmd+P 快速跳轉 session（類 VS Code Quick Open） */
function QuickSwitcher({ sessions, onSelect }) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [idx, setIdx] = useState(0);
  const listRef = useRef(null);

  useEffect(() => {
    const onKey = (e) => {
      if ((e.ctrlKey || e.metaKey) && !e.shiftKey && !e.altKey && e.key.toLowerCase() === 'p') {
        e.preventDefault(); // 擋掉瀏覽器列印
        setQuery('');
        setIdx(0);
        setOpen(true); // 只開不切換（同 VS Code），Esc 才關
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);

  const matches = open ? filterMentionSessions(sessions, null, query) : [];

  useEffect(() => {
    listRef.current?.children[idx]?.scrollIntoView({ block: 'nearest' });
  }, [idx, open]);

  if (!open) return null;

  const pick = (s) => {
    if (!s) return;
    setOpen(false);
    onSelect(s);
  };

  const onInputKey = (e) => {
    if (e.key === 'Escape') setOpen(false);
    else if (e.key === 'ArrowDown') { e.preventDefault(); setIdx((i) => Math.min(i + 1, matches.length - 1)); }
    else if (e.key === 'ArrowUp') { e.preventDefault(); setIdx((i) => Math.max(i - 1, 0)); }
    else if (e.key === 'Enter' && !e.nativeEvent.isComposing) pick(matches[idx]); // 輸入法選字的 Enter 不算
  };

  return (
    <div
      className="fixed inset-0 z-[110] flex items-start justify-center pt-[15vh] px-4 bg-black/50"
      onClick={() => setOpen(false)}
      role="presentation"
    >
      <div
        className="w-full max-w-xl rounded-xl border border-slate-700 bg-slate-950/95 shadow-2xl shadow-black/50 overflow-hidden"
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-label="快速跳轉 Session"
      >
        <input
          autoFocus
          value={query}
          onChange={(e) => { setQuery(e.target.value); setIdx(0); }}
          onKeyDown={onInputKey}
          placeholder="搜尋 session（名稱 / 目錄 / 分支）"
          className="w-full bg-transparent border-b border-slate-700 px-4 py-3 text-sm text-slate-100 placeholder-slate-500 focus:outline-none"
        />
        <div ref={listRef} className="max-h-[50vh] overflow-y-auto app-scroll py-1">
          {matches.length === 0 && <div className="px-4 py-3 text-xs text-slate-500">沒有符合的 session</div>}
          {matches.map((s, i) => (
            <button
              key={s.id}
              type="button"
              onClick={() => pick(s)}
              onMouseMove={() => setIdx(i)}
              className={`w-full flex items-center gap-2 px-4 py-2 text-left ${i === idx ? 'bg-cyan-500/15' : ''}`}
            >
              <span
                className={`inline-flex shrink-0 items-center justify-center w-[22px] h-[22px] rounded-[6px] ${getAgentBadgeClass(s.agent_type)}`}
                title={s.agent_type || 'claude'}
              >
                <AgentBadgeIcon agentType={s.agent_type} />
              </span>
              <span className="min-w-0 truncate text-sm text-slate-100">{s.name || '未命名'}</span>
              <span className="min-w-0 flex-1 truncate font-mono text-[11px] text-slate-500" title={s.work_dir || ''}>
                {workDirGroupShortLabel(s.work_dir)}
              </span>
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}
