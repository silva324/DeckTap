/* DeckTap phone client – SPEC-SEC-001 E2EE transport with Anti-Shoulder-Surfing PIN, vanilla JS. */
(() => {
  "use strict";
  const C = window.DeckTapCrypto || window.SlideCueCrypto;
  const $ = (id) => document.getElementById(id);
  const memStore = {};
  const ss = {
    get: (k) => {
      try {
        const val = localStorage.getItem("dt." + k) || sessionStorage.getItem("dt." + k) || sessionStorage.getItem("sc." + k);
        return val !== null ? val : (memStore[k] || null);
      } catch {
        return memStore[k] || null;
      }
    },
    set: (k, v) => {
      try {
        localStorage.setItem("dt." + k, String(v));
        sessionStorage.setItem("dt." + k, String(v));
      } catch {}
      memStore[k] = String(v);
    },
    del: (k) => {
      try {
        localStorage.removeItem("dt." + k);
        sessionStorage.removeItem("dt." + k);
        sessionStorage.removeItem("sc." + k);
      } catch {}
      delete memStore[k];
    },
    clear: () => ["key", "token", "sid", "cseq", "hseq", "timer", "server_host", "server_proto"].forEach((k) => ss.del(k)),
  };
  const prefs = {
    get: (k, d) => { try { const v = localStorage.getItem("sc.pref." + k); return v === null ? d : JSON.parse(v); } catch { return d; } },
    set: (k, v) => { try { localStorage.setItem("sc.pref." + k, JSON.stringify(v)); } catch { /* ignore */ } },
  };

  let key = null, ws = null, authed = false, terminal = false, pendingNonce = null;
  let state = null, deck = null, retries = 0, retryTimer = null, pingTimer = null;
  let outQueue = Promise.resolve(), inQueue = Promise.resolve();
  let clockOffset = 0, wakeLock = null, toastTimer = null;
  let enteredPin = "";

  // ------------------------------------------------------------ preferences
  let fontSize = prefs.get("font", 22), theme = prefs.get("theme", "auto");
  function applyPrefs() {
    document.documentElement.style.setProperty("--notes-size", fontSize + "px");
    const dark = theme === "dark" || (theme === "auto" && matchMedia("(prefers-color-scheme: dark)").matches);
    document.documentElement.dataset.theme = dark ? "dark" : "light";
    $("t-theme").querySelector("span").textContent = "Theme: " + theme;
  }

  // ------------------------------------------------------------ ui helpers
  function show(id) { for (const s of document.querySelectorAll(".screen")) s.classList.toggle("hidden", s.id !== id); }
  function message(title, text, retry, badge) {
    $("msg-title").textContent = title; $("msg-text").textContent = text;
    $("msg-retry").classList.toggle("hidden", !retry);
    $("conn").classList.add("hidden");
    const badgeEl = $("conn-info-badge");
    if (badgeEl) {
      if (badge) {
        badgeEl.classList.remove("hidden");
        const dot = badgeEl.querySelector(".conn-badge-dot");
        if (dot) dot.classList.toggle("disconnected", badge.type === "disconnected");
        $("conn-info-text").textContent = badge.text;
      } else {
        badgeEl.classList.add("hidden");
      }
    }
    show("screen-msg");
  }
  function toast(msg, ms = 3000) {
    const t = $("toast"); t.textContent = msg; t.classList.remove("hidden");
    clearTimeout(toastTimer); toastTimer = setTimeout(() => t.classList.add("hidden"), ms);
  }
  function buzz() { if (navigator.vibrate) navigator.vibrate(12); }
  function fmt(sec) {
    sec = Math.max(0, Math.floor(sec));
    const h = Math.floor(sec / 3600), m = Math.floor((sec % 3600) / 60), s = sec % 60;
    const mm = String(m).padStart(2, "0"), s2 = String(s).padStart(2, "0");
    return h ? `${h}:${mm}:${s2}` : `${mm}:${s2}`;
  }

  // ------------------------------------------------------------ PIN Pad Logic
  function enterPinDigit(d) {
    if (enteredPin.length >= 4) return;
    buzz();
    enteredPin += d;
    renderPinDots();
    if (enteredPin.length === 4) {
      $("pin-status").textContent = "Verifying with host…";
      show("screen-connecting");
      connect();
    }
  }
  function backspacePin() {
    if (enteredPin.length > 0) {
      buzz();
      enteredPin = enteredPin.slice(0, -1);
      renderPinDots();
      $("pin-status").textContent = "";
    }
  }
  function clearPin() {
    enteredPin = "";
    renderPinDots();
    $("pin-status").textContent = "";
  }
  function renderPinDots() {
    for (let i = 0; i < 4; i++) {
      const el = $("pd-" + i);
      if (el) {
        if (i < enteredPin.length) {
          el.textContent = "●";
          el.classList.add("active");
          el.classList.remove("error");
        } else {
          el.textContent = "-";
          el.classList.remove("active", "error");
        }
      }
    }
  }
  function triggerPinError(msg) {
    buzz();
    for (let i = 0; i < 4; i++) {
      const el = $("pd-" + i);
      if (el) el.classList.add("error");
    }
    $("pin-status").textContent = msg || "Invalid PIN. Please check laptop screen.";
    show("screen-pin");
    setTimeout(() => {
      clearPin();
    }, 1000);
  }

  // ------------------------------------------------------------ secure channel
  function nextSeq() { const s = Number(ss.get("cseq") || 0) + 1; ss.set("cseq", s); return s; }

  /** Encrypt and send in strict order (seq order == wire order, as the host requires). */
  function enqueue(build) {
    outQueue = outQueue.then(async () => {
      if (!ws || ws.readyState !== WebSocket.OPEN) return;
      const payload = build();
      payload.seq = nextSeq();
      ws.send(await C.seal(key, payload));
    }).catch((e) => console.warn("send failed", e));
  }

  function command(action, extra) {
    if (!authed) { toast("Not connected yet…"); return; }
    if (action !== "POINTER") buzz();
    enqueue(() => {
      const p = Object.assign({ sid: ss.get("sid"), action }, typeof extra === "object" ? extra : (extra !== undefined ? { slide: extra } : {}));
      return p;
    });
  }

  function connect() {
    clearTimeout(retryTimer);
    if (terminal || !key) return;
    if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) return;
    if (!state && authed) show("screen-connecting");
    const targetHost = ss.get("server_host") || location.host;
    const targetProto = ss.get("server_proto") || (location.protocol === "https:" ? "https:" : "http:");
    const proto = targetProto === "https:" ? "wss" : "ws";
    const sock = new WebSocket(`${proto}://${targetHost}/ws`);
    sock.binaryType = "arraybuffer";
    ws = sock;
    sock.onopen = () => {
      enqueue(() => {
        pendingNonce = C.randomHex(16);
        const base = { clientNonce: pendingNonce, ts: Math.floor(Date.now() / 1000) };
        const sid = ss.get("sid"), token = ss.get("token");
        if (sid) return Object.assign(base, { type: "AUTH_RESUME", sessionId: sid });
        return Object.assign(base, { type: "AUTH_HELLO", pairingToken: token, pin: enteredPin });
      });
    };
    sock.onmessage = (ev) => { inQueue = inQueue.then(() => onFrame(ev.data)).catch((e) => console.warn(e)); };
    sock.onclose = (ev) => { if (ws === sock) onClose(ev); };
  }

  async function onFrame(data) {
    let m;
    try { m = await C.open(key, data); } catch { console.warn("dropped frame: authentication failed"); return; }
    lastHeartbeat = Date.now();
    clearTimeout(pendingPingTimeout);
    pendingPingTimeout = null;
    const last = Number(ss.get("hseq") || 0);
    if (!(m.seq > last)) { console.warn("dropped frame: replayed seq", m.seq); return; }
    ss.set("hseq", m.seq);
    switch (m.type) {
      case "AUTH_OK":
        if (m.clientNonce !== pendingNonce) { console.warn("AUTH_OK nonce mismatch"); return; }
        ss.set("sid", m.sessionId); ss.del("token"); // the pairing token is burned
        authed = true; retries = 0;
        clockOffset = (m.serverTime || Date.now()) - Date.now();
        state = m.state || state;
        $("conn").classList.add("hidden");
        $("conn").textContent = "Reconnecting…";
        startPing(); render();
        break;
      case "AUTH_FAIL":
        triggerPinError(m.error || "Invalid PIN code");
        break;
      case "STATE_UPDATE": state = m; render(); break;
      case "DECK": deck = m; renderSlideList(); render(); break;
      case "ERROR": toast(m.message || "Command failed"); break;
      case "PONG":
        lastHeartbeat = Date.now();
        clearTimeout(pendingPingTimeout);
        pendingPingTimeout = null;
        break;
    }
  }

  function onClose(ev) {
    authed = false; stopPing();
    const hadSession = !!ss.get("sid");
    if (!hadSession && enteredPin.length === 4) {
      triggerPinError("Incorrect PIN. Please check the 4-digit PIN on your computer screen.");
      return;
    }
    const end = (title, text, clear, retry) => { terminal = true; if (clear) { ss.clear(); key = null; } message(title, text, retry); };
    switch (ev.code) {
      case 4000: return end("Opened Elsewhere", "This presentation was opened on another phone or tab.", false, true);
      case 4001: return end("New Code Created", "A new pairing code was created on your computer. Scan the new code to reconnect.", true);
      case 4002: return end("DeckTap Closed", "DeckTap was closed on your computer.", true);
      case 1008: return end(hadSession ? "Session Ended" : "Pairing Code Expired",
        hadSession ? "The presentation session ended. Scan a new QR code on your computer to connect again."
                   : "This pairing code has expired or was already used. Click 'New Pairing Code' on your computer and scan again.", true);
    }
    retries++;
    if (!hadSession && retries > 10) {
      return end("Cannot Reach Computer", "Unable to connect. Make sure DeckTap is open on your computer and your phone has an internet connection.", false, true);
    }
    const connEl = $("conn");
    connEl.classList.remove("hidden");
    if (retries > 6) {
      connEl.textContent = "Reconnecting… (tap to retry)";
    } else {
      connEl.textContent = "Reconnecting…";
    }
    const delay = Math.min(500 * (2 ** Math.min(retries - 1, 4)), 5000);
    retryTimer = setTimeout(connect, delay);
  }

  let pendingPingTimeout = null;
  let lastHeartbeat = Date.now();

  function stopPing() {
    clearInterval(pingTimer);
    clearTimeout(pendingPingTimeout);
    pingTimer = null;
    pendingPingTimeout = null;
  }

  function pingHost() {
    if (!ws || ws.readyState !== WebSocket.OPEN || !authed) return;
    enqueue(() => ({ sid: ss.get("sid"), action: "PING" }));
    clearTimeout(pendingPingTimeout);
    pendingPingTimeout = setTimeout(() => {
      // If no frame or pong received within 8 seconds after pinging, declare socket dead!
      if (ws && ws.readyState === WebSocket.OPEN && Date.now() - lastHeartbeat >= 8000) {
        console.warn("[DeckTap] Ping timeout (dead socket). Reconnecting…");
        try { ws.close(); } catch {}
      }
    }, 8000);
  }

  function startPing() {
    stopPing();
    lastHeartbeat = Date.now();
    pingTimer = setInterval(pingHost, 15000);
  }

  // ------------------------------------------------------------ rendering
  function render() {
    if (terminal) return;
    if (!state) { show("screen-connecting"); return; }
    if (state.isPresenting) renderShow(); else renderReady();
    updatePointerWatermark();
  }

  function renderReady() {
    show("screen-ready");
    const s = state, btn = $("btn-start");
    let name = s.presentation || "No presentation open", meta = "", ok = true;
    if (!s.appRunning) { name = "PowerPoint is not running"; meta = "Open your presentation on the laptop."; ok = false; }
    else if (!s.presentation) { meta = "Open a presentation in PowerPoint on the laptop."; ok = false; }
    else {
      const withNotes = deck && deck.slides ? deck.slides.filter((x) => x.hasNotes).length : null;
      meta = `${s.totalSlides} slides` + (withNotes === null ? "" : ` · ${withNotes} with notes`);
    }
    $("ready-name").textContent = name; $("ready-meta").textContent = meta;
    btn.disabled = !ok; $("start-at").max = s.totalSlides || 1;
    renderDecksList();
    if (s.error) toast(s.error);
  }

  function renderShow() {
    show("screen-show");
    const s = state;
    $("pos-cur").textContent = s.currentSlide || "–";
    $("pos-total").textContent = s.totalSlides || "–";
    $("show-deck-title").textContent = s.presentation || "Presentation";
    $("progress-bar").style.width = s.totalSlides ? (100 * s.currentSlide / s.totalSlides) + "%" : "0";
    $("slide-title").textContent = s.title || `Slide ${s.currentSlide}`;
    $("next-title").textContent = s.ended ? "" : s.nextSlide ? `Next: ${s.nextTitle || "Slide " + s.nextSlide}` : "Last slide";
    const notes = $("notes");
    notes.textContent = s.notes || "No notes for this slide.";
    notes.classList.toggle("empty", !s.notes);
    if (notes.dataset.idx !== String(s.currentSlide)) { notes.scrollTop = 0; notes.dataset.idx = s.currentSlide; }
    let line = "", warn = false;
    if (s.ended) line = "End of slide show: Next exits, Prev goes back";
    else if (s.blackout) line = "Screen is black: Next/Prev resumes";
    else if (s.hidden) { line = "This slide is hidden"; warn = true; }
    if (s.error) { line = s.error; warn = true; }
    const st = $("status-line");
    st.textContent = line; st.classList.toggle("hidden", !line); st.classList.toggle("warn", warn);
    $("t-black").classList.toggle("active", !!s.blackout);
    for (const el of $("slide-list").children) el.classList.toggle("current", Number(el.dataset.n) === s.currentSlide);
    renderDecksList();
    keepAwake();
  }

  function renderDecksList() {
    const ul = $("deck-list");
    if (!ul) return;
    ul.replaceChildren();
    const decks = (state && state.decks) || [];
    if (decks.length === 0 && state && state.presentation) {
      decks.push({ name: state.presentation, total: state.totalSlides, active: true });
    }
    for (const d of decks) {
      const li = document.createElement("li");
      if (d.active) li.className = "active-deck";
      const info = document.createElement("div");
      info.className = "deck-info";
      const t = document.createElement("div");
      t.className = "deck-title";
      t.textContent = d.name;
      const sub = document.createElement("div");
      sub.className = "deck-sub";
      sub.textContent = `${d.total} slides`;
      info.append(t, sub);
      li.appendChild(info);

      if (d.showing) {
        const badge = document.createElement("span");
        badge.className = "badge-tag";
        badge.textContent = "Presenting";
        li.appendChild(badge);
      } else if (d.active) {
        const badge = document.createElement("span");
        badge.className = "badge-tag";
        badge.textContent = "Active";
        li.appendChild(badge);
      }

      li.addEventListener("click", () => {
        closeDecksSheet();
        if (d.name !== (state && state.presentation)) {
          toast("Switching to " + d.name + "…");
          command("SWITCH_DECK", { deck: d.name });
        }
      });
      ul.appendChild(li);
    }
  }

  function renderSlideList() {
    const ul = $("slide-list"); ul.replaceChildren();
    if (!deck || !deck.slides) return;
    for (const sl of deck.slides) {
      const li = document.createElement("li");
      if (sl.hidden) li.className = "is-hidden";
      li.dataset.n = sl.n;
      const n = document.createElement("span"); n.className = "n"; n.textContent = sl.n;
      const t = document.createElement("span"); t.className = "name"; t.textContent = sl.title || "(untitled)";
      li.append(n, t);
      li.addEventListener("click", () => { closeSheet(); command("GOTO", sl.n); });
      ul.appendChild(li);
    }
  }

  // ------------------------------------------------------------ timer, clock, wake lock
  setInterval(() => {
    $("clock").textContent = new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
    if (!state || !state.showStartedAt) { $("timer").textContent = "00:00"; return; }
    const override = JSON.parse(ss.get("timer") || "null");
    const base = override && override.show === state.showStartedAt ? override.at : state.showStartedAt;
    $("timer").textContent = fmt((Date.now() + clockOffset - base) / 1000);
  }, 500);
  $("timer").addEventListener("click", () => {
    if (state && state.showStartedAt) { ss.set("timer", JSON.stringify({ show: state.showStartedAt, at: Date.now() + clockOffset })); buzz(); }
  });

  async function keepAwake() {
    if (wakeLock || !("wakeLock" in navigator) || document.visibilityState !== "visible") return;
    try { wakeLock = await navigator.wakeLock.request("screen"); wakeLock.addEventListener("release", () => { wakeLock = null; }); } catch { /* not allowed */ }
  }
  function onForeground() {
    if (document.visibilityState !== "visible") return;
    keepAwake();
    if (key && (ss.get("sid") || ss.get("token"))) {
      terminal = false; // Always permit re-connection when presenter returns to the app
      const dead = !ws || ws.readyState === WebSocket.CLOSED || ws.readyState === WebSocket.CLOSING || (Date.now() - lastHeartbeat > 25000);
      if (dead) {
        console.log("[DeckTap] Reconnecting after background/sleep…");
        retries = 0;
        clearTimeout(retryTimer);
        if (ws) {
          try { ws.close(); } catch {}
          ws = null;
        }
        connect();
      } else if (ws && ws.readyState === WebSocket.OPEN) {
        pingHost();
      }
    }
  }

  document.addEventListener("visibilitychange", onForeground);
  window.addEventListener("pageshow", onForeground);
  window.addEventListener("focus", onForeground);
  window.addEventListener("online", onForeground);

  const connBanner = $("conn");
  if (connBanner) {
    connBanner.addEventListener("click", () => {
      connBanner.textContent = "Reconnecting…";
      retries = 0;
      terminal = false;
      clearTimeout(retryTimer);
      if (ws) {
        try { ws.close(); } catch {}
        ws = null;
      }
      connect();
    });
  }

  // ------------------------------------------------------------ input
  const next = () => command(state && state.ended ? "END_SHOW" : "NEXT");
  $("btn-next").addEventListener("click", next);
  $("btn-prev").addEventListener("click", () => command("PREV"));
  $("btn-start").addEventListener("click", () => command("START_SHOW", Math.max(1, Number($("start-at").value) || 1)));
  $("msg-retry").addEventListener("click", () => { terminal = false; retries = 0; if (key) connect(); else location.reload(); });

  let sx = 0, sy = 0, st0 = 0;
  const notes = $("notes");
  notes.addEventListener("touchstart", (e) => { const t = e.touches[0]; sx = t.clientX; sy = t.clientY; st0 = Date.now(); }, { passive: true });
  notes.addEventListener("touchend", (e) => {
    const t = e.changedTouches[0], dx = t.clientX - sx, dy = t.clientY - sy;
    if (Math.abs(dx) < 60 || Math.abs(dx) < Math.abs(dy) * 1.5 || Date.now() - st0 > 800) return;
    notes.classList.add(dx < 0 ? "swipe-l" : "swipe-r");
    setTimeout(() => notes.classList.remove("swipe-l", "swipe-r"), 150);
    if (dx < 0) next(); else command("PREV");
  }, { passive: true });

  document.addEventListener("keydown", (e) => {
    if (!state || !state.isPresenting || e.target.tagName === "INPUT") return;
    if (["ArrowRight", "PageDown", " ", "Enter"].includes(e.key)) { e.preventDefault(); next(); }
    else if (["ArrowLeft", "PageUp", "Backspace"].includes(e.key)) { e.preventDefault(); command("PREV"); }
    else if (e.key === "b" || e.key === ".") command("BLACK");
  });

  // ------------------------------------------------------------ Viewport & Bar Scroll Locking
  // Dynamically sync visual viewport height so bars never get covered by mobile browser UI
  if (window.visualViewport) {
    const syncViewportHeight = () => {
      const h = window.visualViewport.height;
      document.documentElement.style.setProperty("--visual-h", `${h}px`);
    };
    window.visualViewport.addEventListener("resize", syncViewportHeight);
    window.visualViewport.addEventListener("scroll", syncViewportHeight);
    syncViewportHeight();
  }

  // Prevent rubber-band dragging of the window while preserving smooth scroll inside notes/lists
  let scrollTouchStartY = 0;
  document.addEventListener("touchstart", (e) => {
    if (e.touches && e.touches.length) {
      scrollTouchStartY = e.touches[0].clientY;
    }
  }, { passive: true });

  document.addEventListener("touchmove", (e) => {
    const scrollable = e.target.closest(".notes, .content, .sheet-panel, .list");
    if (!scrollable) {
      e.preventDefault();
      return;
    }
    const deltaY = e.touches[0].clientY - scrollTouchStartY;
    const isAtTop = scrollable.scrollTop <= 0 && deltaY > 0;
    const isAtBottom = (scrollable.scrollTop + scrollable.clientHeight >= scrollable.scrollHeight) && deltaY < 0;
    if (isAtTop || isAtBottom) {
      e.preventDefault();
    }
  }, { passive: false });

  document.querySelectorAll(".bar, .preview, .controls, .pointer-pad-header, .pointer-pad-bar").forEach((el) => {
    el.addEventListener("touchmove", (e) => { e.preventDefault(); }, { passive: false });
  });

  const notesEl = $("notes");
  if (notesEl) {
    notesEl.addEventListener("touchstart", () => {
      const top = notesEl.scrollTop;
      const total = notesEl.scrollHeight;
      const current = top + notesEl.offsetHeight;
      if (top === 0) {
        notesEl.scrollTop = 1;
      } else if (current >= total) {
        notesEl.scrollTop = top - 1;
      }
    }, { passive: true });
  }

  function openSheet() { $("sheet").classList.remove("hidden"); const c = $("slide-list").querySelector(".current"); if (c) c.scrollIntoView({ block: "center" }); }
  function closeSheet() { $("sheet").classList.add("hidden"); }
  $("btn-menu").addEventListener("click", openSheet);
  $("t-close").addEventListener("click", closeSheet);
  $("sheet").addEventListener("click", (e) => { if (e.target.id === "sheet") closeSheet(); });

  // ------------------------------------------------------------ Deck Switcher Sheet
  function openDecksSheet() { renderDecksList(); $("sheet-decks").classList.remove("hidden"); }
  function closeDecksSheet() { $("sheet-decks").classList.add("hidden"); }
  const btnDecksReady = $("btn-decks-ready");
  if (btnDecksReady) btnDecksReady.addEventListener("click", openDecksSheet);
  const btnDecksShow = $("btn-decks-show");
  if (btnDecksShow) btnDecksShow.addEventListener("click", openDecksSheet);
  const decksClose = $("decks-close");
  if (decksClose) decksClose.addEventListener("click", closeDecksSheet);
  $("sheet-decks").addEventListener("click", (e) => { if (e.target.id === "sheet-decks") closeDecksSheet(); });

  // ------------------------------------------------------------ Virtual Laser & Spotlight (Gyro + Touch)
  // ------------------------------------------------------------ Laser / Spotlight & Gyro
  let pointerActive = false, pointerMode = "laser";
  let refBeta = null, refGamma = null;
  let latestBeta = null, latestGamma = null;
  let curX = 0.5, curY = 0.5, lastSent = 0;
  let isTouching = false;
  let touchStartX = 0, touchStartY = 0;
  let touchStartCurX = 0.5, touchStartCurY = 0.5;
  let lastTapTime = 0;

  function updateReticle(x, y) {
    const area = $("pointer-slide-canvas") || $("pointer-pad-area");
    const ret = $("pointer-reticle");
    if (!area || !ret) return;
    ret.style.left = (x * 100) + "%";
    ret.style.top = (y * 100) + "%";
    if (pointerMode === "spotlight") {
      ret.style.width = "72px";
      ret.style.height = "72px";
      ret.style.borderColor = "#f59e0b";
      ret.style.backgroundColor = "rgba(245, 158, 11, 0.25)";
    } else {
      ret.style.width = "36px";
      ret.style.height = "36px";
      ret.style.borderColor = "#ef4444";
      ret.style.backgroundColor = "rgba(239, 68, 68, 0.35)";
    }
  }

  function updatePointerWatermark() {
    const wmTitle = $("watermark-title");
    const wmNum = $("watermark-num");
    const wmTag = $("watermark-deck-tag");
    const wmContent = $("watermark-content");
    const wmBg = $("watermark-bg");

    const cur = state ? (state.currentSlide || 1) : 1;
    const tot = state ? (state.totalSlides || 1) : 1;
    const title = (state && state.title) ? state.title : ("Slide " + cur);

    if (wmTitle) wmTitle.textContent = title;
    if (wmNum) wmNum.textContent = cur;
    if (wmTag) wmTag.textContent = cur + " / " + tot;

    if (wmContent) {
      const notes = (state && state.notes) ? state.notes.trim() : "";
      if (notes) {
        const lines = notes.split("\n").map(l => l.trim()).filter(Boolean).slice(0, 3);
        wmContent.innerHTML = lines.map(l => `<div class="wm-line">• ${l.replace(/&/g, "&amp;").replace(/</g, "&lt;")}</div>`).join("");
      } else {
        wmContent.innerHTML = `<div class="wm-line" style="opacity: 0.18;">Widescreen 16:9 Canvas</div>`;
      }
    }

    if (wmBg && state && state.slideImage) {
      wmBg.style.backgroundImage = `url(${state.slideImage})`;
      wmBg.style.display = "block";
    }
  }

  function sendPointer(x, y) {
    curX = Math.max(0.01, Math.min(0.99, x));
    curY = Math.max(0.01, Math.min(0.99, y));
    updateReticle(curX, curY);
    const now = Date.now();
    if (now - lastSent < 35) return;
    lastSent = now;
    command("POINTER", { active: true, x: Number(curX.toFixed(3)), y: Number(curY.toFixed(3)), mode: pointerMode });
  }

  function recenterGyro(targetX, targetY) {
    if (latestBeta === null || latestGamma === null) return;
    refGamma = latestGamma - ((targetX - 0.5) * 45.0);
    refBeta = latestBeta - ((targetY - 0.5) * 45.0);
  }

  function centerPointer() {
    curX = 0.5;
    curY = 0.5;
    recenterGyro(0.5, 0.5);
    sendPointer(0.5, 0.5);
    buzz();
  }

  function handleOrientation(e) {
    if (!pointerActive) return;
    const beta = e.beta;
    const gamma = e.gamma;
    if (beta === null || gamma === null) return;
    latestBeta = beta;
    latestGamma = gamma;

    // While user is dragging with finger, suspend gyro so it doesn't fight the finger
    if (isTouching) return;

    if (refBeta === null || refGamma === null) {
      recenterGyro(curX, curY);
      return;
    }
    const dGamma = gamma - refGamma;
    const dBeta = beta - refBeta;
    const targetX = 0.5 + (dGamma / 45.0);
    const targetY = 0.5 + (dBeta / 45.0);
    const smoothX = curX * 0.4 + targetX * 0.6;
    const smoothY = curY * 0.4 + targetY * 0.6;
    sendPointer(smoothX, smoothY);
  }

  function setPointerMode(mode) {
    pointerMode = mode || "laser";
    const isLaser = pointerMode === "laser";
    const pLaser = $("p-pill-laser"), pSpot = $("p-pill-spot");
    if (pLaser) pLaser.classList.toggle("active", isLaser);
    if (pSpot) pSpot.classList.toggle("active", !isLaser);
    const bLaser = $("btn-laser"), bSpot = $("btn-spotlight");
    if (bLaser) bLaser.classList.toggle("active", isLaser);
    if (bSpot) bSpot.classList.toggle("active", !isLaser);
    updateReticle(curX, curY);
    if (pointerActive) {
      command("POINTER", { active: true, x: Number(curX.toFixed(3)), y: Number(curY.toFixed(3)), mode: pointerMode });
    }
  }

  async function startPointer(mode) {
    if (pointerActive && pointerMode === mode) { stopPointer(); return; }
    pointerActive = true;
    refBeta = null;
    refGamma = null;
    latestBeta = null;
    latestGamma = null;
    curX = 0.5;
    curY = 0.5;
    buzz();
    setPointerMode(mode);
    const pad = $("pointer-pad");
    if (pad) pad.classList.remove("hidden");
    updatePointerWatermark();
    updateReticle(0.5, 0.5);

    try {
      history.pushState({ decktap_pointer: true }, "");
    } catch {}

    if (typeof DeviceOrientationEvent !== "undefined" && typeof DeviceOrientationEvent.requestPermission === "function") {
      try {
        const res = await DeviceOrientationEvent.requestPermission();
        if (res !== "granted") console.warn("DeviceOrientation permission denied");
      } catch (err) {
        console.warn("DeviceOrientation request error", err);
      }
    }
    window.addEventListener("deviceorientation", handleOrientation);
    sendPointer(0.5, 0.5);
  }

  function stopPointer(fromPopState) {
    if (!pointerActive) return;
    pointerActive = false;
    isTouching = false;
    window.removeEventListener("deviceorientation", handleOrientation);
    const pad = $("pointer-pad");
    if (pad) pad.classList.add("hidden");
    const bLaser = $("btn-laser"), bSpot = $("btn-spotlight");
    if (bLaser) bLaser.classList.remove("active");
    if (bSpot) bSpot.classList.remove("active");
    buzz();
    command("POINTER", { active: false });

    if (!fromPopState && history.state && history.state.decktap_pointer) {
      try { history.back(); } catch {}
    }
  }

  const padArea = $("pointer-slide-canvas") || $("pointer-pad-area");

  function onTouchStart(e) {
    const t = e.touches ? e.touches[0] : e;
    const now = Date.now();
    if (now - lastTapTime < 320) {
      centerPointer();
      lastTapTime = 0;
      return;
    }
    lastTapTime = now;

    isTouching = true;
    touchStartX = t.clientX;
    touchStartY = t.clientY;
    touchStartCurX = curX;
    touchStartCurY = curY;
  }

  function onTouchMove(e) {
    if (!isTouching) return;
    const t = e.touches ? e.touches[0] : e;
    const r = padArea.getBoundingClientRect();
    const dx = (t.clientX - touchStartX) / r.width;
    const dy = (t.clientY - touchStartY) / r.height;

    const nextX = touchStartCurX + (dx * 1.25);
    const nextY = touchStartCurY + (dy * 1.25);
    sendPointer(nextX, nextY);
  }

  function onTouchEnd() {
    if (!isTouching) return;
    isTouching = false;
    recenterGyro(curX, curY);
  }

  if (padArea) {
    padArea.addEventListener("touchstart", (e) => { e.preventDefault(); onTouchStart(e); }, { passive: false });
    padArea.addEventListener("touchmove", (e) => { e.preventDefault(); onTouchMove(e); }, { passive: false });
    padArea.addEventListener("touchend", (e) => { e.preventDefault(); onTouchEnd(); }, { passive: false });
    padArea.addEventListener("touchcancel", () => { onTouchEnd(); });

    padArea.addEventListener("mousedown", (e) => {
      onTouchStart(e);
      const move = (ev) => onTouchMove(ev);
      const up = () => {
        window.removeEventListener("mousemove", move);
        window.removeEventListener("mouseup", up);
        onTouchEnd();
      };
      window.addEventListener("mousemove", move);
      window.addEventListener("mouseup", up);
    });
  }
  const padEl = $("pointer-pad");
  if (padEl) {
    padEl.addEventListener("click", (e) => {
      if (e.target.id === "pointer-pad" || e.target.classList.contains("pointer-pad-hint")) {
        stopPointer();
      }
    });
  }
  const btnRecenter = $("p-btn-recenter");
  if (btnRecenter) btnRecenter.addEventListener("click", () => centerPointer());
  const btnExitTop = $("pointer-exit-btn");
  if (btnExitTop) btnExitTop.addEventListener("click", () => stopPointer());
  const btnExitBottom = $("pointer-bottom-exit");
  if (btnExitBottom) btnExitBottom.addEventListener("click", () => stopPointer());
  const pillLaser = $("p-pill-laser");
  if (pillLaser) pillLaser.addEventListener("click", () => setPointerMode("laser"));
  const pillSpot = $("p-pill-spot");
  if (pillSpot) pillSpot.addEventListener("click", () => setPointerMode("spotlight"));
  const pNavPrev = $("p-nav-prev");
  if (pNavPrev) pNavPrev.addEventListener("click", () => { command("PREV_SLIDE"); buzz(); });
  const pNavNext = $("p-nav-next");
  if (pNavNext) pNavNext.addEventListener("click", () => { command("NEXT_SLIDE"); buzz(); });

  window.addEventListener("popstate", () => {
    if (pointerActive) stopPointer(true);
  });
  window.addEventListener("keydown", (e) => {
    if (pointerActive && e.key === "Escape") stopPointer();
  });
  const btnLaser = $("btn-laser");
  if (btnLaser) btnLaser.addEventListener("click", () => startPointer("laser"));
  const btnSpot = $("btn-spotlight");
  if (btnSpot) btnSpot.addEventListener("click", () => startPointer("spotlight"));
  $("t-black").addEventListener("click", () => { command("BLACK"); closeSheet(); });
  $("t-end").addEventListener("click", () => { if (confirm("End the slide show?")) { command("END_SHOW"); closeSheet(); } });
  $("t-smaller").addEventListener("click", () => { fontSize = Math.max(14, fontSize - 2); prefs.set("font", fontSize); applyPrefs(); });
  $("t-bigger").addEventListener("click", () => { fontSize = Math.min(44, fontSize + 2); prefs.set("font", fontSize); applyPrefs(); });
  $("t-theme").addEventListener("click", () => { theme = { auto: "light", light: "dark", dark: "auto" }[theme]; prefs.set("theme", theme); applyPrefs(); });
  function disconnect() {
    if (!confirm("Disconnect from this presentation?")) return;
    closeSheet();
    terminal = true;
    ss.clear();
    key = null;
    if (ws) {
      try { ws.close(1000); } catch {}
      ws = null;
    }
    message("Disconnected", "You are disconnected. Scan a new QR code on your computer whenever you're ready to present.", false, { type: "disconnected", text: "Disconnected" });
  }

  $("t-disconnect").addEventListener("click", disconnect);
  const btnReadyDisconnect = $("btn-ready-disconnect");
  if (btnReadyDisconnect) btnReadyDisconnect.addEventListener("click", disconnect);

  // ------------------------------------------------------------ PWA Installation & Service Worker
  let deferredInstallPrompt = null;
  const isStandalone = window.matchMedia("(display-mode: standalone)").matches || window.navigator.standalone === true;
  const isIOS = /iPhone|iPad|iPod/.test(navigator.userAgent) && !window.MSStream;

  if ("serviceWorker" in navigator) {
    window.addEventListener("load", () => {
      navigator.serviceWorker.register("sw.js").catch(() => {});
    });
  }

  function showInstallGuide() {
    if (deferredInstallPrompt) {
      deferredInstallPrompt.prompt();
      deferredInstallPrompt.userChoice.then(() => {
        deferredInstallPrompt = null;
        hideInstallBanner();
      });
    } else if (isIOS) {
      const iosSheet = $("sheet-ios-install");
      if (iosSheet) iosSheet.classList.remove("hidden");
    } else {
      toast("To install: tap browser menu (⋮) and choose 'Add to Home screen'");
    }
  }

  function hideInstallBanner() {
    const banner = $("install-banner");
    if (banner) banner.classList.add("hidden");
    try { localStorage.setItem("dt_install_dismissed", "1"); } catch {}
  }

  function checkInstallPrompt() {
    if (isStandalone) return;
    try {
      if (localStorage.getItem("dt_install_dismissed") === "1") return;
    } catch {}
    const banner = $("install-banner");
    if (banner) {
      setTimeout(() => {
        if (!isStandalone) banner.classList.remove("hidden");
      }, 2500);
    }
  }

  window.addEventListener("beforeinstallprompt", (e) => {
    e.preventDefault();
    deferredInstallPrompt = e;
    checkInstallPrompt();
  });

  // On iOS, check whether to suggest installation after delay
  if (isIOS && !isStandalone) {
    checkInstallPrompt();
  }

  const btnBannerInstall = $("btn-banner-install");
  if (btnBannerInstall) btnBannerInstall.addEventListener("click", showInstallGuide);
  const btnBannerDismiss = $("btn-banner-dismiss");
  if (btnBannerDismiss) btnBannerDismiss.addEventListener("click", hideInstallBanner);
  const toolInstall = $("t-install");
  if (toolInstall) toolInstall.addEventListener("click", () => { closeSheet(); showInstallGuide(); });
  const iosClose = $("ios-install-close");
  if (iosClose) iosClose.addEventListener("click", () => $("sheet-ios-install").classList.add("hidden"));

  // ------------------------------------------------------------ QR Code Scanner & Manual Pairing
  let barcodeDetector = null;
  if ("BarcodeDetector" in window) {
    try {
      barcodeDetector = new BarcodeDetector({ formats: ["qr_code"] });
    } catch (e) {
      console.warn("BarcodeDetector init failed", e);
    }
  }

  let scannerActive = false;
  let scannerStream = null;
  let scanAnimFrame = null;
  let currentCameraFacing = "environment";
  let torchActive = false;

  async function startScanner() {
    terminal = false;
    retries = 0;
    if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
      toast("Camera API not supported in this browser");
      return;
    }

    const modal = $("scanner-modal");
    const video = $("scanner-video");
    const feedback = $("scanner-feedback");
    const torchBtn = $("btn-scanner-torch");
    const flipBtn = $("btn-scanner-flip");

    if (modal) modal.classList.remove("hidden");
    scannerActive = true;
    if (feedback) feedback.textContent = "Starting camera…";

    try {
      const constraints = {
        video: {
          facingMode: { ideal: currentCameraFacing },
          width: { ideal: 1280 },
          height: { ideal: 720 },
        },
        audio: false,
      };
      const stream = await navigator.mediaDevices.getUserMedia(constraints);
      scannerStream = stream;
      if (video) {
        video.srcObject = stream;
        await video.play();
      }

      // Check device count for flip button
      try {
        const devices = await navigator.mediaDevices.enumerateDevices();
        const videoInputs = devices.filter((d) => d.kind === "videoinput");
        if (flipBtn) flipBtn.classList.toggle("hidden", videoInputs.length < 2);
      } catch {}

      // Check torch capability
      const track = stream.getVideoTracks()[0];
      const capabilities = track && track.getCapabilities ? track.getCapabilities() : {};
      if (torchBtn) {
        torchBtn.classList.toggle("hidden", !capabilities.torch);
        torchBtn.classList.remove("active");
        torchActive = false;
      }

      if (feedback) {
        if (!barcodeDetector) {
          feedback.textContent = "Live detector unsupported: use Upload or paste link below.";
        } else {
          feedback.textContent = "Point camera at QR code on laptop screen";
        }
      }
      runScanLoop();
    } catch (err) {
      console.error("Camera access error:", err);
      if (feedback) {
        if (err.name === "NotAllowedError" || err.name === "PermissionDeniedError") {
          feedback.textContent = "Camera access denied. Please allow camera in settings or paste link.";
        } else {
          feedback.textContent = "Camera error: " + (err.message || err.name);
        }
      }
    }
  }

  function stopScannerStream() {
    if (scannerStream) {
      scannerStream.getTracks().forEach((track) => track.stop());
      scannerStream = null;
    }
    const video = $("scanner-video");
    if (video) video.srcObject = null;
  }

  function stopScanner() {
    scannerActive = false;
    if (scanAnimFrame) {
      cancelAnimationFrame(scanAnimFrame);
      scanAnimFrame = null;
    }
    stopScannerStream();
    const modal = $("scanner-modal");
    if (modal) modal.classList.add("hidden");
  }

  async function toggleTorch() {
    if (!scannerStream) return;
    const track = scannerStream.getVideoTracks()[0];
    if (!track || !track.applyConstraints) return;
    try {
      torchActive = !torchActive;
      await track.applyConstraints({ advanced: [{ torch: torchActive }] });
      const torchBtn = $("btn-scanner-torch");
      if (torchBtn) torchBtn.classList.toggle("active", torchActive);
    } catch (e) {
      console.warn("Torch failed", e);
    }
  }

  async function flipCamera() {
    currentCameraFacing = currentCameraFacing === "environment" ? "user" : "environment";
    stopScannerStream();
    await startScanner();
  }

  async function runScanLoop() {
    if (!scannerActive) return;
    const video = $("scanner-video");

    if (video && video.readyState >= HTMLMediaElement.HAVE_CURRENT_DATA && barcodeDetector) {
      try {
        const barcodes = await barcodeDetector.detect(video);
        if (barcodes && barcodes.length > 0) {
          for (const item of barcodes) {
            if (await handleScannedCode(item.rawValue)) {
              return; // Matched and handled!
            }
          }
        }
      } catch (e) {
        // Video frame might be empty or transitioning
      }
    }

    if (scannerActive) {
      scanAnimFrame = requestAnimationFrame(runScanLoop);
    }
  }

  async function handleScannedCode(rawValue) {
    if (!rawValue) return false;
    rawValue = rawValue.trim();

    let hashPart = "";
    let targetHost = null;
    let targetProto = null;

    try {
      if (rawValue.startsWith("http://") || rawValue.startsWith("https://")) {
        const u = new URL(rawValue);
        hashPart = u.hash;
        targetHost = u.host;
        targetProto = u.protocol;
      } else if (rawValue.includes("#")) {
        hashPart = rawValue.substring(rawValue.indexOf("#"));
      } else {
        hashPart = "#" + rawValue;
      }
    } catch (e) {
      console.warn("URL parse failed", e);
    }

    const parsed = C.parseFragment(hashPart);
    if (!parsed) return false;

    buzz();
    stopScanner();

    // Reset terminal state and retries for new pairing
    terminal = false;
    retries = 0;

    // Reset session storage with fresh credentials
    ss.clear();
    if (parsed.host) {
      targetHost = parsed.host;
      targetProto = (targetHost.startsWith("127.0.0.1") || targetHost.startsWith("localhost") || (targetHost.includes(":") && !targetHost.endsWith(":443"))) ? "http:" : "https:";
    }
    if (targetHost) {
      ss.set("server_host", targetHost);
      ss.set("server_proto", targetProto || "https:");
      prefs.set("last_host", targetHost);
    }
    ss.set("key", parsed.key);
    ss.set("token", parsed.token);
    ss.set("cseq", 0);
    ss.set("hseq", 0);

    try {
      key = await C.importKey(parsed.key);
    } catch (e) {
      message("Encryption Error", "Browser failed to import key: " + e.message);
      return true;
    }

    clearPin();
    show("screen-pin");
    toast("QR code scanned! Enter the 4-digit PIN shown on your screen.", 3000);
    return true;
  }

  async function handleFileInput(e) {
    const file = e.target.files && e.target.files[0];
    if (!file) return;
    const img = new Image();
    img.onload = async () => {
      URL.revokeObjectURL(img.src);
      if (barcodeDetector) {
        try {
          const barcodes = await barcodeDetector.detect(img);
          if (barcodes && barcodes.length > 0) {
            for (const item of barcodes) {
              if (await handleScannedCode(item.rawValue)) return;
            }
          }
        } catch (err) {
          console.warn("BarcodeDetector image error:", err);
        }
      }
      toast("No QR code detected in this photo");
    };
    img.src = URL.createObjectURL(file);
    e.target.value = "";
  }

  function toggleManualLink() {
    const container = $("manual-link-container");
    if (!container) return;
    const isHidden = container.classList.toggle("hidden");
    if (!isHidden) {
      const input = $("manual-link-input");
      if (input) input.focus();
    }
  }

  async function submitManualLink() {
    const input = $("manual-link-input");
    if (!input) return;
    const val = input.value.trim();
    if (!val) return;
    const ok = await handleScannedCode(val);
    if (!ok) {
      toast("Invalid pairing link or token");
    }
  }

  const btnScanQr = $("btn-scan-qr");
  if (btnScanQr) btnScanQr.addEventListener("click", startScanner);
  const btnManualToggle = $("btn-manual-toggle");
  if (btnManualToggle) btnManualToggle.addEventListener("click", toggleManualLink);
  const btnManualSubmit = $("btn-manual-submit");
  if (btnManualSubmit) btnManualSubmit.addEventListener("click", submitManualLink);
  const manualInput = $("manual-link-input");
  if (manualInput) {
    manualInput.addEventListener("keydown", (e) => {
      if (e.key === "Enter") submitManualLink();
    });
  }

  const btnCloseScanner = $("btn-close-scanner");
  if (btnCloseScanner) btnCloseScanner.addEventListener("click", stopScanner);
  const btnScannerTorch = $("btn-scanner-torch");
  if (btnScannerTorch) btnScannerTorch.addEventListener("click", toggleTorch);
  const btnScannerFlip = $("btn-scanner-flip");
  if (btnScannerFlip) btnScannerFlip.addEventListener("click", flipCamera);
  const fileInput = $("scanner-file-input");
  if (fileInput) fileInput.addEventListener("change", handleFileInput);

  const toolScan = $("t-scan");
  if (toolScan) toolScan.addEventListener("click", () => { closeSheet(); startScanner(); });

  // ------------------------------------------------------------ PIN Keypad listeners
  document.querySelectorAll(".keypad .key-btn[data-key]").forEach((b) => {
    b.addEventListener("click", () => enterPinDigit(b.dataset.key));
  });
  const clearBtn = $("pin-clear");
  if (clearBtn) clearBtn.addEventListener("click", clearPin);
  const backBtn = $("pin-back");
  if (backBtn) backBtn.addEventListener("click", backspacePin);

  // ------------------------------------------------------------ boot
  async function boot() {
    applyPrefs();
    matchMedia("(prefers-color-scheme: dark)").addEventListener("change", applyPrefs);
    const frag = C.parseFragment(location.hash);
    // Sanitise immediately: secrets must not stay in the address bar or history.
    if (location.hash) history.replaceState(null, "", location.pathname);
    if (frag) {
      terminal = false;
      retries = 0;
      ss.clear();
      if (frag.host) {
        ss.set("server_host", frag.host);
        ss.set("server_proto", (frag.host.startsWith("127.0.0.1") || frag.host.startsWith("localhost") || (frag.host.includes(":") && !frag.host.endsWith(":443"))) ? "http:" : "https:");
        prefs.set("last_host", frag.host);
      }
      ss.set("key", frag.key);
      ss.set("token", frag.token);
      ss.set("cseq", 0);
      ss.set("hseq", 0);
    }
    const k = ss.get("key");
    if (!k || (!ss.get("token") && !ss.get("sid"))) {
      const lastHost = prefs.get("last_host", null);
      const badge = lastHost ? { type: "ready", text: "Ready to connect (" + lastHost.split(":")[0] + ")" } : null;
      return message("Connect to Presentation", "Open DeckTap on your computer and scan the QR code shown on your screen to start presenting.", false, badge);
    }
    try {
      key = await C.importKey(k);
    } catch (e) {
      return message("Browser not supported", "This browser can't do the required encryption (" + e.message + ").");
    }
    if (!ss.get("sid")) {
      show("screen-pin");
      return;
    }
    show("screen-connecting");
    connect();
  }
  boot();
})();
