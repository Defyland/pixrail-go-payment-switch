import http from "k6/http";
import { check } from "k6";
import { BASE_URL, headers, transferPayload } from "./common.js";

export const options = {
  stages: [
    { duration: "10s", target: 5 },
    { duration: "10s", target: 120 },
    { duration: "20s", target: 120 },
    { duration: "10s", target: 0 },
  ],
  thresholds: {
    http_req_duration: ["p(99)<1000"],
  },
};

export default function () {
  const res = http.post(`${BASE_URL}/v1/pix/transfers`, transferPayload(__ITER), { headers: headers(__ITER) });
  check(res, { "expected status": (r) => [200, 201, 429].includes(r.status) });
}
