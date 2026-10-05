// 桌面殼（cmd/desktop）專用：無邊框視窗的透明標題列、拖曳、雙擊最大化與邊緣縮放。
// 一般 <script>（不經 Babel、不依賴 React），只在網址帶 ?desktop=1 且跑在 WebView2 裡時啟用；
// 網頁版與 Telegram Mini App 完全不受影響。
//
// 頁面不是從 Wails 資源伺服器載入，拿不到完整 runtime（drag.ts），只有 chrome.webview.postMessage。
// 所以照 Wails 的訊息協定自己送：
//   wails:drag / wails:resize:<edge> → Wails 內建處理（原生移動、縮放）
//   ra:window:*                      → cmd/desktop 的 RawMessageHandler
(function () {
  const webview = window.chrome && window.chrome.webview;
  if (!webview || !new URLSearchParams(location.search).has('desktop')) return;

  const post = (msg) => webview.postMessage(msg);
  document.documentElement.classList.add('ra-desktop');

  // ── 邊緣縮放：無邊框視窗沒有原生縮放邊框，滑鼠在邊緣時改游標、按下就交給 Windows 縮放 ──
  const EDGE = 5;
  const CORNER = 15;
  const CURSORS = {
    'n-resize': 'ns-resize', 's-resize': 'ns-resize',
    'e-resize': 'ew-resize', 'w-resize': 'ew-resize',
    'nw-resize': 'nwse-resize', 'se-resize': 'nwse-resize',
    'ne-resize': 'nesw-resize', 'sw-resize': 'nesw-resize',
  };
  const cursorStyle = document.createElement('style');
  document.head.appendChild(cursorStyle);
  let edge = '';

  // 最大化時視窗剛好填滿工作區，此時不提供縮放、按鈕換成「還原」圖示。
  const isMaximised = () => window.outerWidth >= screen.availWidth && window.outerHeight >= screen.availHeight;

  function edgeAt(x, y) {
    if (isMaximised()) return '';
    const w = window.innerWidth;
    const h = window.innerHeight;
    const top = y < EDGE, bottom = y >= h - EDGE, left = x < EDGE, right = x >= w - EDGE;
    if (!top && !bottom && !left && !right) return '';
    const nearTop = y < CORNER, nearBottom = y >= h - CORNER;
    const nearLeft = x < CORNER, nearRight = x >= w - CORNER;
    if (nearTop && nearLeft) return 'nw-resize';
    if (nearTop && nearRight) return 'ne-resize';
    if (nearBottom && nearLeft) return 'sw-resize';
    if (nearBottom && nearRight) return 'se-resize';
    if (top) return 'n-resize';
    if (bottom) return 's-resize';
    if (left) return 'w-resize';
    return 'e-resize';
  }

  function setEdge(next) {
    if (next === edge) return;
    edge = next;
    // 用 !important 蓋過按鈕、輸入框自己的游標，否則貼邊的元素會吃掉縮放游標。
    cursorStyle.textContent = edge ? `*{cursor:${CURSORS[edge]}!important}` : '';
  }

  // ── 拖曳：按下先記著，滑鼠一動才開始移動視窗（與 Wails drag.ts 相同），雙擊才收得到 ──
  let dragArmed = false;

  window.addEventListener('mousedown', (e) => {
    if (e.button !== 0) return;
    if (edge) {
      e.preventDefault();
      e.stopPropagation();
      post('wails:resize:' + edge);
      return;
    }
    dragArmed = !!(e.target.closest && e.target.closest('#ra-titlebar .ra-tb-drag'));
  }, true);

  window.addEventListener('mousemove', (e) => {
    if (dragArmed) {
      dragArmed = false;
      if (e.buttons & 1) post('wails:drag');
      return;
    }
    if (!e.buttons) setEdge(edgeAt(e.clientX, e.clientY));
  }, true);

  window.addEventListener('mouseup', () => { dragArmed = false; }, true);
  document.documentElement.addEventListener('mouseleave', () => setEdge(''));

  // ── 標題列本體 ──
  // 圖示用 Windows 內建字型（Win11 Segoe Fluent Icons，Win10 退回 Segoe MDL2 Assets，碼位相同）。
  const GLYPH_MAXIMISE = '';
  const GLYPH_RESTORE = '';

  function mount() {
    const bar = document.createElement('div');
    bar.id = 'ra-titlebar';

    const drag = document.createElement('div');
    drag.className = 'ra-tb-drag';
    drag.textContent = document.title;
    drag.addEventListener('dblclick', () => post('ra:window:toggle-maximise'));
    bar.appendChild(drag);

    const button = (act, glyph, label, extraClass) => {
      const b = document.createElement('button');
      b.type = 'button';
      b.className = 'ra-tb-btn' + (extraClass ? ' ' + extraClass : '');
      b.textContent = glyph;
      b.title = label;
      b.setAttribute('aria-label', label);
      b.addEventListener('click', () => post('ra:window:' + act));
      bar.appendChild(b);
      return b;
    };
    button('minimise', '', '最小化');
    const maxBtn = button('toggle-maximise', GLYPH_MAXIMISE, '最大化');
    button('close', '', '關閉', 'ra-tb-close');

    const syncMaxButton = () => {
      const max = isMaximised();
      maxBtn.textContent = max ? GLYPH_RESTORE : GLYPH_MAXIMISE;
      maxBtn.title = max ? '還原' : '最大化';
      maxBtn.setAttribute('aria-label', maxBtn.title);
    };
    syncMaxButton();
    window.addEventListener('resize', syncMaxButton);

    // 視窗失焦時標題列變淡，跟原生標題列一樣。
    window.addEventListener('blur', () => bar.classList.add('ra-tb-inactive'));
    window.addEventListener('focus', () => bar.classList.remove('ra-tb-inactive'));

    document.body.appendChild(bar);
  }

  if (document.body) mount();
  else document.addEventListener('DOMContentLoaded', mount, { once: true });
})();
