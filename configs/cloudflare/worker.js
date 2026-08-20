/**
 * snowden.system — Cloudflare Worker
 *
 * Endpoints:
 *   GET /api/config      → dynamic VPN config (servers, endpoints, protocols)
 *   GET /api/health      → edge health-check of the VPS (pings from CF edge)
 *   GET /api/version     → latest app version + download URL
 *   POST /api/telemetry  → anonymous telemetry (D1)
 *   GET /                → status page (JSON)
 *
 * KV namespaces needed:
 *   SNOWDEN_CONFIG  — stores the dynamic config JSON
 *   SNOWDEN_VERSION — stores latest version string + download URL
 *
 * D1 binding needed:
 *   DB — telemetry database (table: events)
 */

// ─── Dynamic Config ───

// Public fallback config. Private credentials are deliberately not returned by
// /api/config; authenticated provisioning must deliver them separately.
function defaultConfig(env) {
  const vpsAddress = env.VPS_IP || "YOUR_VPS_IP";
  const domain = env.VPN_DOMAIN || "YOUR_DOMAIN.nip.io";

  return {
    servers: [
      {
        id: "vps-nl",
        name: "VPS Netherlands",
        address: vpsAddress,
        port: 443,
        protocol: "vless+tls",
        domain,
        active: true
      },
      {
        id: "hysteria2",
        name: "Hysteria2 (UDP)",
        address: vpsAddress,
        port: 8443,
        protocol: "hysteria2",
        domain,
        active: true
      },
      {
        id: "warp-cf",
        name: "WARP Cloudflare",
        address: "162.159.192.1",
        port: 4500,
        protocol: "wireguard",
        publicKey: "bmXOC+F1FxEMF9dyiK2H5/1SUtzH00JuVo51h2wPfgyo=",
        active: false
      }
    ],
    routing: {
      ruCidrUrl: "https://raw.githubusercontent.com/itdoginfo/allow-domains/refs/heads/main/Services/meta.lst",
      splitTunneling: true
    },
    version: "1.0.0",
    updatedAt: new Date().toISOString()
  };
}

