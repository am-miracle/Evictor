import { access, readFile } from "node:fs/promises";
import path from "node:path";

import lintStagedConfig from "../lint-staged.config.mjs";

const hooks = [".husky/pre-commit", ".husky/commit-msg", ".husky/pre-push"];

await Promise.all(hooks.map((hook) => access(hook)));

const frontendFormatter =
  lintStagedConfig["frontend/**/*.{js,mjs,ts,tsx,json,css,md,yml,yaml}"];
const formatterCommand = frontendFormatter([path.resolve("frontend/src/app/page.tsx")]);
if (!formatterCommand.startsWith("cd frontend && ")) {
  throw new Error("frontend staged-file tools must execute from the frontend directory");
}

const prePush = await readFile(".husky/pre-push", "utf8");
if (!prePush.includes("make lint") || !prePush.includes("make test")) {
  throw new Error("pre-push hook does not require lint and test");
}

if (!prePush.includes("refs/heads/master)") || !prePush.includes("refs/heads/dev)")) {
  throw new Error("pre-push hook does not protect master and dev");
}
if (!prePush.includes("EVICTOR_BOOTSTRAP_DEV")) {
  throw new Error("pre-push hook does not define the one-time dev bootstrap path");
}
if (!prePush.includes('dev_ref="refs/remotes/origin/dev"')) {
  throw new Error("pre-push hook does not require branches based on origin/dev");
}

process.stdout.write("Git hook policy is configured.\n");
