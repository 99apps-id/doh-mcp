#!/usr/bin/env node
// Launches the doh-mcp binary, downloading the release for this platform on
// first run and caching it. This is what makes `npx -y doh-mcp` work in any MCP
// client without a manual install. Set DOH_MCP_BINARY to use a local build.
"use strict";

const { spawnSync } = require("child_process");
const fs = require("fs");
const os = require("os");
const path = require("path");
const https = require("https");

const VERSION = require("./package.json").version;
const REPO = "99apps-id/doh-mcp";

function platform() {
  switch (process.platform) {
    case "win32":
      return "windows";
    case "darwin":
      return "darwin";
    case "linux":
      return "linux";
    default:
      return process.platform;
  }
}

function arch() {
  switch (process.arch) {
    case "x64":
      return "amd64";
    case "arm64":
      return "arm64";
    default:
      return process.arch;
  }
}

function assetName() {
  const ext = process.platform === "win32" ? ".exe" : "";
  return `doh-mcp_${VERSION}_${platform()}_${arch()}${ext}`;
}

function cacheDir() {
  if (process.env.DOH_MCP_CACHE) {
    return process.env.DOH_MCP_CACHE;
  }
  const base =
    process.platform === "win32"
      ? process.env.LOCALAPPDATA || path.join(os.homedir(), "AppData", "Local")
      : process.env.XDG_CACHE_HOME || path.join(os.homedir(), ".cache");
  return path.join(base, "doh-mcp", VERSION);
}

function download(url, dest, redirects = 5) {
  return new Promise((resolve, reject) => {
    https
      .get(url, { headers: { "User-Agent": "doh-mcp-npm" } }, (response) => {
        if (
          response.statusCode >= 300 &&
          response.statusCode < 400 &&
          response.headers.location
        ) {
          response.resume();
          if (redirects <= 0) {
            reject(new Error("too many redirects"));
            return;
          }
          download(response.headers.location, dest, redirects - 1).then(resolve, reject);
          return;
        }
        if (response.statusCode !== 200) {
          response.resume();
          reject(new Error(`download failed: HTTP ${response.statusCode} for ${url}`));
          return;
        }
        const file = fs.createWriteStream(dest);
        response.pipe(file);
        file.on("finish", () => file.close(() => resolve()));
        file.on("error", reject);
      })
      .on("error", reject);
  });
}

async function ensureBinary() {
  if (process.env.DOH_MCP_BINARY) {
    return process.env.DOH_MCP_BINARY;
  }
  const dir = cacheDir();
  const binary = path.join(dir, process.platform === "win32" ? "doh-mcp.exe" : "doh-mcp");
  if (fs.existsSync(binary)) {
    return binary;
  }
  fs.mkdirSync(dir, { recursive: true });
  const url = `https://github.com/${REPO}/releases/download/v${VERSION}/${assetName()}`;
  await download(url, binary);
  if (process.platform !== "win32") {
    fs.chmodSync(binary, 0o755);
  }
  return binary;
}

(async () => {
  try {
    const binary = await ensureBinary();
    const result = spawnSync(binary, process.argv.slice(2), { stdio: "inherit" });
    process.exit(result.status === null ? 1 : result.status);
  } catch (error) {
    console.error(`doh-mcp: ${error.message}`);
    console.error(
      "Check that the release for this version exists, or set DOH_MCP_BINARY to a local build."
    );
    process.exit(1);
  }
})();
