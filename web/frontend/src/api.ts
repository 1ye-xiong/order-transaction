export type Row = Record<string, any>;
// Requests always use the same origin; identity is supplied only by the server JWT.
export async function request(
  path: string,
  token: string,
  method = "GET",
  body?: unknown,
): Promise<Row> {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 20000);
  try {
    const response = await fetch("/api/v1" + path, {
      method,
      signal: controller.signal,
      headers: {
        ...(token ? { Authorization: "Bearer " + token } : {}),
        ...(body !== undefined ? { "Content-Type": "application/json" } : {}),
      },
      ...(body !== undefined ? { body: JSON.stringify(body) } : {}),
    });
    const result = await response.json();
    if (!response.ok)
      throw new Error(
        `${response.status}: ${result.error?.message || result.message || "请求失败"}`,
      );
    const data = result.data ?? result;
    if (
      method === "POST" &&
      (((path.endsWith("/ship") || path.startsWith("/cs/pay-exceptions/")) && !data.success) ||
        (path.endsWith("/pay") && !data.accepted))
    )
      throw new Error(data.message || "服务端未接受操作");
    return data;
  } finally {
    clearTimeout(timeout);
  }
}
export function money(value: unknown) {
  const cents = Number(value ?? 0);
  if (!Number.isSafeInteger(cents)) return "金额异常";
  return `${cents < 0 ? "-" : ""}${Math.floor(Math.abs(cents) / 100)}.${String(Math.abs(cents) % 100).padStart(2, "0")}`;
}
