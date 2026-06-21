// common.ts — shared primitives & unified error shape
// v3.0 contracts SSOT
import { z } from "zod";

/** Market identifier for securities */
export type Market = 'sh' | 'sz' | 'hk' | 'us';

/** Type discriminator between funds and individual stocks */
export type SecurityType = 'fund' | 'stock';

/**
 * Unified API error shape (fixes G8).
 * All endpoints return this JSON on error. Historical variants
 * ({error}, {error,message}, {status,error}) are accepted on parse
 * via optionals, but producers should emit {error} (+ optional code/message).
 */
export const ApiErrorSchema = z.object({
  error: z.string(),
  message: z.string().optional(),
  code: z.string().optional(),
});
export type ApiError = z.infer<typeof ApiErrorSchema>;
