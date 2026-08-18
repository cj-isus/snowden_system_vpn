// R2 file server — раздаёт файлы из bucket snowden-releases
// по публичным ссылкам без авторизации.
// Деплоится как Cloudflare Worker.

export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    const path = url.pathname;

    // CORS
    const corsHeaders = {
      "Access-Control-Allow-Origin": "*",
      "Access-Control-Allow-Methods": "GET, HEAD, OPTIONS",
      "Access-Control-Allow-Headers": "Content-Type",
    };

    if (request.method === "OPTIONS") {
      return new Response(null, { headers: corsHeaders });
    }

    // === Главная страница (лендинг) ===
    if (path === "/" || path === "/index.html") {
      // Редирект на Pages лендинг
      return Response.redirect("https://snowden-system.pages.dev/", 302);
    }

    // === Version JSON (для проверки обновлений) ===
    if (path === "/version.json") {
      return new Response(JSON.stringify({
        version: "1.1.0",
        versionCode: 110,
        pc_url: "https://r2.snowden-system.workers.dev/snowden-portable.zip",
        android_url: "https://r2.snowden-system.workers.dev/snowden-android.apk",
        ios_config_url: "https://r2.snowden-system.workers.dev/snowden-ios-config.json",
        changelog: "2 сервера (NL+FR), gRPC, HTTPUpgrade, Firefox uTLS, авто-failover"
      }), {
        headers: { "Content-Type": "application/json", ...corsHeaders }
      });
    }

    // === Раздача файлов из R2 ===
    // Убираем ведущий /
    const key = path.replace(/^\//, "");

    if (!key) {
      return new Response("Not found", { status: 404 });
    }

    const object = await env.RELEASES.get(key);

    if (object === null) {
      return new Response("File not found: " + key, { status: 404 });
    }

    // Определяем Content-Type
    const contentTypes = {
      ".zip": "application/zip",
      ".apk": "application/vnd.android.package-archive",
      ".json": "application/json",
    };
    let contentType = "application/octet-stream";
    for (const [ext, type] of Object.entries(contentTypes)) {
      if (key.endsWith(ext)) { contentType = type; break; }
    }

    const headers = new Headers();
    object.writeHttpMetadata(headers);
    headers.set("Content-Type", contentType);
    headers.set("Content-Disposition", `attachment; filename="${key}"`);
    headers.set("Access-Control-Allow-Origin", "*");
    headers.set("Cache-Control", "public, max-age=3600");

    return new Response(object.body, { headers });
  }
};
