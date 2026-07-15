import path from "node:path";

const frontendRoot = path.resolve("frontend");

function shellQuote(value) {
  return `'${value.replaceAll("'", `'"'"'`)}'`;
}

function fromFrontend(command, files) {
  const relativeFiles = files.map((file) => shellQuote(path.relative(frontendRoot, file)));
  return `cd frontend && ${command} -- ${relativeFiles.join(" ")}`;
}

export default {
  "frontend/**/*.{js,mjs,ts,tsx,json,css,md,yml,yaml}": (files) =>
    fromFrontend("./node_modules/.bin/prettier --write --ignore-unknown", files),
  "frontend/**/*.{js,mjs,ts,tsx}": (files) =>
    fromFrontend("./node_modules/.bin/eslint --fix --max-warnings=0", files),
  "backend/**/*.go": ["gofmt -w"],
};
