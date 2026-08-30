import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { EnvSwitcher } from "./EnvSwitcher";
import type { Session } from "../session";

afterEach(() => {
  cleanup();
});

const multi: Session = {
  activeInstallId: "dev",
  environments: [
    { installId: "dev", installRole: "test", baseUrl: "http://dev", token: "t1" },
    { installId: "uat", installRole: "staging", baseUrl: "http://uat", token: "t2" },
    { installId: "prod", installRole: "prod", baseUrl: "http://prod", token: "" },
  ],
  baseUrl: "http://dev",
  token: "t1",
};

describe("EnvSwitcher", () => {
  it("shows empty CTA when no environments", async () => {
    const user = userEvent.setup();
    const onAdd = vi.fn();
    render(<EnvSwitcher session={null} onSwitch={vi.fn()} onAddEnvironment={onAdd} />);
    await user.click(screen.getByTestId("env-switcher"));
    expect(onAdd).toHaveBeenCalled();
  });

  it("opens menu, switches env, and adds environment", async () => {
    const user = userEvent.setup();
    const onSwitch = vi.fn();
    const onAdd = vi.fn();
    render(<EnvSwitcher session={multi} onSwitch={onSwitch} onAddEnvironment={onAdd} />);
    expect(screen.getByText("test")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: /test/i }));
    expect(await screen.findByTestId("env-switcher-menu")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: /staging/i }));
    expect(onSwitch).toHaveBeenCalledWith("uat");
    await user.click(screen.getByRole("button", { name: /test/i }));
    await user.click(screen.getByTestId("env-switcher-add"));
    expect(onAdd).toHaveBeenCalled();
  });

  it("disables disconnected peers", async () => {
    const user = userEvent.setup();
    const onSwitch = vi.fn();
    render(<EnvSwitcher session={multi} onSwitch={onSwitch} onAddEnvironment={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: /test/i }));
    const prod = screen.getByRole("button", { name: /prod/i });
    expect((prod as HTMLButtonElement).disabled).toBe(true);
    await user.click(prod);
    expect(onSwitch).not.toHaveBeenCalledWith("prod");
  });

  it("closes menu on outside click", async () => {
    const user = userEvent.setup();
    render(
      <div>
        <EnvSwitcher session={multi} onSwitch={vi.fn()} onAddEnvironment={vi.fn()} />
        <button type="button">outside</button>
      </div>,
    );
    await user.click(screen.getByRole("button", { name: /test/i }));
    expect(screen.getByTestId("env-switcher-menu")).toBeTruthy();
    fireEvent.mouseDown(screen.getByRole("button", { name: /outside/i }));
    expect(screen.queryByTestId("env-switcher-menu")).toBeNull();
  });
});
