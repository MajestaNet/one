import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import {
  SESSION_FILE_MODE,
  decryptLocalSession,
  encryptLocalSession,
  isLocalEncryptedSession,
  loadOrCreateLocalKey,
  readSessionFile,
  writeSessionFile,
  type SessionCrypto,
} from "./sessionStore";

const cleanup: string[] = [];

afterEach(() => {
  for (const p of cleanup.splice(0)) fs.rmSync(p, { recursive: true, force: true });
});

function tmpDir(): string {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "control-ide-session-"));
  cleanup.push(dir);
  return dir;
}

function tmpFile(): string {
  return path.join(tmpDir(), "session.bin");
}

function cryptoStub(opts: { available: boolean; roundTrip?: boolean } = { available: true }): SessionCrypto {
  return {
    isEncryptionAvailable: () => opts.available,
    encryptString: (plain) => Buffer.from(`ENC:${plain}`, "utf8"),
    decryptString: (cipher) => {
      const s = cipher.toString("utf8");
      if (!s.startsWith("ENC:")) throw new Error("not encrypted");
      return s.slice(4);
    },
  };
}

describe("writeSessionFile", () => {
  it("refuses to write when encryption is unavailable and no local key path is given", () => {
    const file = tmpFile();
    const result = writeSessionFile(file, JSON.stringify({ token: "secret" }), cryptoStub({ available: false }));
    expect(result.ok).toBe(false);
    if (!result.ok) expect(result.error).toMatch(/secure storage is unavailable/i);
    expect(fs.existsSync(file)).toBe(false);
  });

  it("writes encrypted bytes with mode 0600", () => {
    const file = tmpFile();
    const result = writeSessionFile(file, JSON.stringify({ token: "secret" }), cryptoStub());
    expect(result).toEqual({ ok: true, backend: "os" });
    const stat = fs.statSync(file);
    expect(stat.mode & 0o777).toBe(SESSION_FILE_MODE);
    // Stub crypto prefixes rather than opaque-encrypts; real safeStorage output is opaque.
    expect(fs.readFileSync(file, "utf8")).toMatch(/^ENC:/);
  });

  it("deletes the file when data is null", () => {
    const file = tmpFile();
    writeSessionFile(file, "{}", cryptoStub());
    expect(writeSessionFile(file, null, cryptoStub())).toEqual({ ok: true });
    expect(fs.existsSync(file)).toBe(false);
  });

  it("persists with local AES-GCM when OS encryption is unavailable", () => {
    const dir = tmpDir();
    const file = path.join(dir, "session.bin");
    const keyPath = path.join(dir, "session.key");
    const payload = JSON.stringify({ token: "secret" });
    const result = writeSessionFile(file, payload, cryptoStub({ available: false }), { localKeyPath: keyPath });
    expect(result).toEqual({ ok: true, backend: "local" });
    expect(fs.existsSync(file)).toBe(true);
    expect(fs.existsSync(keyPath)).toBe(true);
    expect(fs.statSync(file).mode & 0o777).toBe(SESSION_FILE_MODE);
    expect(fs.statSync(keyPath).mode & 0o777).toBe(SESSION_FILE_MODE);
    const buf = fs.readFileSync(file);
    expect(isLocalEncryptedSession(buf)).toBe(true);
    expect(buf.toString("utf8")).not.toContain("secret");
    expect(readSessionFile(file, cryptoStub({ available: false }), { localKeyPath: keyPath })).toBe(payload);
  });
});

describe("readSessionFile", () => {
  it("returns null when encryption is unavailable and the blob is OS-encrypted", () => {
    const file = tmpFile();
    writeSessionFile(file, JSON.stringify({ token: "x" }), cryptoStub());
    expect(readSessionFile(file, cryptoStub({ available: false }))).toBeNull();
  });

  it("decrypts a session written by writeSessionFile", () => {
    const file = tmpFile();
    const payload = JSON.stringify({ baseUrl: "http://localhost:8080", token: "t" });
    writeSessionFile(file, payload, cryptoStub());
    expect(readSessionFile(file, cryptoStub())).toBe(payload);
  });

  it("migrates a legacy plaintext session when encryption becomes available", () => {
    const file = tmpFile();
    const payload = JSON.stringify({ baseUrl: "http://localhost:8080", token: "legacy" });
    fs.writeFileSync(file, payload, "utf8");
    expect(readSessionFile(file, cryptoStub())).toBe(payload);
    expect(fs.readFileSync(file, "utf8")).toMatch(/^ENC:/);
  });

  it("migrates legacy plaintext to local AES when OS encryption is unavailable", () => {
    const dir = tmpDir();
    const file = path.join(dir, "session.bin");
    const keyPath = path.join(dir, "session.key");
    const payload = JSON.stringify({ token: "legacy" });
    fs.writeFileSync(file, payload, "utf8");
    expect(readSessionFile(file, cryptoStub({ available: false }), { localKeyPath: keyPath })).toBe(payload);
    expect(isLocalEncryptedSession(fs.readFileSync(file))).toBe(true);
    expect(fs.readFileSync(file, "utf8")).not.toContain("legacy");
  });

  it("migrates a local-encrypted session to OS crypto when the keyring returns", () => {
    const dir = tmpDir();
    const file = path.join(dir, "session.bin");
    const keyPath = path.join(dir, "session.key");
    const payload = JSON.stringify({ token: "roundtrip" });
    writeSessionFile(file, payload, cryptoStub({ available: false }), { localKeyPath: keyPath });
    expect(isLocalEncryptedSession(fs.readFileSync(file))).toBe(true);
    expect(readSessionFile(file, cryptoStub({ available: true }), { localKeyPath: keyPath })).toBe(payload);
    expect(fs.readFileSync(file, "utf8")).toMatch(/^ENC:/);
  });
});

describe("local AES helpers", () => {
  it("round-trips and rejects a wrong key", () => {
    const key = loadOrCreateLocalKey(path.join(tmpDir(), "k"));
    const other = loadOrCreateLocalKey(path.join(tmpDir(), "k2"));
    const blob = encryptLocalSession("hello", key);
    expect(decryptLocalSession(blob, key)).toBe("hello");
    expect(() => decryptLocalSession(blob, other)).toThrow();
  });
});