export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);
    const path = url.pathname;

    // CORS
    const corsHeaders = {
      "Access-Control-Allow-Origin": "*",
      "Access-Control-Allow-Methods": "GET, POST, OPTIONS",
      "Access-Control-Allow-Headers": "Content-Type, Authorization",
    };

    if (request.method === "OPTIONS") {
      return new Response(null, { headers: corsHeaders });
    }

    // ─── Static file hosting (landing + downloads) ───
    // Лендинг доступен по корневому URL
    if (path === "/" || path === "/index.html") {
      const html = await env.SNOWDEN_CONFIG.get("landing");
      if (html) {
        return new Response(html, { headers: { "Content-Type": "text/html; charset=utf-8", ...corsHeaders } });
      }
    }

    // version.json — endpoint для проверки обновлений приложениями
    if (path === "/version.json") {
      const ver = await env.SNOWDEN_CONFIG.get("version.json");
      if (ver) {
        return new Response(ver, { headers: { "Content-Type": "application/json", ...corsHeaders } });
      }
    }

    // iOS config
    if (path === "/snowden-ios-config.json") {
      const cfg = await env.SNOWDEN_CONFIG.get("ios-config.json");
      if (cfg) {
        return new Response(cfg, { headers: { "Content-Type": "application/json", ...corsHeaders } });
      }
    }

    // Downloads — PC ZIP и Android APK из KV
    if (path === "/files/snowden-portable.zip") {
      const data = await env.SNOWDEN_CONFIG.get("pc-zip", "arrayBuffer");
      if (data) {
        return new Response(data, { headers: { "Content-Type": "application/zip", "Content-Disposition": "attachment; filename=snowden-portable.zip", ...corsHeaders } });
      }
    }
    if (path === "/files/app-release.apk") {
      const data = await env.SNOWDEN_CONFIG.get("android-apk", "arrayBuffer");
      if (data) {
        return new Response(data, { headers: { "Content-Type": "application/vnd.android.package-archive", "Content-Disposition": "attachment; filename=snowden-android.apk", ...corsHeaders } });
      }
    }

    try {
      // ─── GET /api/config ───
      if (path === "/api/config" && request.method === "GET") {
        // Try KV first, fallback to default
        let config = defaultConfig(env);
        if (env.SNOWDEN_CONFIG) {
          const kvConfig = await env.SNOWDEN_CONFIG.get("config");
          if (kvConfig) {
            config = JSON.parse(kvConfig);
          }
        }
        return jsonResponse(publicConfig(config), corsHeaders);
      }

      // ─── GET /api/health ───
      if (path === "/api/health" && request.method === "GET") {
        const results = await checkVpsHealth(request, env);
        return jsonResponse(results, corsHeaders);
      }

      // ─── GET /api/version ───
      if (path === "/api/version" && request.method === "GET") {
        let version = { version: "1.0.0", downloadUrl: "" };
        if (env.SNOWDEN_VERSION) {
          const kvVer = await env.SNOWDEN_VERSION.get("latest");
          if (kvVer) version = JSON.parse(kvVer);
        }
        return jsonResponse(version, corsHeaders);
      }

      // ─── POST /api/telemetry ───
      if (path === "/api/telemetry" && request.method === "POST") {
        const body = await request.json();
        if (env.DB) {
          await env.DB.prepare(
            "INSERT INTO events (region, event, protocol, latency_ms, timestamp) VALUES (?, ?, ?, ?, ?)"
          ).bind(
            body.region || "unknown",
            body.event || "unknown",
            body.protocol || "unknown",
            body.latency_ms || 0,
            Date.now()
          ).run();
        }
        return jsonResponse({ ok: true }, corsHeaders);
      }

      // ─── GET / (status page) ───
      if (path === "/" || path === "") {
        const health = await checkVpsHealth(request, env);
        return jsonResponse({
          status: "online",
          service: "snowden.system",
          time: new Date().toISOString(),
          vps: health,
          version: "1.0.0"
        }, corsHeaders);
      }

      return new Response("Not Found", { status: 404, headers: corsHeaders });

    } catch (err) {
      return jsonResponse({ error: err.message }, corsHeaders, 500);
    }
  }
};

// Remove credentials from the unauthenticated metadata endpoint. This is not a
// replacement for authenticated config delivery; it is a safety boundary for
// the public fallback/KV route.
function publicConfig(config) {
  return {
    ...config,
    servers: (config.servers || []).map((server) => {
      const {
        uuid,
        password,
        privateKey,
        private_key,
        token,
        secret,
        ...metadata
      } = server;
      return metadata;
    })
  };
}

// ─── Edge Health Check ───

async function checkVpsHealth(request, env) {
  const vpsHost = env.VPS_IP || "YOUR_VPS_IP";
  const colo = request?.cf?.colo || "unknown";

  // Test TCP connectivity from CF edge to VPS
  const tests = {};

  // Port 443 (VLESS)
  try {
    const start = Date.now();
    const resp = await fetch(`https://${vpsHost}:443/`, {
      signal: AbortSignal.timeout(5000),
      redirect: "manual"
    });
    tests.vless = {
      port: 443,
      reachable: true,
      latency: Date.now() - start,
      status: resp.status
    };
  } catch (e) {
    tests.vless = { port: 443, reachable: false, error: e.message };
  }

  // Generate_204 test (end-to-end tunnel check would need the client)
  try {
    const start = Date.now();
    const resp = await fetch("https://www.gstatic.com/generate_204", {
      signal: AbortSignal.timeout(5000)
    });
    tests.internet = {
      reachable: true,
      latency: Date.now() - start
    };
  } catch (e) {
    tests.internet = { reachable: false, error: e.message };
  }

  return {
    edge: colo,
    timestamp: new Date().toISOString(),
    tests
  };
}

// ─── Helpers ───

function jsonResponse(data, headers, status = 200) {
  return new Response(JSON.stringify(data, null, 2), {
    status,
    headers: {
      "Content-Type": "application/json",
      ...headers
    }
  });
}
