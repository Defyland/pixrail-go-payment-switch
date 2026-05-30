import http from "k6/http";
import { check, sleep } from "k6";
import { BASE_URL, headers, transferPayload } from "./common.js";

export const options = {
  stages: [
    { duration: "30s", target: 20 },
    { duration: "1m", target: 20 },
    { duration: "30s", target: 0 },
  ],
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<150", "p(99)<300"],
  },
};

export default function () {
  const res = http.post(`${BASE_URL}/v1/pix/transfers`, transferPayload(__ITER), { headers: headers(__ITER) });
  check(res, { "accepted": (r) => r.status === 201 || r.status === 200 || r.status === 429 });
  sleep(0.2);
}
