// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import "./testkit";
import { WorkbenchChangesPage } from "./WorkbenchChangesPage";
import { MockPort } from "../port/mock";
import type { AgentPort, WorkspaceChange } from "../port/port";

afterEach(cleanup);

const many = (n: number): WorkspaceChange[] =>
  Array.from({ length: n }, (_, i) => ({
    path: `pkg${i % 40}/dir${i % 7}/file${i}.go`,
    status: i % 3 === 0 ? "M" : "??",
    unstaged: true,
  }));

function draw(list: WorkspaceChange[]) {
  const port = new MockPort() as unknown as AgentPort;
  vi.spyOn(port, "changeDiff").mockResolvedValue({ path: "x", diff: "diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b\n", truncated: false });
  const stage = vi.fn(async () => {});
  port.stageChanges = stage;
  render(<WorkbenchChangesPage port={port} changes={list} branch="main" shown remote={false} onOpen={() => {}} />);
  return { port, stage };
}

const rowCount = () => document.querySelectorAll(".wb-change").length;

describe("the changes page with a workspace too large to draw whole", () => {
  it("draws a bounded page of rows and says how many it left out", () => {
    draw(many(5000));
    expect(rowCount()).toBeLessThanOrEqual(300);
    expect(document.querySelector(".wb-changes-head")!.textContent).toContain("5000 个文件");
    expect(screen.getByRole("button", { name: /再显示/ })).toBeTruthy();
    expect(document.querySelector(".wb-changes-body")!.textContent).toMatch(/还有 \d+ 项没有显示/);
  });

  it("shows more rows on request, and stages every change rather than only the drawn ones", async () => {
    const user = userEvent.setup();
    const { port, stage } = draw(many(5000));
    const before = rowCount();
    await user.click(screen.getByRole("button", { name: /再显示/ }));
    expect(rowCount()).toBeGreaterThan(before);
    await user.click(screen.getByRole("button", { name: "暂存全部" }));
    expect(stage).toHaveBeenCalledWith({ all: true, unstage: false });
  });

  it("draws a short list whole, with nothing left out", () => {
    draw(many(30));
    expect(screen.queryByRole("button", { name: /再显示/ })).toBeNull();
    expect(rowCount()).toBe(30);
  });
});
