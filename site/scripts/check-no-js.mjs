// The site ships no JavaScript. `npm run build` runs this after `astro build`
// and fails if dist/ has a .js file or any page has a <script> tag, so CI
// catches a stray client component, island, or prefetch script.
import fs from "node:fs";
import path from "node:path";

const dist = path.join(import.meta.dirname, "..", "dist");
const problems = [];

for (const file of fs.readdirSync(dist, { recursive: true })) {
  if (/\.(js|mjs|cjs)$/.test(file)) {
    problems.push(`${file}: a JavaScript file`);
  }
  if (file.endsWith(".html")) {
    const html = fs.readFileSync(path.join(dist, file), "utf8");
    if (/<script/i.test(html)) problems.push(`${file}: has a <script> tag`);
  }
}

if (problems.length > 0) {
  console.error("The site must ship no JavaScript, but dist/ has some:");
  for (const problem of problems) console.error(`  ${problem}`);
  process.exit(1);
}
console.log("check-no-js: no .js files and no <script> tags in dist/");
