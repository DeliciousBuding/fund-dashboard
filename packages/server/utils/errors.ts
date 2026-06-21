/** Unified error handling — ApiError with factory functions */

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
    // Maintain proper stack trace in V8
    if (Error.captureStackTrace) {
      Error.captureStackTrace(this, ApiError);
    }
  }
}

// ── Factory functions ──────────────────────────────────────────────────

export function badRequest(message: string): ApiError {
  return new ApiError(400, "bad_request", message);
}

export function notFound(message: string): ApiError {
  return new ApiError(404, "not_found", message);
}

export function unauthorized(message: string = "Unauthorized"): ApiError {
  return new ApiError(401, "unauthorized", message);
}

export function internalError(message: string = "Internal server error"): ApiError {
  return new ApiError(500, "internal", message);
}

export function conflict(message: string): ApiError {
  return new ApiError(409, "conflict", message);
}
