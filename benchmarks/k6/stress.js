import http from "k6/http";
import { check, sleep } from "k6";
import { BASE_URL, transferParams, transferPayload, warmup } from "./common.js";

http.setResponseCallback(http.expectedStatuses({ min: 200, max: 201 }, 429));

export const options = {
  stages: [
    { duration: "30s", target: 50 },
    { duration: "1m", target: 100 },
    { duration: "30s", target: 0 },
  ],
  thresholds: {
    "http_req_duration{phase:measured}": ["p(99)<800"],
  },
};

export function setup() {
  warmup();
}

export default function () {
  const res = http.post(`${BASE_URL}/v1/pix/transfers`, transferPayload(__ITER), transferParams(__ITER));
  check(res, { "expected status": (r) => [200, 201, 429].includes(r.status) });
  sleep(0.1);
}
