import argparse
import sys
import socket


class RateLimiterClientError(Exception):
    pass


def send_request(
    host: str,
    port: int,
    message: str,
    timeout: float,
) -> str:

    payload = message.encode("utf-8")

    try:
        with socket.create_connection((host, port), timeout=timeout) as sock:
            sock.settimeout(timeout)
            sock.sendall(payload)
            sock.shutdown(socket.SHUT_WR)

            chunks = []
            while True:
                chunk = sock.recv(4096)
                if not chunk:
                    break
                chunks.append(chunk)

    except socket.timeout as exc:
        raise RateLimiterClientError("request timed out") from exc
    except ConnectionRefusedError as exc:
        raise RateLimiterClientError("connection refused") from exc
    except socket.gaierror as exc:
        raise RateLimiterClientError(f"failed to resolve host: {host}") from exc
    except OSError as exc:
        raise RateLimiterClientError(f"network error: {exc}") from exc

    try:
        response = b"".join(chunks).decode("utf-8").strip()

    except UnicodeDecodeError as exc:
        raise RateLimiterClientError("server returned non-utf8 response") from exc

    if not response:
        raise RateLimiterClientError("empty response from server")

    if response.startswith("e:"):
        raise RateLimiterClientError(f"server error: {response}")

    return response


def main() -> None:
    parser = argparse.ArgumentParser(
        description="TCP client for ratelimiter server",
    )
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=49105)
    parser.add_argument("--group", default="default")
    parser.add_argument("--client-id", default="token_value")
    parser.add_argument("--timeout", type=float, default=1.0)

    args = parser.parse_args()


    message = f"{args.group}:{args.client_id}"
    try:
        response = send_request(
            args.host,
            args.port,
            message,
            args.timeout,
        )
    except RateLimiterClientError as exc:
        print(f"error: {exc}", file=sys.stderr)
        raise SystemExit(1)

    print(response)


if __name__ == "__main__":
    main()
