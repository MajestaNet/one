/**
 * Persistent session file helpers (CIDE-10).
 *
 * The Majesta One JWT lives in this file. Prefer OS `safeStorage` (keyring / DPAPI / Keychain).
 * When that is unavailable (common on Linux without a secret service), encrypt with AES-256-GCM
 * using a per-install key at mode 0600 so a restart still restores the session — same-user
 * processes can read that key, which matches other desktop apps' Linux fallback. Never write
 * the JWT as UTF-8 JSON.
 */

import { createCipheriv, createDecipheriv, randomBytes } from "node:crypto";
import fs from "node:fs";
import path from "node:path";

export type SessionCrypto = {
  isEncryptionAvailable: () => boolean;
  encryptString: (plain: string) => Buffer;
  decryptString: (cipher: Buffer) => string;
};

export const SESSION_FILE_MODE = 0o600;

/** Magic prefix for AES-256-GCM local fallback blobs (`LCSE01`). */
export const LOCAL_SESSION_MAGIC = Buffer.from("LCSE01");

const AES_ALGO = "aes-256-gcm";
const IV_LEN = 12;
const KEY_LEN = 32;
const TAG_LEN = 16;
const LOCAL_HEADER_LEN = LOCAL_SESSION_MAGIC.length + IV_LEN + TAG_LEN;

export type SessionPersistBackend = "os" | "local";

export type WriteSessionResult =
  | { ok: true; backend?: SessionPersistBackend; ephemeral?: boolean }
  | { ok: false; error: string };

export type SessionPersistOptions = {
  /** When OS crypto is unavailable, AES-GCM with this 32-byte key file (mode 0600). */
  localKeyPath?: string;
};

function looksLikeJsonObject(text: string): boolean {
  const trimmed = text.trimStart();
  if (!trimmed.startsWith("{")) return false;
  try {
    JSON.parse(trimmed);
    return true;
  } catch {
    return false;
  }
}

function chmodQuiet(filePath: string): void {
  try {
    fs.chmodSync(filePath, SESSION_FILE_MODE);
  } catch {
    /* chmod can fail on some filesystems; mode on writeFileSync is the primary control */
  }
}

function writeAtomic(filePath: string, buf: Buffer): void {
  fs.mkdirSync(path.dirname(filePath), { recursive: true });
  fs.writeFileSync(filePath, buf, { mode: SESSION_FILE_MODE });
  chmodQuiet(filePath);
}

export function isLocalEncryptedSession(buf: Buffer): boolean {
  return buf.length >= LOCAL_HEADER_LEN && buf.subarray(0, LOCAL_SESSION_MAGIC.length).equals(LOCAL_SESSION_MAGIC);
}

/** Load or create the 32-byte AES key used when OS secure storage is unavailable. */
export function loadOrCreateLocalKey(keyPath: string): Buffer {
  if (fs.existsSync(keyPath)) {
    const existing = fs.readFileSync(keyPath);
    if (existing.length === KEY_LEN) return existing;
  }
  const key = randomBytes(KEY_LEN);
  writeAtomic(keyPath, key);
  return key;
}

export function encryptLocalSession(plain: string, key: Buffer): Buffer {
  const iv = randomBytes(IV_LEN);
  const cipher = createCipheriv(AES_ALGO, key, iv);
  const ciphertext = Buffer.concat([cipher.update(plain, "utf8"), cipher.final()]);
  const tag = cipher.getAuthTag();
  return Buffer.concat([LOCAL_SESSION_MAGIC, iv, tag, ciphertext]);
}

export function decryptLocalSession(buf: Buffer, key: Buffer): string {
  if (!isLocalEncryptedSession(buf)) {
    throw new Error("not a local-encrypted session blob");
  }
  const ivStart = LOCAL_SESSION_MAGIC.length;
  const tagStart = ivStart + IV_LEN;
  const dataStart = tagStart + TAG_LEN;
  const iv = buf.subarray(ivStart, tagStart);
  const tag = buf.subarray(tagStart, dataStart);
  const ciphertext = buf.subarray(dataStart);
  const decipher = createDecipheriv(AES_ALGO, key, iv);
  decipher.setAuthTag(tag);
  return Buffer.concat([decipher.update(ciphertext), decipher.final()]).toString("utf8");
}

function persistEncrypted(
  filePath: string,
  data: string,
  crypto: SessionCrypto,
  localKeyPath: string | undefined,
): WriteSessionResult {
  if (crypto.isEncryptionAvailable()) {
    writeAtomic(filePath, crypto.encryptString(data));
    return { ok: true, backend: "os" };
  }
  if (!localKeyPath) {
    return {
      ok: false,
      error:
        "OS secure storage is unavailable — refusing to store the Majesta One token in cleartext. Unlock your keyring, or reconnect each session.",
    };
  }
  try {
    const key = loadOrCreateLocalKey(localKeyPath);
    writeAtomic(filePath, encryptLocalSession(data, key));
    return { ok: true, backend: "local" };
  } catch (err) {
    return {
      ok: false,
      error: err instanceof Error ? err.message : "Failed to persist local-encrypted session",
    };
  }
}

/** Persist `data` encrypted, or delete the file when `data` is null. */
export function writeSessionFile(
  filePath: string,
  data: string | null,
  crypto: SessionCrypto,
  opts?: SessionPersistOptions,
): WriteSessionResult {
  if (data === null) {
    if (fs.existsSync(filePath)) fs.unlinkSync(filePath);
    return { ok: true };
  }
  return persistEncrypted(filePath, data, crypto, opts?.localKeyPath);
}

/**
 * Load and decrypt a session file. Returns null when missing, undecryptable, or when
 * no decryptor can handle the blob.
 *
 * Prefers OS `safeStorage`. Local AES-GCM blobs (LCSE01) decrypt with `localKeyPath`.
 * Legacy plaintext JSON is migrated in place to the best available backend.
 */
export function readSessionFile(
  filePath: string,
  crypto: SessionCrypto,
  opts?: SessionPersistOptions,
): string | null {
  if (!fs.existsSync(filePath)) return null;
  const buf = fs.readFileSync(filePath);

  if (isLocalEncryptedSession(buf)) {
    if (!opts?.localKeyPath || !fs.existsSync(opts.localKeyPath)) return null;
    try {
      const plain = decryptLocalSession(buf, fs.readFileSync(opts.localKeyPath));
      if (crypto.isEncryptionAvailable()) {
        const migrated = persistEncrypted(filePath, plain, crypto, opts.localKeyPath);
        if (migrated.ok && migrated.backend === "os") return plain;
      }
      return plain;
    } catch {
      return null;
    }
  }

  if (crypto.isEncryptionAvailable()) {
    try {
      return crypto.decryptString(buf);
    } catch {
      // Pre-hardening builds wrote UTF-8 JSON when safeStorage was unavailable.
      const asText = buf.toString("utf8");
      if (!looksLikeJsonObject(asText)) return null;
      persistEncrypted(filePath, asText, crypto, opts?.localKeyPath);
      return asText;
    }
  }

  const asText = buf.toString("utf8");
  if (looksLikeJsonObject(asText) && opts?.localKeyPath) {
    persistEncrypted(filePath, asText, crypto, opts.localKeyPath);
    return asText;
  }
  return null;
}
