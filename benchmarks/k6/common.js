export const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";
export const API_KEY = __ENV.PIXRAIL_API_KEY || "dev-secret";

export function transferPayload(i) {
  return JSON.stringify({
    account_id: "acct_k6",
    amount_cents: 12345 + i,
    currency: "BRL",
    receiver_key: `receiver-${i}@example.com`,
    receiver_key_type: "EMAIL",
  });
}

export function headers(i) {
  return {
    Authorization: `Bearer ${API_KEY}`,
    "Content-Type": "application/json",
    "Idempotency-Key": `k6-${Date.now()}-${i}`,
    "X-Correlation-ID": `corr-k6-${Date.now()}-${i}`,
  };
}
