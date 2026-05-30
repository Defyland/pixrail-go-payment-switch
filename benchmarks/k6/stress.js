import http from "k6/http";
import { check, sleep } from "k6";
import { BASE_URL, headers, transferPayload } from "./common.js";

export const options = {
  stages: [
    { duration: "30s", target: 50 },
    { duration: "1m", target: 100 },
    { duration: "30s", target: 0 },
  ],
  thresholds: {
    http_req_duration: ["p(99)<800"],
  },
};

export default function () {
  const res = http.post(`${BASE_URL}/v1/pix/transfers`, transferPayload(__ITER), { headers: headers(__ITER) });
  check(res, { "expected status": (r) => [200, 201, 429].includes(r.status) });
  sleep(0.1);
}
