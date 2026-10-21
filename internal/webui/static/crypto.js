/* DeckTap E2EE (SPEC-SEC-001) – WebCrypto AES-256-GCM, shared by the app and the tests. */
(function (root) {
  "use strict";
  const enc = new TextEncoder(), dec = new TextDecoder();
  const AAD_C2H = enc.encode("slidecue/v1 c2h");
  const AAD_H2C = enc.encode("slidecue/v1 h2c");

  function b64uEncode(bytes) {
    let bin = "";
    for (let i = 0; i < bytes.length; i++) bin += String.fromCharCode(bytes[i]);
    return btoa(bin).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
  }
  function b64uDecode(s) {
    if (!/^[A-Za-z0-9_-]*$/.test(s)) throw new Error("bad base64url");
    s = s.replace(/-/g, "+").replace(/_/g, "/");
    while (s.length % 4) s += "=";
    const bin = atob(s), out = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
    return out;
  }
  function randomHex(n) {
    return Array.from(crypto.getRandomValues(new Uint8Array(n)), (b) => b.toString(16).padStart(2, "0")).join("");
  }

  /** Parse "#<PairingToken>.<MasterKey>" or "#<PairingToken>.<MasterKey>@<Host>" → {token, key, host} or null. */
  function parseFragment(hash) {
    if (!hash) return null;
    hash = String(hash).trim();
    if (hash.startsWith("#")) hash = hash.slice(1);
    if (!hash) return null;

    // Format A: <token>.<key>@<host> or <token>.<key>
    const m = /^([A-Za-z0-9_-]{22})\.([A-Za-z0-9_-]{43})(?:@([A-Za-z0-9_.:-]+))?$/.exec(hash);
    if (m) {
      try {
        if (b64uDecode(m[1]).length === 16 && b64uDecode(m[2]).length === 32) {
          return { token: m[1], key: m[2], host: m[3] || null };
        }
      } catch (e) {
        return null;
      }
    }

    // Format B: query-style fragment host=...&token=...&key=...
    if (hash.includes("=") && (hash.includes("token=") || hash.includes("key="))) {
      try {
        const params = new URLSearchParams(hash);
        const token = params.get("token");
        const key = params.get("key");
        const host = params.get("host");
        if (token && key && /^[A-Za-z0-9_-]{22}$/.test(token) && /^[A-Za-z0-9_-]{43}$/.test(key)) {
          if (b64uDecode(token).length === 16 && b64uDecode(key).length === 32) {
            return { token, key, host: host || null };
          }
        }
      } catch (e) {
        return null;
      }
    }

    return null;
  }

  async function importKey(keyB64) {
    if (!root.crypto || !root.crypto.subtle) throw new Error("WebCrypto unavailable (needs HTTPS)");
    return crypto.subtle.importKey("raw", b64uDecode(keyB64), { name: "AES-GCM" }, false, ["encrypt", "decrypt"]);
  }

  /** Encrypt obj → Option A JSON envelope string. */
  async function seal(key, obj) {
    const iv = crypto.getRandomValues(new Uint8Array(12));
    const ct = await crypto.subtle.encrypt({ name: "AES-GCM", iv, additionalData: AAD_C2H, tagLength: 128 },
      key, enc.encode(JSON.stringify(obj)));
    return JSON.stringify({ iv: b64uEncode(iv), ct: b64uEncode(new Uint8Array(ct)) });
  }

  /** Decrypt an Option A envelope (string) or Option B frame (ArrayBuffer) from the host. */
  async function open(key, frame) {
    let iv, ct;
    if (typeof frame === "string") {
      const env = JSON.parse(frame);
      iv = b64uDecode(env.iv); ct = b64uDecode(env.ct);
    } else {
      const b = new Uint8Array(frame);
      iv = b.slice(0, 12); ct = b.slice(12);
    }
    if (iv.length !== 12) throw new Error("bad iv");
    const pt = await crypto.subtle.decrypt({ name: "AES-GCM", iv, additionalData: AAD_H2C, tagLength: 128 }, key, ct);
    return JSON.parse(dec.decode(pt));
  }

  root.DeckTapCrypto = root.SlideCueCrypto = { b64uEncode, b64uDecode, randomHex, parseFragment, importKey, seal, open, AAD_C2H, AAD_H2C };
})(typeof window !== "undefined" ? window : globalThis);
