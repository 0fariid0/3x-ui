import { z } from 'zod';

export const ClientEventDetailsSchema = z.object({
  version: z.literal(1).optional(),
  source: z.string().optional(),
  previousTotalBytes: z.number().optional(),
  totalBytes: z.number().optional(),
  addedBytes: z.number().optional(),
  previousExpiryTime: z.number().optional(),
  expiryTime: z.number().optional(),
  addedMilliseconds: z.number().nullish(),
  previousUsedBytes: z.number().optional(),
  usedBytes: z.number().optional(),
  totalGB: z.number().optional(),
  addBytes: z.number().optional(),
  addDays: z.number().optional(),
});

export function parseClientEventDetails(value: string | undefined) {
  if (!value) return null;
  try {
    const parsed = ClientEventDetailsSchema.safeParse(JSON.parse(value));
    return parsed.success ? parsed.data : null;
  } catch {
    return null;
  }
}
