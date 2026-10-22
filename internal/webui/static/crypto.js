/* DeckTap E2EE (SPEC-SEC-001) – WebCrypto AES-256-GCM with pure-JS fallback for Local LAN mode. */
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

  function getRandomBytes(n) {
    const b = new Uint8Array(n);
    if (root.crypto && root.crypto.getRandomValues) {
      root.crypto.getRandomValues(b);
    } else {
      for (let i = 0; i < n; i++) b[i] = Math.floor(Math.random() * 256);
    }
    return b;
  }

  function randomHex(n) {
    return Array.from(getRandomBytes(n), (b) => b.toString(16).padStart(2, "0")).join("");
  }

  // ------------------------------------------------------------ Pure-JS AES-256-GCM Engine
  // Used as a transparent zero-warning fallback when running over local LAN HTTP (where SubtleCrypto is disabled)
  const SBOX = new Uint8Array(256);
  const RCON = new Uint32Array(15);
  const xtime = new Uint8Array(256);

  (function initTables() {
    const p = new Uint8Array(256), inv = new Uint8Array(256);
    let x = 1;
    for (let i = 0; i < 256; i++) {
      p[i] = x;
      inv[x] = i;
      x ^= (x << 1) ^ ((x >> 7) * 0x11b);
    }
    SBOX[0] = 0x63;
    for (let i = 1; i < 256; i++) {
      let y = p[255 - inv[i]];
      y = y ^ ((y << 1) | (y >> 7)) ^ ((y << 2) | (y >> 6)) ^ ((y << 3) | (y >> 5)) ^ ((y << 4) | (y >> 4)) ^ 0x63;
      SBOX[i] = y & 0xff;
    }
    let r = 1;
    for (let i = 1; i < 15; i++) {
      RCON[i] = (r << 24);
      r = (r << 1) ^ ((r >> 7) * 0x11b);
    }
    for (let i = 0; i < 256; i++) {
      xtime[i] = ((i << 1) ^ ((i >> 7) * 0x11b)) & 0xff;
    }
  })();

  function subWord(w) {
    return (SBOX[(w >>> 24) & 0xff] << 24) |
           (SBOX[(w >>> 16) & 0xff] << 16) |
           (SBOX[(w >>> 8) & 0xff] << 8) |
           SBOX[w & 0xff];
  }

  function rotWord(w) {
    return ((w << 8) | (w >>> 24)) >>> 0;
  }

  function keyExpansion(keyBytes) {
    const w = new Uint32Array(60);
    for (let i = 0; i < 8; i++) {
      w[i] = (keyBytes[4 * i] << 24) | (keyBytes[4 * i + 1] << 16) | (keyBytes[4 * i + 2] << 8) | keyBytes[4 * i + 3];
    }
    for (let i = 8; i < 60; i++) {
      let temp = w[i - 1];
      if (i % 8 === 0) {
        temp = subWord(rotWord(temp)) ^ RCON[i / 8];
      } else if (i % 8 === 4) {
        temp = subWord(temp);
      }
      w[i] = (w[i - 8] ^ temp) >>> 0;
    }
    return w;
  }

  function cipher(w, inBytes, outBytes) {
    let s0 = (inBytes[0] << 24) | (inBytes[1] << 16) | (inBytes[2] << 8) | inBytes[3];
    let s1 = (inBytes[4] << 24) | (inBytes[5] << 16) | (inBytes[6] << 8) | inBytes[7];
    let s2 = (inBytes[8] << 24) | (inBytes[9] << 16) | (inBytes[10] << 8) | inBytes[11];
    let s3 = (inBytes[12] << 24) | (inBytes[13] << 16) | (inBytes[14] << 8) | inBytes[15];

    s0 ^= w[0]; s1 ^= w[1]; s2 ^= w[2]; s3 ^= w[3];

    for (let round = 1; round < 14; round++) {
      const idx = round * 4;
      const t0 = (SBOX[(s0 >>> 24) & 0xff] << 24) | (SBOX[(s1 >>> 16) & 0xff] << 16) | (SBOX[(s2 >>> 8) & 0xff] << 8) | SBOX[s3 & 0xff];
      const t1 = (SBOX[(s1 >>> 24) & 0xff] << 24) | (SBOX[(s2 >>> 16) & 0xff] << 16) | (SBOX[(s3 >>> 8) & 0xff] << 8) | SBOX[s0 & 0xff];
      const t2 = (SBOX[(s2 >>> 24) & 0xff] << 24) | (SBOX[(s3 >>> 16) & 0xff] << 16) | (SBOX[(s0 >>> 8) & 0xff] << 8) | SBOX[s1 & 0xff];
      const t3 = (SBOX[(s3 >>> 24) & 0xff] << 24) | (SBOX[(s0 >>> 16) & 0xff] << 16) | (SBOX[(s1 >>> 8) & 0xff] << 8) | SBOX[s2 & 0xff];

      const mix = (t) => {
        const a0 = (t >>> 24) & 0xff, a1 = (t >>> 16) & 0xff, a2 = (t >>> 8) & 0xff, a3 = t & 0xff;
        return ((xtime[a0] ^ xtime[a1] ^ a1 ^ a2 ^ a3) << 24) |
               ((xtime[a1] ^ xtime[a2] ^ a2 ^ a3 ^ a0) << 16) |
               ((xtime[a2] ^ xtime[a3] ^ a3 ^ a0 ^ a1) << 8) |
               (xtime[a3] ^ xtime[a0] ^ a0 ^ a1 ^ a2);
      };

      s0 = (mix(t0) ^ w[idx]) >>> 0;
      s1 = (mix(t1) ^ w[idx + 1]) >>> 0;
      s2 = (mix(t2) ^ w[idx + 2]) >>> 0;
      s3 = (mix(t3) ^ w[idx + 3]) >>> 0;
    }

    const idx = 14 * 4;
    const t0 = (SBOX[(s0 >>> 24) & 0xff] << 24) | (SBOX[(s1 >>> 16) & 0xff] << 16) | (SBOX[(s2 >>> 8) & 0xff] << 8) | SBOX[s3 & 0xff];
    const t1 = (SBOX[(s1 >>> 24) & 0xff] << 24) | (SBOX[(s2 >>> 16) & 0xff] << 16) | (SBOX[(s3 >>> 8) & 0xff] << 8) | SBOX[s0 & 0xff];
    const t2 = (SBOX[(s2 >>> 24) & 0xff] << 24) | (SBOX[(s3 >>> 16) & 0xff] << 16) | (SBOX[(s0 >>> 8) & 0xff] << 8) | SBOX[s1 & 0xff];
    const t3 = (SBOX[(s3 >>> 24) & 0xff] << 24) | (SBOX[(s0 >>> 16) & 0xff] << 16) | (SBOX[(s1 >>> 8) & 0xff] << 8) | SBOX[s2 & 0xff];

    s0 = (t0 ^ w[idx]) >>> 0;
    s1 = (t1 ^ w[idx + 1]) >>> 0;
    s2 = (t2 ^ w[idx + 2]) >>> 0;
    s3 = (t3 ^ w[idx + 3]) >>> 0;

    outBytes[0] = (s0 >>> 24) & 0xff; outBytes[1] = (s0 >>> 16) & 0xff; outBytes[2] = (s0 >>> 8) & 0xff; outBytes[3] = s0 & 0xff;
    outBytes[4] = (s1 >>> 24) & 0xff; outBytes[5] = (s1 >>> 16) & 0xff; outBytes[6] = (s1 >>> 8) & 0xff; outBytes[7] = s1 & 0xff;
    outBytes[8] = (s2 >>> 24) & 0xff; outBytes[9] = (s2 >>> 16) & 0xff; outBytes[10] = (s2 >>> 8) & 0xff; outBytes[11] = s2 & 0xff;
    outBytes[12] = (s3 >>> 24) & 0xff; outBytes[13] = (s3 >>> 16) & 0xff; outBytes[14] = (s3 >>> 8) & 0xff; outBytes[15] = s3 & 0xff;
  }

  function ghashMult(x, y) {
    const z = new Uint32Array(4);
    const v = new Uint32Array(y);
    for (let i = 0; i < 128; i++) {
      const bit = (x[Math.floor(i / 32)] >>> (31 - (i % 32))) & 1;
      if (bit) {
        z[0] ^= v[0]; z[1] ^= v[1]; z[2] ^= v[2]; z[3] ^= v[3];
      }
      const lsb = v[3] & 1;
      v[3] = (v[3] >>> 1) | ((v[2] & 1) << 31);
      v[2] = (v[2] >>> 1) | ((v[1] & 1) << 31);
      v[1] = (v[1] >>> 1) | ((v[0] & 1) << 31);
      v[0] = v[0] >>> 1;
      if (lsb) {
        v[0] ^= 0xe1000000;
      }
    }
    x[0] = z[0]; x[1] = z[1]; x[2] = z[2]; x[3] = z[3];
  }

  function ghash(hWords, aad, ct) {
    const x = new Uint32Array(4);
    function update(data) {
      const len = data.length;
      for (let i = 0; i < len; i += 16) {
        const block = new Uint8Array(16);
        const rem = Math.min(16, len - i);
        block.set(data.subarray(i, i + rem));
        x[0] ^= (block[0] << 24) | (block[1] << 16) | (block[2] << 8) | block[3];
        x[1] ^= (block[4] << 24) | (block[5] << 16) | (block[6] << 8) | block[7];
        x[2] ^= (block[8] << 24) | (block[9] << 16) | (block[10] << 8) | block[11];
        x[3] ^= (block[12] << 24) | (block[13] << 16) | (block[14] << 8) | block[15];
        ghashMult(x, hWords);
      }
    }
    if (aad && aad.length > 0) update(aad);
    if (ct && ct.length > 0) update(ct);

    const aadBits = BigInt((aad ? aad.length : 0) * 8);
    const ctBits = BigInt((ct ? ct.length : 0) * 8);
    x[0] ^= Number((aadBits >> 32n) & 0xffffffffn);
    x[1] ^= Number(aadBits & 0xffffffffn);
    x[2] ^= Number((ctBits >> 32n) & 0xffffffffn);
    x[3] ^= Number(ctBits & 0xffffffffn);
    ghashMult(x, hWords);

    const out = new Uint8Array(16);
    for (let i = 0; i < 4; i++) {
      out[i * 4] = (x[i] >>> 24) & 0xff;
      out[i * 4 + 1] = (x[i] >>> 16) & 0xff;
      out[i * 4 + 2] = (x[i] >>> 8) & 0xff;
      out[i * 4 + 3] = x[i] & 0xff;
    }
    return out;
  }

  function pureEncryptGCM(keyBytes, ivBytes, ptBytes, aadBytes) {
    const w = keyExpansion(keyBytes);
    const zero = new Uint8Array(16);
    const hBytes = new Uint8Array(16);
    cipher(w, zero, hBytes);
    const hWords = new Uint32Array(4);
    for (let i = 0; i < 4; i++) {
      hWords[i] = (hBytes[i * 4] << 24) | (hBytes[i * 4 + 1] << 16) | (hBytes[i * 4 + 2] << 8) | hBytes[i * 4 + 3];
    }

    const j0 = new Uint8Array(16);
    j0.set(ivBytes.subarray(0, 12));
    j0[15] = 1;

    const tagMask = new Uint8Array(16);
    cipher(w, j0, tagMask);

    let counter = 2;
    const ct = new Uint8Array(ptBytes.length);
    const cb = new Uint8Array(16);
    cb.set(ivBytes.subarray(0, 12));
    const block = new Uint8Array(16);

    for (let i = 0; i < ptBytes.length; i += 16) {
      cb[12] = (counter >>> 24) & 0xff;
      cb[13] = (counter >>> 16) & 0xff;
      cb[14] = (counter >>> 8) & 0xff;
      cb[15] = counter & 0xff;
      counter++;
      cipher(w, cb, block);
      const rem = Math.min(16, ptBytes.length - i);
      for (let j = 0; j < rem; j++) {
        ct[i + j] = ptBytes[i + j] ^ block[j];
      }
    }

    const g = ghash(hWords, aadBytes, ct);
    const tag = new Uint8Array(16);
    for (let i = 0; i < 16; i++) tag[i] = g[i] ^ tagMask[i];

    const result = new Uint8Array(ct.length + 16);
    result.set(ct, 0);
    result.set(tag, ct.length);
    return result;
  }

  function pureDecryptGCM(keyBytes, ivBytes, ctAndTagBytes, aadBytes) {
    if (ctAndTagBytes.length < 16) throw new Error("GCM tag too short");
    const ctLen = ctAndTagBytes.length - 16;
    const ct = ctAndTagBytes.subarray(0, ctLen);
    const tag = ctAndTagBytes.subarray(ctLen);

    const w = keyExpansion(keyBytes);
    const zero = new Uint8Array(16);
    const hBytes = new Uint8Array(16);
    cipher(w, zero, hBytes);
    const hWords = new Uint32Array(4);
    for (let i = 0; i < 4; i++) {
      hWords[i] = (hBytes[i * 4] << 24) | (hBytes[i * 4 + 1] << 16) | (hBytes[i * 4 + 2] << 8) | hBytes[i * 4 + 3];
    }

    const j0 = new Uint8Array(16);
    j0.set(ivBytes.subarray(0, 12));
    j0[15] = 1;

    const tagMask = new Uint8Array(16);
    cipher(w, j0, tagMask);

    const g = ghash(hWords, aadBytes, ct);
    let diff = 0;
    for (let i = 0; i < 16; i++) {
      diff |= (g[i] ^ tagMask[i]) ^ tag[i];
    }
    if (diff !== 0) throw new Error("GCM authentication tag mismatch");

    let counter = 2;
    const pt = new Uint8Array(ctLen);
    const cb = new Uint8Array(16);
    cb.set(ivBytes.subarray(0, 12));
    const block = new Uint8Array(16);

    for (let i = 0; i < ctLen; i += 16) {
      cb[12] = (counter >>> 24) & 0xff;
      cb[13] = (counter >>> 16) & 0xff;
      cb[14] = (counter >>> 8) & 0xff;
      cb[15] = counter & 0xff;
      counter++;
      cipher(w, cb, block);
      const rem = Math.min(16, ctLen - i);
      for (let j = 0; j < rem; j++) {
        pt[i + j] = ct[i + j] ^ block[j];
      }
    }
    return pt;
  }

  // ------------------------------------------------------------ DeckTap Crypto Interface
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
    const raw = b64uDecode(keyB64);
    let subtleKey = null;
    if (root.crypto && root.crypto.subtle) {
      try {
        subtleKey = await root.crypto.subtle.importKey("raw", raw, { name: "AES-GCM" }, false, ["encrypt", "decrypt"]);
      } catch (e) {
        subtleKey = null;
      }
    }
    return { raw, subtleKey };
  }

  /** Encrypt obj → Option A JSON envelope string. */
  async function seal(keyObj, obj) {
    const iv = getRandomBytes(12);
    const plaintext = enc.encode(JSON.stringify(obj));
    let ctAndTag;
    if (keyObj && keyObj.subtleKey && root.crypto && root.crypto.subtle) {
      const ctBuf = await root.crypto.subtle.encrypt(
        { name: "AES-GCM", iv, additionalData: AAD_C2H, tagLength: 128 },
        keyObj.subtleKey,
        plaintext
      );
      ctAndTag = new Uint8Array(ctBuf);
    } else {
      const rawKey = keyObj.raw || keyObj;
      ctAndTag = pureEncryptGCM(rawKey, iv, plaintext, AAD_C2H);
    }
    return JSON.stringify({ iv: b64uEncode(iv), ct: b64uEncode(ctAndTag) });
  }

  /** Decrypt an Option A envelope (string) or Option B frame (ArrayBuffer) from the host. */
  async function open(keyObj, frame) {
    let iv, ctAndTag;
    if (typeof frame === "string") {
      const env = JSON.parse(frame);
      iv = b64uDecode(env.iv);
      ctAndTag = b64uDecode(env.ct);
    } else {
      const b = new Uint8Array(frame);
      iv = b.slice(0, 12);
      ctAndTag = b.slice(12);
    }
    if (iv.length !== 12) throw new Error("bad iv");

    let ptBytes;
    if (keyObj && keyObj.subtleKey && root.crypto && root.crypto.subtle) {
      const ptBuf = await root.crypto.subtle.decrypt(
        { name: "AES-GCM", iv, additionalData: AAD_H2C, tagLength: 128 },
        keyObj.subtleKey,
        ctAndTag
      );
      ptBytes = new Uint8Array(ptBuf);
    } else {
      const rawKey = keyObj.raw || keyObj;
      ptBytes = pureDecryptGCM(rawKey, iv, ctAndTag, AAD_H2C);
    }
    return JSON.parse(dec.decode(ptBytes));
  }

  root.DeckTapCrypto = root.SlideCueCrypto = {
    b64uEncode,
    b64uDecode,
    randomHex,
    parseFragment,
    importKey,
    seal,
    open,
    AAD_C2H,
    AAD_H2C,
    pureEncryptGCM,
    pureDecryptGCM,
  };
})(typeof window !== "undefined" ? window : globalThis);
