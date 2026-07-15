import { describe, expect, it } from "vitest";
import { productName } from "./product";

describe("dashboard shell", () => {
  it("identifies the product", () => {
    expect(productName).toBe("Evictor");
  });
});
