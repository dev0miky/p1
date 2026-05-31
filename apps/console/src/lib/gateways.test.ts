import { describe, it, expect } from "vitest";
import type { Gateway } from "./gateways";

type RegisterStatus = Gateway["register_status"];

function statusLabel(s: RegisterStatus): string {
  switch (s) {
    case "registered": return "registered";
    case "trying": return "trying";
    case "failed": return "failed";
    case "noreg": return "noreg";
    case "down": return "down";
    default: return "unknown";
  }
}

function statusClass(s: RegisterStatus): string {
  switch (s) {
    case "registered": return "text-phosphor";
    case "trying": return "text-amber";
    case "failed":
    case "noreg":
    case "down": return "text-danger";
    default: return "text-ink-700";
  }
}

function passwordBody(hasPassword: boolean, newPassword: string): { password?: string } {
  if (newPassword) return { password: newPassword };
  return {};
}

describe("statusLabel", () => {
  it("maps all known statuses", () => {
    expect(statusLabel("registered")).toBe("registered");
    expect(statusLabel("trying")).toBe("trying");
    expect(statusLabel("failed")).toBe("failed");
    expect(statusLabel("noreg")).toBe("noreg");
    expect(statusLabel("down")).toBe("down");
    expect(statusLabel("unknown")).toBe("unknown");
  });
});

describe("statusClass", () => {
  it("registered is phosphor green", () => {
    expect(statusClass("registered")).toBe("text-phosphor");
  });
  it("trying is amber", () => {
    expect(statusClass("trying")).toBe("text-amber");
  });
  it("failure states are danger", () => {
    expect(statusClass("failed")).toBe("text-danger");
    expect(statusClass("noreg")).toBe("text-danger");
    expect(statusClass("down")).toBe("text-danger");
  });
  it("unknown is muted", () => {
    expect(statusClass("unknown")).toBe("text-ink-700");
  });
});

describe("password omit logic", () => {
  it("omits password when field is empty", () => {
    const body = passwordBody(true, "");
    expect(body).not.toHaveProperty("password");
  });
  it("sends password when user typed a new one", () => {
    const body = passwordBody(true, "newpass");
    expect(body.password).toBe("newpass");
  });
  it("sends password on create even when no prior password", () => {
    const body = passwordBody(false, "secret");
    expect(body.password).toBe("secret");
  });
});
