let value = "";

const model = {
  getWordUntilPosition(_position: { lineNumber: number; column: number }) {
    return { word: "", startColumn: 1, endColumn: 1 };
  },
  getLineContent(_line: number) {
    return value.split("\n")[_line - 1] ?? "";
  },
};

export const languages = {
  CompletionItemKind: {
    Field: 3,
    Enum: 15,
    Value: 12,
  },
  registerCompletionItemProvider(_lang: string, _provider: unknown) {
    return { dispose() {} };
  },
};

export const editor = {
  create(_host?: unknown, opts?: { value?: string }) {
    value = opts?.value ?? "";
    return {
      setValue(next: string) {
        value = next;
      },
      getValue() {
        return value;
      },
      getModel() {
        return model;
      },
      dispose() {
        value = "";
      },
      onDidChangeModelContent(_cb: () => void) {
        return { dispose() {} };
      },
    };
  },
  setTheme(_theme: string) {
    /* no-op for tests */
  },
};

export default { editor, languages };
