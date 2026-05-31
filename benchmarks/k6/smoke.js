import http from "k6/http";
import { check } from "k6";
import { BASE_URL, transferParams, transferPayload, warmup } from "./common.js";

export const options = {
  vus: 1,
  iterations: 5,
  thresholds: {
    "http_req_failed{phase:measured}": ["rate<0.01"],
    "http_req_duration{phase:measured}": ["p(95)<100"],
  },
};

export function setup() {
  warmup();
}

export default function () {
  const res = http.post(`${BASE_URL}/v1/pix/transfers`, transferPayload(__ITER), transferParams(__ITER));
  check(res, {
    "created or replayed": (r) => r.status === 201 || r.status === 200,
  });
}
