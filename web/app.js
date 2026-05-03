const statusEl = document.getElementById("status");
const runBtn = document.getElementById("runBtn");
const replayBtn = document.getElementById("replayBtn");
const seedBtn = document.getElementById("seedBtn");

const claimsEl = document.getElementById("claims");
const followupsEl = document.getElementById("followups");
const riskEl = document.getElementById("riskLevel");
const draftEl = document.getElementById("draft");
const auditEl = document.getElementById("audit");

let cachedDemoToken = null;
let cachedDemoTokenExp = 0;

function base64Url(bytes) {
  const binary = Array.from(bytes, (byte) => String.fromCharCode(byte)).join("");
  return btoa(binary).replaceAll("+", "-").replaceAll("/", "_").replaceAll("=", "");
}

async function demoAuthHeader() {
  const now = Math.floor(Date.now() / 1000);
  if (cachedDemoToken && cachedDemoTokenExp - now > 60) {
    return `Bearer ${cachedDemoToken}`;
  }

  cachedDemoTokenExp = now + 3600;
  const encoder = new TextEncoder();
  const header = base64Url(encoder.encode(JSON.stringify({ alg: "HS256", typ: "JWT" })));
  const payload = base64Url(
    encoder.encode(
      JSON.stringify({
        sub: "u-demo",
        tenant_id: "tenant-demo",
        roles: ["doctor"],
        exp: cachedDemoTokenExp,
      }),
    ),
  );
  const key = await crypto.subtle.importKey(
    "raw",
    encoder.encode("dev-secret"),
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign"],
  );
  const signature = await crypto.subtle.sign("HMAC", key, encoder.encode(`${header}.${payload}`));
  cachedDemoToken = `${header}.${payload}.${base64Url(new Uint8Array(signature))}`;
  return `Bearer ${cachedDemoToken}`;
}

function readForm() {
  const sessionId = document.getElementById("sessionId").value.trim();
  const userId = document.getElementById("userId").value.trim();
  const locale = document.getElementById("locale").value.trim();
  const inputText = document.getElementById("inputText").value.trim();
  const metadataText = document.getElementById("metadata").value.trim();
  let metadata = {};
  if (metadataText) {
    try {
      metadata = JSON.parse(metadataText);
    } catch (err) {
      statusEl.textContent = "Metadata JSON 无法解析";
      throw err;
    }
  }
  return { session_id: sessionId, user_id: userId, locale, input_text: inputText, metadata };
}

function setStatus(text) {
  statusEl.textContent = text;
}

function renderClaims(claims = []) {
  claimsEl.innerHTML = "";
  claims.forEach((claim) => {
    const container = document.createElement("div");
    container.className = "claim";

    const header = document.createElement("div");
    header.className = "claim-header";
    const title = document.createElement("div");
    title.textContent = claim.text || "";
    const badge = document.createElement("span");
    badge.className = "badge" + (claim.degraded ? " degraded" : "");
    badge.textContent = claim.degraded ? "DEGRADED" : "EVIDENCED";
    header.appendChild(title);
    header.appendChild(badge);

    const evidence = document.createElement("div");
    evidence.className = "evidence";
    (claim.evidence || []).forEach((ev) => {
      const item = document.createElement("div");
      item.textContent = `${ev.source || ""} · ${ev.title || ""} · ${ev.region || ""} ${ev.version || ""}`;
      evidence.appendChild(item);
      if (ev.snippet) {
        const snippet = document.createElement("div");
        snippet.textContent = ev.snippet;
        evidence.appendChild(snippet);
      }
    });

    container.appendChild(header);
    container.appendChild(evidence);
    claimsEl.appendChild(container);
  });
}

function renderFollowups(list = []) {
  followupsEl.innerHTML = "";
  if (!list.length) {
    const li = document.createElement("li");
    li.textContent = "暂无追问";
    followupsEl.appendChild(li);
    return;
  }
  list.forEach((item) => {
    const li = document.createElement("li");
    li.textContent = item;
    followupsEl.appendChild(li);
  });
}

function renderRisk(level) {
  riskEl.querySelector(".value").textContent = level || "UNKNOWN";
}

function renderDraft(text) {
  draftEl.textContent = text || "暂无草稿";
}

async function runSession() {
  try {
    const payload = readForm();
    setStatus("请求中...");
    const res = await fetch("/v1/session/run", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: await demoAuthHeader(),
      },
      body: JSON.stringify(payload),
    });
    if (!res.ok) {
      const errText = await res.text();
      throw new Error(errText);
    }
    const data = await res.json();
    renderRisk(data.risk_level);
    renderDraft(data.draft);
    renderFollowups(data.followups || []);
    renderClaims(data.claims || []);
    setStatus("已完成");
  } catch (err) {
    console.error(err);
    setStatus("请求失败，请检查控制台日志");
  }
}

async function replayAudit() {
  try {
    const { session_id } = readForm();
    setStatus("拉取回放...");
    const res = await fetch("/v1/session/replay", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: await demoAuthHeader(),
      },
      body: JSON.stringify({ session_id }),
    });
    if (!res.ok) {
      const errText = await res.text();
      throw new Error(errText);
    }
    const data = await res.json();
    auditEl.textContent = (data.events || []).join("\n");
    setStatus("回放完成");
  } catch (err) {
    console.error(err);
    setStatus("回放失败，请检查控制台日志");
  }
}

seedBtn.addEventListener("click", () => {
  document.getElementById("sessionId").value = "s-demo";
  document.getElementById("userId").value = "u-demo";
  document.getElementById("locale").value = "zh-CN";
  document.getElementById("inputText").value = "服用阿司匹林后出现胃部不适，是否会与布洛芬冲突？";
  document.getElementById("metadata").value = '{"channel":"ui","department":"cardio"}';
});

runBtn.addEventListener("click", runSession);
replayBtn.addEventListener("click", replayAudit);
