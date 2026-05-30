export const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";
export const API_KEY = __ENV.PIXRAIL_API_KEY || "dev-secret";

export function transferPayload(i) {
  const requestID = `${__VU}-${i}-${Date.now()}`;
  return JSON.stringify({
    account_id: "acct_k6",
    amount_cents: 12345 + i,
    currency: "BRL",
    receiver_key: `receiver-${requestID}@example.com`,
    receiver_key_type: "EMAIL",
  });
}

export function headers(i) {
  const requestID = `${__VU}-${i}-${Date.now()}`;
  return {
    Authorization: `Bearer ${API_KEY}`,
    "Content-Type": "application/json",
    "Idempotency-Key": `k6-${requestID}`,
    "X-Correlation-ID": `corr-k6-${requestID}`,
  };
}
