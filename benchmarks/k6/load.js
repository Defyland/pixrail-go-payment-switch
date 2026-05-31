import http from "k6/http";
import { check, sleep } from "k6";
import { BASE_URL, transferParams, transferPayload, warmup } from "./common.js";

http.setResponseCallback(http.expectedStatuses({ min: 200, max: 201 }, 429));

export const options = {
  stages: [
    { duration: "30s", target: 20 },
    { duration: "1m", target: 20 },
    { duration: "30s", target: 0 },
  ],
  thresholds: {
    "http_req_failed{phase:measured}": ["rate<0.01"],
    "http_req_duration{phase:measured}": ["p(95)<150", "p(99)<300"],
  },
};

export function setup() {
  warmup();
}

export default function () {
  const res = http.post(`${BASE_URL}/v1/pix/transfers`, transferPayload(__ITER), transferParams(__ITER));
  check(res, { "accepted": (r) => r.status === 201 || r.status === 200 || r.status === 429 });
  sleep(0.2);
}
